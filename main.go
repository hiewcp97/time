package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"time-retention/internal/config"
	"time-retention/internal/db"
	"time-retention/internal/server"
	"time-retention/internal/worker"

	"github.com/redis/go-redis/v9"
)

func main() {
	config.Load()
	log.Println("Starting Retention Agent Platform...")

	// 1. Get configurations from config library
	dbURL := config.AppConfig.DatabaseURL
	redisURL := config.AppConfig.RedisURL
	port := config.AppConfig.Port
	role := config.AppConfig.Role

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. Connect to Database
	dbPool, err := db.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer dbPool.Close()

	// 3. Connect to Redis (optional fallback)
	var rdb *redis.Client
	opt, err := redis.ParseURL("redis://" + redisURL)
	if err == nil {
		rdb = redis.NewClient(opt)
		// Test redis connection
		err = rdb.Ping(ctx).Err()
		if err == nil {
			log.Printf("Connected to Redis at %s", redisURL)
		} else {
			log.Printf("Redis ping failed: %v. Running in DB-only cache mode.", err)
			rdb = nil
		}
	} else {
		// Try fallback to simple connection
		rdb = redis.NewClient(&redis.Options{Addr: redisURL})
		err = rdb.Ping(ctx).Err()
		if err == nil {
			log.Printf("Connected to Redis at %s", redisURL)
		} else {
			log.Printf("Redis Parse/Connect failed: %v. Running in DB-only cache mode.", err)
			rdb = nil
		}
	}

	// 4. Bootstrap database (run schema and seed 500,000 customers if empty)
	// We only run this in API or BOTH mode, to prevent duplicate worker conflicts
	if role == "api" || role == "both" {
		err = db.Bootstrap(ctx, dbPool, "./db/schema.sql", "./db/seed.sql")
		if err != nil {
			log.Fatalf("Database bootstrap failed: %v", err)
		}
	}

	// 5. Start Roles
	switch role {
	case "worker":
		log.Println("Running in WORKER mode")
		worker.StartWorkerPool(ctx, dbPool, rdb)
		<-ctx.Done()
		log.Println("Shutting down worker...")

	case "api":
		log.Println("Running in API mode")
		srv := server.NewServer(dbPool, rdb)
		r := srv.SetupRouter()
		
		log.Printf("Server listening on port %s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("Server failed to run: %v", err)
		}

	case "both":
		log.Println("Running in BOTH mode (API + Workers)")
		worker.StartWorkerPool(ctx, dbPool, rdb)
		
		srv := server.NewServer(dbPool, rdb)
		r := srv.SetupRouter()
		
		log.Printf("Server listening on port %s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("Server failed to run: %v", err)
		}

	default:
		log.Fatalf("Unknown role: %s", role)
	}

	log.Println("Retention Agent Platform shutdown successfully.")
}
package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect sets up a pgxpool.Pool, retrying up to 15 times for the DB container to start.
func Connect(ctx context.Context, connStr string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error
	for i := 0; i < 15; i++ {
		config, err := pgxpool.ParseConfig(connStr)
		if err == nil {
			// Optimize connection settings for concurrent throughput
			config.MaxConns = 50
			config.MinConns = 5
			config.MaxConnIdleTime = 30 * time.Minute
			pool, err = pgxpool.NewWithConfig(ctx, config)
			if err == nil {
				err = pool.Ping(ctx)
				if err == nil {
					return pool, nil
				}
			}
		}
		log.Printf("Waiting for Postgres to be ready... (%d/15): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to database: %w", err)
}

// Bootstrap runs the schema.sql and seeds customers if the DB has fewer than 500,000 customers.
func Bootstrap(ctx context.Context, pool *pgxpool.Pool, schemaPath, seedPath string) error {
	log.Printf("Executing schema.sql from %s...", schemaPath)
	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}
	_, err = pool.Exec(ctx, string(schemaSQL))
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	var count int64
	// Check if the table exists and check count. If error because table doesn't exist, we bootstrap anyway.
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM customers").Scan(&count)
	if err != nil {
		// Table might not exist, but schema.sql should have created it. Try again.
		return fmt.Errorf("failed to verify customers table count: %w", err)
	}

	if count < 500000 {
		log.Printf("Database contains %d customers. Seeding 500k customers with usage history from %s...", count, seedPath)
		seedSQL, err := os.ReadFile(seedPath)
		if err != nil {
			return fmt.Errorf("failed to read seed file: %w", err)
		}
		start := time.Now()
		_, err = pool.Exec(ctx, string(seedSQL))
		if err != nil {
			return fmt.Errorf("failed to execute seeding: %w", err)
		}
		log.Printf("Database successfully seeded in %v", time.Since(start))
	} else {
		log.Printf("Database already contains %d customers. Skipping seed.", count)
	}

	return nil
}

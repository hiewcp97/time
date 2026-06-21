package worker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"time-retention/internal/llm"
	"time-retention/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// WorkItem represents a single customer pitch generation task within a bulk job.
type WorkItem struct {
	JobID      string
	CustomerID int64
}

// Global Job Queue (backpressure limit of 1000 bulk jobs)
var JobQueue = make(chan string, 1000)

// Global Work Queue (processes individual customer items in workers)
var WorkQueue = make(chan WorkItem, 10000)

// RateLimiter limits LLM requests to 5 per second across all workers
var RateLimiter = time.Tick(200 * time.Millisecond)

// StartWorkerPool runs the background workers and the job coordinator.
func StartWorkerPool(ctx context.Context, dbPool *pgxpool.Pool, rdb *redis.Client) {
	// Reset orphaned PROCESSING jobs back to PENDING on startup so they resume processing
	_, err := dbPool.Exec(ctx, "UPDATE bulk_jobs SET status = 'PENDING' WHERE status = 'PROCESSING'")
	if err != nil {
		log.Printf("Worker Pool Startup: Failed to reset orphaned processing jobs: %v", err)
	}

	// Start 10 workers to process individual work items
	for i := 1; i <= 10; i++ {
		go runWorker(ctx, dbPool, rdb, i)
	}

	// Start coordinator to monitor bulk jobs in JobQueue
	go runCoordinator(ctx, dbPool)
}

// runCoordinator reads pending jobs from the database, claims them, and dispatches them to WorkQueue.
func runCoordinator(ctx context.Context, dbPool *pgxpool.Pool) {
	log.Println("Job Coordinator started")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Fetch next pending job (oldest first)
			var jobID string
			err := dbPool.QueryRow(ctx, "SELECT id FROM bulk_jobs WHERE status = 'PENDING' ORDER BY created_at ASC LIMIT 1").Scan(&jobID)
			if err != nil {
				// No pending jobs, continue polling
				continue
			}

			log.Printf("Coordinator: Found pending bulk job %s", jobID)

			// Claim the job (Update status to PROCESSING)
			_, err = dbPool.Exec(ctx, "UPDATE bulk_jobs SET status = 'PROCESSING' WHERE id = $1 AND status = 'PENDING'", jobID)
			if err != nil {
				log.Printf("Coordinator Error: Failed to update job %s to PROCESSING: %v", jobID, err)
				continue
			}

			// Read and dispatch pending items in chunks of 100
			for {
				rows, err := dbPool.Query(ctx, 
					"SELECT customer_id FROM bulk_job_items WHERE bulk_job_id = $1 AND status = 'PENDING' ORDER BY id LIMIT 100", 
					jobID)
				if err != nil {
					log.Printf("Coordinator Error: Failed to query job items for %s: %v", jobID, err)
					break
				}

				var customerIDs []int64
				for rows.Next() {
					var cid int64
					if err := rows.Scan(&cid); err == nil {
						customerIDs = append(customerIDs, cid)
					}
				}
				rows.Close()

				if len(customerIDs) == 0 {
					break // No more pending items
				}

				for _, cid := range customerIDs {
					select {
					case <-ctx.Done():
						return
					case WorkQueue <- WorkItem{JobID: jobID, CustomerID: cid}:
						// Queued successfully
					}
				}
			}
			log.Printf("Coordinator: Dispatched all items for job %s", jobID)
		}
	}
}

// runWorker pulls customer items from WorkQueue and processes them.
func runWorker(ctx context.Context, dbPool *pgxpool.Pool, rdb *redis.Client, workerID int) {
	log.Printf("Worker %d started", workerID)
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-WorkQueue:
			processWorkItem(ctx, dbPool, rdb, item)
		}
	}
}

func processWorkItem(ctx context.Context, dbPool *pgxpool.Pool, rdb *redis.Client, item WorkItem) {
	start := time.Now()

	// 1. Fetch customer details
	details, err := FetchCustomerDetails(ctx, dbPool, item.CustomerID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to fetch customer: %v", err)
		updateJobProgress(ctx, dbPool, item.JobID, item.CustomerID, false, errMsg)
		logPitchEvent(ctx, dbPool, item.CustomerID, "FAILED", "", 0, 0, start, err.Error())
		log.Printf(`{"customer_id": %d, "job_id": "%s", "status": "FAILED", "error": "%s", "latency_ms": %d}`,
			item.CustomerID, item.JobID, errMsg, time.Since(start).Milliseconds())
		return
	}

	// 2. Compute customer hash and check cache
	hash := llm.ComputeCustomerHash(details)
	if cachedPitch, _, hit := GetCachedPitch(ctx, rdb, dbPool, item.CustomerID, hash); hit {
		updateJobProgress(ctx, dbPool, item.JobID, item.CustomerID, true, "")
		// Log a successful event but mark it cached
		logPitchEvent(ctx, dbPool, item.CustomerID, "SUCCESS", "cached", 0, 0, start, "")
		log.Printf(`{"customer_id": %d, "job_id": "%s", "status": "SUCCESS (CACHED)", "latency_ms": %d}`,
			item.CustomerID, item.JobID, time.Since(start).Milliseconds())
		_ = cachedPitch
		return
	}

	// 3. Rate Limit LLM Calls
	<-RateLimiter

	// 4. Generate the pitch via LLM (with fallback to mock)
	pitchText, modelUsed, err := llm.GeneratePitch(ctx, details)
	if err != nil {
		errMsg := fmt.Sprintf("LLM error: %v", err)
		updateJobProgress(ctx, dbPool, item.JobID, item.CustomerID, false, errMsg)
		logPitchEvent(ctx, dbPool, item.CustomerID, "FAILED", modelUsed, 0, 0, start, err.Error())
		log.Printf(`{"customer_id": %d, "job_id": "%s", "status": "FAILED", "error": "%s", "latency_ms": %d}`,
			item.CustomerID, item.JobID, errMsg, time.Since(start).Milliseconds())
		return
	}

	// 5. Store Cache (Redis + Postgres)
	SetCachedPitch(ctx, rdb, dbPool, item.CustomerID, hash, pitchText, modelUsed)

	// 6. Update progress and log success event
	updateJobProgress(ctx, dbPool, item.JobID, item.CustomerID, true, "")
	logPitchEvent(ctx, dbPool, item.CustomerID, "SUCCESS", modelUsed, len(details.FullName)/4+20, len(pitchText)/4, start, "")
	log.Printf(`{"customer_id": %d, "job_id": "%s", "status": "SUCCESS", "latency_ms": %d}`,
		item.CustomerID, item.JobID, time.Since(start).Milliseconds())
}

// FetchCustomerDetails retrieves customer data and usage history.
func FetchCustomerDetails(ctx context.Context, dbPool *pgxpool.Pool, customerID int64) (*models.CustomerDetails, error) {
	var details models.CustomerDetails
	var startDate, endDate time.Time

	err := dbPool.QueryRow(ctx, `
		SELECT id, customer_number, full_name, plan_name, contract_start_date, contract_end_date, monthly_fee, tenure_months
		FROM customers WHERE id = $1`, customerID).Scan(
		&details.ID, &details.CustomerNumber, &details.FullName, &details.PlanName, &startDate, &endDate, &details.MonthlyFee, &details.TenureMonths,
	)
	if err != nil {
		return nil, err
	}

	details.ContractStartDate = startDate.Format("2006-01-02")
	details.ContractEndDate = endDate.Format("2006-01-02")

	rows, err := dbPool.Query(ctx, `
		SELECT usage_month, download_gb, upload_gb 
		FROM usage_history WHERE customer_id = $1 
		ORDER BY usage_month DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var u models.UsageHistory
		var month time.Time
		if err := rows.Scan(&month, &u.DownloadGB, &u.UploadGB); err == nil {
			u.MonthStr = month.Format("2006-01")
			details.UsageHistory = append(details.UsageHistory, u)
		}
	}

	return &details, nil
}

// GetCachedPitch retrieves pitch from Redis or DB if hash matches.
func GetCachedPitch(ctx context.Context, rdb *redis.Client, dbPool *pgxpool.Pool, customerID int64, hash string) (string, time.Time, bool) {
	if rdb != nil {
		key := fmt.Sprintf("pitch:%d:%s", customerID, hash)
		val, err := rdb.Get(ctx, key).Result()
		if err == nil {
			parts := strings.SplitN(val, "|", 2)
			if len(parts) == 2 {
				if t, err := time.Parse(time.RFC3339, parts[0]); err == nil {
					return parts[1], t, true
				}
			}
		}
	}

	var pitchText string
	var cachedHash string
	var generatedAt time.Time
	err := dbPool.QueryRow(ctx, "SELECT pitch_text, customer_data_hash, generated_at FROM generated_pitches WHERE customer_id = $1", customerID).Scan(&pitchText, &cachedHash, &generatedAt)
	if err == nil && cachedHash == hash {
		if rdb != nil {
			key := fmt.Sprintf("pitch:%d:%s", customerID, hash)
			val := fmt.Sprintf("%s|%s", generatedAt.Format(time.RFC3339), pitchText)
			_ = rdb.Set(ctx, key, val, 24*time.Hour)
		}
		return pitchText, generatedAt, true
	}

	return "", time.Time{}, false
}

// SetCachedPitch caches a pitch in DB and Redis.
func SetCachedPitch(ctx context.Context, rdb *redis.Client, dbPool *pgxpool.Pool, customerID int64, hash string, pitchText string, modelName string) {
	now := time.Now()
	_, err := dbPool.Exec(ctx, `
		INSERT INTO generated_pitches (customer_id, customer_data_hash, pitch_text, llm_model, generated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (customer_id) 
		DO UPDATE SET customer_data_hash = EXCLUDED.customer_data_hash,
                      pitch_text = EXCLUDED.pitch_text,
                      llm_model = EXCLUDED.llm_model,
                      generated_at = EXCLUDED.generated_at`,
		customerID, hash, pitchText, modelName, now)
	if err != nil {
		log.Printf("Failed to cache pitch in database: %v", err)
	}

	if rdb != nil {
		key := fmt.Sprintf("pitch:%d:%s", customerID, hash)
		val := fmt.Sprintf("%s|%s", now.Format(time.RFC3339), pitchText)
		_ = rdb.Set(ctx, key, val, 24*time.Hour)
	}
}

// updateJobProgress updates the bulk_job_item and increments completion counters in bulk_jobs.
func updateJobProgress(ctx context.Context, dbPool *pgxpool.Pool, jobID string, customerID int64, isSuccess bool, errMsg string) {
	var status string
	var errStr *string
	if isSuccess {
		status = "SUCCESS"
	} else {
		status = "FAILED"
		errStr = &errMsg
	}

	_, err := dbPool.Exec(ctx, `
		UPDATE bulk_job_items 
		SET status = $1, error_message = $2, processed_at = NOW() 
		WHERE bulk_job_id = $3 AND customer_id = $4`,
		status, errStr, jobID, customerID)
	if err != nil {
		log.Printf("Worker Error: Failed to update bulk_job_item status: %v", err)
	}

	var updateQuery string
	if isSuccess {
		updateQuery = `
			UPDATE bulk_jobs 
			SET completed_count = completed_count + 1,
				status = CASE WHEN completed_count + failed_count + 1 >= total_count THEN 'COMPLETED' ELSE 'PROCESSING' END
			WHERE id = $1`
	} else {
		updateQuery = `
			UPDATE bulk_jobs 
			SET failed_count = failed_count + 1,
				status = CASE WHEN completed_count + failed_count + 1 >= total_count THEN 'COMPLETED' ELSE 'PROCESSING' END
			WHERE id = $1`
	}
	_, err = dbPool.Exec(ctx, updateQuery, jobID)
	if err != nil {
		log.Printf("Worker Error: Failed to update bulk_job counters: %v", err)
	}
}

func logPitchEvent(ctx context.Context, dbPool *pgxpool.Pool, customerID int64, status string, modelName string, promptTok, compTok int, start time.Time, errMsg string) {
	var errStr *string
	if errMsg != "" {
		errStr = &errMsg
	}
	latency := time.Since(start).Milliseconds()

	_, err := dbPool.Exec(ctx, `
		INSERT INTO pitch_generation_events (customer_id, status, model_name, prompt_tokens, completion_tokens, latency_ms, retry_count, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, 0, $7)`,
		customerID, status, modelName, promptTok, compTok, latency, errStr)
	if err != nil {
		log.Printf("Failed to log pitch generation event: %v", err)
	}
}

package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"time-retention/internal/llm"
	"time-retention/internal/models"
	"time-retention/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	dbPool *pgxpool.Pool
	rdb    *redis.Client
}

func NewServer(dbPool *pgxpool.Pool, rdb *redis.Client) *Server {
	return &Server{
		dbPool: dbPool,
		rdb:    rdb,
	}
}

func (s *Server) SetupRouter() *gin.Engine {
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Serve Static Files
	r.Static("/static", "./static")
	r.StaticFile("/", "./static/index.html")

	// API V1 Group
	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", s.handleHealth)
		v1.GET("/customers", s.handleListCustomers)
		v1.GET("/customers/:customer_id", s.handleGetCustomer)
		v1.POST("/customers/:customer_id/pitch", s.handleGeneratePitch)
		v1.GET("/customers/:customer_id/pitch", s.handleGetExistingPitch)
		v1.GET("/bulk-pitches", s.handleListBulkJobs)
		v1.POST("/bulk-pitches", s.handleCreateBulkJob)
		v1.GET("/bulk-pitches/:job_id", s.handleGetBulkJobStatus)
		v1.GET("/bulk-pitches/:job_id/items", s.handleGetBulkJobItems)
	}

	return r
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// handleListCustomers fetches a list of customers with search, filtering, and keyset pagination.
func (s *Server) handleListCustomers(c *gin.Context) {
	search := c.Query("search")
	plan := c.Query("plan")
	expiresBefore := c.Query("expires_before")
	cursor := c.Query("cursor")
	limitStr := c.Query("limit")

	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	var whereClauses []string
	var args []interface{}
	argCount := 1

	if search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("full_name ILIKE $%d", argCount))
		args = append(args, "%"+search+"%")
		argCount++
	}

	if plan != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("plan_name = $%d", argCount))
		args = append(args, plan)
		argCount++
	}

	if expiresBefore != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("contract_end_date <= $%d", argCount))
		args = append(args, expiresBefore)
		argCount++
	}

	// Decode Keyset Pagination Cursor (Base64 of "YYYY-MM-DD|id")
	var cursorDate string
	var cursorID int64
	if cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursor)
		if err == nil {
			parts := strings.Split(string(decoded), "|")
			if len(parts) == 2 {
				cursorDate = parts[0]
				if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					cursorID = id
				}
			}
		}
	}

	if cursorDate != "" && cursorID > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("(contract_end_date, id) > ($%d, $%d)", argCount, argCount+1))
		args = append(args, cursorDate, cursorID)
		argCount += 2
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, full_name, plan_name, contract_end_date 
		FROM customers 
		%s 
		ORDER BY contract_end_date ASC, id ASC 
		LIMIT $%d`, whereSQL, argCount)
	args = append(args, limit)

	rows, err := s.dbPool.Query(c.Request.Context(), query, args...)
	if err != nil {
		log.Printf("DB Error in ListCustomers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to fetch customers"})
		return
	}
	defer rows.Close()

	type CustomerResponse struct {
		ID               int64  `json:"id"`
		Name             string `json:"name"`
		PlanName         string `json:"plan_name"`
		ContractEndDate  string `json:"contract_end_date"`
	}

	var data []CustomerResponse
	var lastEndDate time.Time
	var lastID int64

	for rows.Next() {
		var r CustomerResponse
		var endDate time.Time
		if err := rows.Scan(&r.ID, &r.Name, &r.PlanName, &endDate); err == nil {
			r.ContractEndDate = endDate.Format("2006-01-02")
			data = append(data, r)
			lastEndDate = endDate
			lastID = r.ID
		}
	}

	nextCursor := ""
	if len(data) == limit {
		cursorStr := fmt.Sprintf("%s|%d", lastEndDate.Format("2006-01-02"), lastID)
		nextCursor = base64.StdEncoding.EncodeToString([]byte(cursorStr))
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        data,
		"next_cursor": nextCursor,
	})
}

// handleGetCustomer retrieves customer profile details along with usage history.
func (s *Server) handleGetCustomer(c *gin.Context) {
	idStr := c.Param("customer_id")
	customerID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "Invalid customer ID"})
		return
	}

	details, err := worker.FetchCustomerDetails(c.Request.Context(), s.dbPool, customerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}

	c.JSON(http.StatusOK, details)
}

// handleGeneratePitch generates (or retrieves from cache) a personalized sales pitch.
func (s *Server) handleGeneratePitch(c *gin.Context) {
	idStr := c.Param("customer_id")
	customerID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "Invalid customer ID"})
		return
	}

	ctx := c.Request.Context()
	start := time.Now()

	// 1. Fetch customer details
	details, err := worker.FetchCustomerDetails(ctx, s.dbPool, customerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}

	// 2. Compute customer hash and check cache
	hash := llm.ComputeCustomerHash(details)
	if cachedPitch, hit := worker.GetCachedPitch(ctx, s.rdb, s.dbPool, customerID, hash); hit {
		c.JSON(http.StatusOK, models.Pitch{
			CustomerID:  customerID,
			PitchText:   cachedPitch,
			GeneratedAt: time.Now(),
			Cached:      true,
		})
		return
	}

	// 3. Rate Limit
	<-worker.RateLimiter

	// 4. Generate pitch
	pitchText, modelUsed, err := llm.GeneratePitch(ctx, details)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}

	// 5. Store Cache
	worker.SetCachedPitch(ctx, s.rdb, s.dbPool, customerID, hash, pitchText, modelUsed)

	// Log event
	logPitchEvent(ctx, s.dbPool, customerID, "SUCCESS", modelUsed, len(details.FullName)/4+20, len(pitchText)/4, start, "")

	c.JSON(http.StatusOK, models.Pitch{
		CustomerID:  customerID,
		PitchText:   pitchText,
		GeneratedAt: time.Now(),
		Cached:      false,
	})
}

// handleGetExistingPitch retrieves the cached pitch if present in DB.
func (s *Server) handleGetExistingPitch(c *gin.Context) {
	idStr := c.Param("customer_id")
	customerID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "Invalid customer ID"})
		return
	}

	var pitchText string
	var generatedAt time.Time
	err = s.dbPool.QueryRow(c.Request.Context(), 
		"SELECT pitch_text, generated_at FROM generated_pitches WHERE customer_id = $1", customerID).Scan(&pitchText, &generatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"customer_id":  customerID,
		"pitch":        pitchText,
		"generated_at": generatedAt.Format(time.RFC3339),
	})
}

// handleCreateBulkJob creates a background job matching filter criteria.
func (s *Server) handleCreateBulkJob(c *gin.Context) {
	var req models.CreateBulkJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// 1. Build where clause dynamically
	var whereClauses []string
	var args []interface{}
	argCount := 2

	if req.Filters.ExpiresBefore != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("contract_end_date <= $%d", argCount))
		args = append(args, req.Filters.ExpiresBefore)
		argCount++
	}

	if req.Filters.PlanName != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("plan_name = $%d", argCount))
		args = append(args, req.Filters.PlanName)
		argCount++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	jobID := uuid.New().String()

	// Check global active jobs count in DB (backpressure control)
	var activeCount int
	err := s.dbPool.QueryRow(ctx, "SELECT COUNT(*) FROM bulk_jobs WHERE status = 'PENDING' OR status = 'PROCESSING'").Scan(&activeCount)
	if err == nil && activeCount >= 1000 {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":   "rate_limited",
			"message": "Too many active bulk jobs. Please try again later.",
		})
		return
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to start transaction"})
		return
	}
	defer tx.Rollback(ctx)

	// Insert Job Record
	_, err = tx.Exec(ctx, `
		INSERT INTO bulk_jobs (id, status, total_count, completed_count, failed_count, created_at)
		VALUES ($1, 'PENDING', 0, 0, 0, NOW())`, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to create job"})
		return
	}

	// Insert Job Items in Bulk inside the Database
	insertQuery := fmt.Sprintf(`
		INSERT INTO bulk_job_items (bulk_job_id, customer_id, status)
		SELECT $1::uuid, id, 'PENDING'
		FROM customers
		%s`, whereSQL)

	_, err = tx.Exec(ctx, insertQuery, append([]interface{}{jobID}, args...)...)
	if err != nil {
		log.Printf("Bulk Job Error: failed bulk insert of job items: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to initialize job items"})
		return
	}

	// Set the total counts
	_, err = tx.Exec(ctx, `
		UPDATE bulk_jobs 
		SET total_count = (SELECT COUNT(*) FROM bulk_job_items WHERE bulk_job_id = $1)
		WHERE id = $1`, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to count job items"})
		return
	}

	err = tx.Commit(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Transaction commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"job_id": jobID,
		"status": "PENDING",
	})
}

// handleGetBulkJobStatus returns the bulk job counters.
func (s *Server) handleGetBulkJobStatus(c *gin.Context) {
	jobID := c.Param("job_id")

	var job models.BulkJob
	err := s.dbPool.QueryRow(c.Request.Context(), `
		SELECT id, status, total_count, completed_count, failed_count, created_at
		FROM bulk_jobs WHERE id = $1`, jobID).Scan(
		&job.ID, &job.Status, &job.TotalCount, &job.CompletedCount, &job.FailedCount, &job.CreatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// handleGetBulkJobItems retrieves a list of items for a bulk job with filtering and offset-based limit.
func (s *Server) handleGetBulkJobItems(c *gin.Context) {
	jobID := c.Param("job_id")
	status := c.Query("status")
	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")

	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	var whereClauses = []string{"bulk_job_id = $1"}
	var args = []interface{}{jobID}
	argCount := 2

	if status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argCount))
		args = append(args, status)
		argCount++
	}

	whereSQL := "WHERE " + strings.Join(whereClauses, " AND ")

	query := fmt.Sprintf(`
		SELECT customer_id, status, error_message 
		FROM bulk_job_items 
		%s 
		ORDER BY id ASC 
		LIMIT $%d OFFSET $%d`, whereSQL, argCount, argCount+1)
	args = append(args, limit, offset)

	rows, err := s.dbPool.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	defer rows.Close()

	var data []models.BulkJobItem
	for rows.Next() {
		var item models.BulkJobItem
		if err := rows.Scan(&item.CustomerID, &item.Status, &item.ErrorMessage); err == nil {
			data = append(data, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
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
		log.Printf("Server Event Log Error: %v", err)
	}
}

func (s *Server) handleListBulkJobs(c *gin.Context) {
	rows, err := s.dbPool.Query(c.Request.Context(), `
		SELECT id, status, total_count, completed_count, failed_count, created_at
		FROM bulk_jobs 
		ORDER BY created_at DESC 
		LIMIT 100`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	defer rows.Close()

	var data []models.BulkJob
	for rows.Next() {
		var job models.BulkJob
		if err := rows.Scan(&job.ID, &job.Status, &job.TotalCount, &job.CompletedCount, &job.FailedCount, &job.CreatedAt); err == nil {
			data = append(data, job)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

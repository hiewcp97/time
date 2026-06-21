package models

import (
	"time"
)

type Customer struct {
	ID                int64     `json:"id" db:"id"`
	CustomerNumber    string    `json:"customer_number,omitempty" db:"customer_number"`
	FullName          string    `json:"name" db:"full_name"`
	PlanName          string    `json:"plan_name" db:"plan_name"`
	ContractStartDate time.Time `json:"contract_start_date,omitempty" db:"contract_start_date"`
	ContractEndDate   time.Time `json:"contract_end_date" db:"contract_end_date"`
	MonthlyFee        float64   `json:"monthly_fee,omitempty" db:"monthly_fee"`
	TenureMonths      int       `json:"tenure_months,omitempty" db:"tenure_months"`
	Version           int       `json:"version,omitempty" db:"version"`
	CreatedAt         time.Time `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

type UsageHistory struct {
	ID          int64     `json:"-" db:"id"`
	CustomerID  int64     `json:"-" db:"customer_id"`
	UsageMonth  time.Time `json:"-" db:"usage_month"`
	MonthStr    string    `json:"month" db:"-"` // Formatted as "YYYY-MM" in Go
	DownloadGB  float64   `json:"download_gb" db:"download_gb"`
	UploadGB    float64   `json:"upload_gb,omitempty" db:"upload_gb"`
	CreatedAt   time.Time `json:"-" db:"created_at"`
}

type CustomerDetails struct {
	ID                int64          `json:"id"`
	CustomerNumber    string         `json:"customer_number"`
	FullName          string         `json:"name"`
	PlanName          string         `json:"plan_name"`
	ContractStartDate string         `json:"contract_start_date"`
	ContractEndDate   string         `json:"contract_end_date"`
	MonthlyFee        float64        `json:"monthly_fee"`
	TenureMonths      int            `json:"tenure_months"`
	UsageHistory      []UsageHistory `json:"usage_history"`
}

type Pitch struct {
	CustomerID  int64     `json:"customer_id" db:"customer_id"`
	PitchText   string    `json:"pitch" db:"pitch_text"`
	GeneratedAt time.Time `json:"generated_at" db:"generated_at"`
	Cached      bool      `json:"cached" db:"-"`
}

type BulkJobFilters struct {
	ExpiresBefore string `json:"expires_before"`
	PlanName      string `json:"plan_name"`
}

type CreateBulkJobRequest struct {
	Filters BulkJobFilters `json:"filters"`
}

type BulkJob struct {
	ID             string    `json:"job_id" db:"id"`
	Status         string    `json:"status" db:"status"`
	TotalCount     int       `json:"total_count" db:"total_count"`
	CompletedCount int       `json:"completed_count" db:"completed_count"`
	FailedCount    int       `json:"failed_count" db:"failed_count"`
	CreatedAt      time.Time `json:"created_at,omitempty" db:"created_at"`
}

type BulkJobItem struct {
	CustomerID   int64   `json:"customer_id" db:"customer_id"`
	Status       string  `json:"status" db:"status"`
	ErrorMessage *string `json:"error_message,omitempty" db:"error_message"`
}

package llm

import (
	"context"
	"strings"
	"testing"

	"time-retention/internal/config"
	"time-retention/internal/models"
)

func TestComputeCustomerHash(t *testing.T) {
	details := &models.CustomerDetails{
		FullName:        "John Doe",
		PlanName:        "100Mbps",
		MonthlyFee:      39.99,
		TenureMonths:    12,
		ContractEndDate: "2026-07-01",
		UsageHistory: []models.UsageHistory{
			{MonthStr: "2026-05", DownloadGB: 100, UploadGB: 50},
		},
	}

	hash1 := ComputeCustomerHash(details)
	hash2 := ComputeCustomerHash(details)

	if hash1 != hash2 {
		t.Errorf("Expected identical customer details to produce same hash, got %s and %s", hash1, hash2)
	}

	// Change details and check if hash changes
	details.FullName = "Jane Doe"
	hash3 := ComputeCustomerHash(details)
	if hash1 == hash3 {
		t.Error("Expected modified details to produce different hash")
	}
}

func TestGeneratePitch_Mock(t *testing.T) {
	// Set mock config
	config.AppConfig.GeminiAPIKey = ""
	config.AppConfig.LLMModel = "test-model"

	details := &models.CustomerDetails{
		FullName:        "Alice Smith",
		PlanName:        "1Gbps",
		MonthlyFee:      59.99,
		TenureMonths:    24,
		ContractEndDate: "2026-08-01",
		UsageHistory: []models.UsageHistory{
			{MonthStr: "2026-05", DownloadGB: 600, UploadGB: 100}, // > 500 download
		},
	}

	pitch, model, err := GeneratePitch(context.Background(), details)
	if err != nil {
		t.Fatalf("Expected no error from GeneratePitch mock, got %v", err)
	}

	if !strings.Contains(model, "mock") {
		t.Errorf("Expected model metadata to contain 'mock', got %s", model)
	}

	if !strings.Contains(pitch, "Alice Smith") {
		t.Errorf("Expected pitch to contain customer name Alice Smith, got: %s", pitch)
	}

	if !strings.Contains(pitch, "2Gbps plan to unlock double the speed") {
		t.Errorf("Expected power user recommendations in pitch, got: %s", pitch)
	}
}

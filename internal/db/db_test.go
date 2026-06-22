package db

import (
	"context"
	"testing"
)

func TestConnect_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context immediately

	_, err := Connect(ctx, "postgres://postgres:postgres@localhost:5432/retention?sslmode=disable")
	if err == nil {
		t.Error("Expected error from Connect with a canceled context, got nil")
	}
}

func TestConnect_InvalidParseConfig(t *testing.T) {
	// ParseConfig fails for empty or invalid schemes
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context to avoid loop sleeping

	_, err := Connect(ctx, "invalid_connection_string")
	if err == nil {
		t.Error("Expected error for invalid connection string, got nil")
	}
}

func TestBootstrap_MissingSchemaFile(t *testing.T) {
	ctx := context.Background()
	// Test error path when schema file does not exist
	err := Bootstrap(ctx, nil, "nonexistent_schema.sql", "nonexistent_seed.sql")
	if err == nil {
		t.Error("Expected error when schema file does not exist, got nil")
	}
}

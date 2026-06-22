package worker

import (
	"context"
	"testing"
)

func TestRunWorker_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context immediately

	// Worker should exit immediately without block or panic
	runWorker(ctx, nil, nil, 1)
}

func TestRunCoordinator_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel context immediately

	// Coordinator should exit immediately without block or panic
	runCoordinator(ctx, nil)
}

func TestFetchCustomerDetails_NilPool(t *testing.T) {
	ctx := context.Background()
	
	// Recover from expected panic on nil pointer dereference
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected FetchCustomerDetails to panic on nil dbPool connection, but it did not")
		}
	}()
	
	_, _ = FetchCustomerDetails(ctx, nil, 123)
}

func TestGetCachedPitch_NilConnections(t *testing.T) {
	ctx := context.Background()

	// Recover from expected panic on nil pointer dereference
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected GetCachedPitch to panic on nil dbPool, but it did not")
		}
	}()

	_, _, _ = GetCachedPitch(ctx, nil, nil, 123, "hash")
}

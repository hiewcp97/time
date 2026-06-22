package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCustomerJSONSerialization(t *testing.T) {
	now := time.Now()
	cust := Customer{
		ID:              123,
		CustomerNumber:  "CN-1234",
		FullName:        "Bob Jones",
		PlanName:        "500Mbps",
		ContractEndDate: now,
	}

	data, err := json.Marshal(cust)
	if err != nil {
		t.Fatalf("Failed to marshal Customer: %v", err)
	}

	var decoded Customer
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal Customer: %v", err)
	}

	if decoded.ID != cust.ID {
		t.Errorf("Expected ID %d, got %d", cust.ID, decoded.ID)
	}
	if decoded.FullName != cust.FullName {
		t.Errorf("Expected FullName %q, got %q", cust.FullName, decoded.FullName)
	}
}

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewServer(t *testing.T) {
	srv := NewServer(nil, nil)
	if srv == nil {
		t.Error("Expected NewServer to return non-nil Server instance")
	}
}

func TestSetupRouter(t *testing.T) {
	srv := NewServer(nil, nil)
	r := srv.SetupRouter()
	if r == nil {
		t.Error("Expected SetupRouter to return a gin Engine")
	}
}

func TestHandleHealth(t *testing.T) {
	srv := NewServer(nil, nil)
	r := srv.SetupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected health status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %q", resp["status"])
	}
}

func TestHandleGetCustomer_InvalidID(t *testing.T) {
	srv := NewServer(nil, nil)
	r := srv.SetupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/customers/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected Bad Request 400 for invalid ID, got %d", w.Code)
	}
}

func TestHandleGeneratePitch_InvalidID(t *testing.T) {
	srv := NewServer(nil, nil)
	r := srv.SetupRouter()

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/customers/abc/pitch", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected Bad Request 400 for invalid ID, got %d", w.Code)
	}
}

func TestHandleGetExistingPitch_InvalidID(t *testing.T) {
	srv := NewServer(nil, nil)
	r := srv.SetupRouter()

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/customers/abc/pitch", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected Bad Request 400 for invalid ID, got %d", w.Code)
	}
}

func TestHandleCreateBulkJob_InvalidJSON(t *testing.T) {
	srv := NewServer(nil, nil)
	r := srv.SetupRouter()

	badJSON := []byte(`{invalid_json`)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/bulk-pitches", bytes.NewBuffer(badJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected Bad Request 400 for malformed JSON, got %d", w.Code)
	}
}

func TestCorsMiddleware(t *testing.T) {
	srv := NewServer(nil, nil)
	r := srv.SetupRouter()

	req, _ := http.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent { // CORS OPTIONS returns 204
		t.Errorf("Expected 204 for OPTIONS preflight, got %d", w.Code)
	}

	corsHeader := w.Header().Get("Access-Control-Allow-Origin")
	if corsHeader != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin to be '*', got %q", corsHeader)
	}
}

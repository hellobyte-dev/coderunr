package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/coderunr/api/internal/config"
	"github.com/coderunr/api/internal/handler"
	"github.com/coderunr/api/internal/job"
	"github.com/coderunr/api/internal/middleware"
	"github.com/coderunr/api/internal/runtime"
	"github.com/coderunr/api/internal/types"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

func TestAPIEndpoints(t *testing.T) {
	// Set up test environment
	os.Setenv("CODERUNR_LOG_LEVEL", "error")
	os.Setenv("CODERUNR_DATA_DIRECTORY", "/tmp/coderunr-test")

	// Create test directories first
	os.MkdirAll("/tmp/coderunr-test/packages", 0755) // Load configuration
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize components
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	runtimeManager := runtime.NewManager(cfg)
	jobManager := job.NewManager(cfg)
	h := handler.NewHandler(jobManager, runtimeManager, logger)

	// Set up router
	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.CORS())

	r.Route("/api/v2", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.JSON)
			r.Post("/execute", h.ExecuteCode)
		})
		r.Get("/runtimes", h.GetRuntimes)
	})
	r.Get("/", h.GetVersion)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Test cases
	tests := []struct {
		name           string
		method         string
		path           string
		body           interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "Health Check",
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				if string(body) != "OK" {
					t.Errorf("Expected 'OK', got %s", string(body))
				}
			},
		},
		{
			name:           "Get Version",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if message, ok := response["message"].(string); !ok || message == "" {
					t.Error("Expected message in response")
				}
			},
		},
		{
			name:           "Get Runtimes",
			method:         "GET",
			path:           "/api/v2/runtimes",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var runtimes []types.RuntimeInfo
				if err := json.Unmarshal(body, &runtimes); err != nil {
					t.Fatalf("Failed to unmarshal runtimes: %v", err)
				}
				// Even with no packages, should return empty array
				if runtimes == nil {
					t.Error("Expected array response for runtimes")
				}
			},
		},
		{
			name:   "Execute Code - Invalid Request",
			method: "POST",
			path:   "/api/v2/execute",
			body: map[string]interface{}{
				"language": "",
				"files":    []map[string]string{},
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to unmarshal error response: %v", err)
				}
				if _, ok := response["message"]; !ok {
					t.Error("Expected message in response")
				}
			},
		},
		{
			name:   "Execute Code - No Runtime",
			method: "POST",
			path:   "/api/v2/execute",
			body: map[string]interface{}{
				"language": "nonexistent",
				"version":  "1.0.0",
				"files": []map[string]interface{}{
					{
						"content": "print('hello')",
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to unmarshal error response: %v", err)
				}
				if _, ok := response["message"]; !ok {
					t.Error("Expected message in response for nonexistent runtime")
				}
			},
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			var err error

			if tt.body != nil {
				bodyBytes, _ := json.Marshal(tt.body)
				req, err = http.NewRequest(tt.method, tt.path, bytes.NewBuffer(bodyBytes))
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, err = http.NewRequest(tt.method, tt.path, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}
			}

			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, rr.Body.Bytes())
			}
		})
	}

	// Cleanup
	os.RemoveAll("/tmp/coderunr-test")
}

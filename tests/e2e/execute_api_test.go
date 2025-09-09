package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteAPI tests the core code execution API
func TestExecuteAPI(t *testing.T) {
	t.Run("Execute Python Code", func(t *testing.T) {
		request := ExecutionRequest{
			Language: "python",
			Version:  "3.12.0",
			Files: []File{
				{Content: "print('Hello CodeRunr API!')"},
			},
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result ExecutionResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 0, result.Run.Code)
		assert.Equal(t, "Hello CodeRunr API!\n", result.Run.Stdout)
		assert.Empty(t, result.Run.Stderr)
	})

	t.Run("Execute Invalid Language", func(t *testing.T) {
		request := ExecutionRequest{
			Language: "invalid_language",
			Version:  "1.0.0",
			Files: []File{
				{Content: "print('test')"},
			},
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Execute Empty Code", func(t *testing.T) {
		request := ExecutionRequest{
			Language: "python",
			Version:  "3.12.0",
			Files:    []File{}, // Empty files
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Stdin Without Newline", func(t *testing.T) {
		// Program that prints the exact length of stdin
		code := "import sys\n" +
			"data = sys.stdin.read()\n" +
			"print(len(data))\n"

		request := ExecutionRequest{
			Language: "python",
			Version:  "3.12.0",
			Files:    []File{{Content: code}},
			Stdin:    "abc", // no trailing newline; expect length 3 exactly
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result ExecutionResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 0, result.Run.Code)
		assert.Equal(t, "3\n", result.Run.Stdout)
	})

	t.Run("Run Timeout Override", func(t *testing.T) {
		// Program sleeps for 1s; we set run_timeout=50ms to force timeout
		code := "import time\n" +
			"time.sleep(1)\n" +
			"print('done')\n"

		request := map[string]interface{}{
			"language":    "python",
			"version":     "3.12.0",
			"files":       []map[string]string{{"content": code}},
			"run_timeout": 50, // milliseconds
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result ExecutionResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		// Expect timeout: signal should be present (SIGKILL), code may be null/ignored
		if result.Run.Signal == "" {
			t.Fatalf("expected a timeout signal, got none; run: %+v", result.Run)
		}
		assert.Equal(t, "SIGKILL", result.Run.Signal)
	})
}

// TestHealthAPI tests the health check endpoint
func TestHealthAPI(t *testing.T) {
	t.Run("Health Check", func(t *testing.T) {
		resp, err := http.Get(APIBaseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

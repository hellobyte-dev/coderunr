package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPackageManagement tests package install/uninstall operations
func TestPackageManagement(t *testing.T) {
	t.Run("Install Package", func(t *testing.T) {
		// Try to install an already installed package
		request := map[string]string{
			"language": "python",
			"version":  "3.12.0",
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/packages", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		// When already installed, the API may return 500 with specific error message
		if resp.StatusCode == http.StatusInternalServerError {
			var errorResp map[string]string
			json.NewDecoder(resp.Body).Decode(&errorResp)
			assert.Contains(t, errorResp["message"], "already installed")
		} else {
			// If not already installed, should succeed with 201 Created
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		}
	})

	t.Run("Install Invalid Package", func(t *testing.T) {
		request := map[string]string{
			"language": "nonexistent_language",
			"version":  "1.0.0",
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/packages", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Install Package Invalid JSON", func(t *testing.T) {
		resp, err := http.Post(APIBaseURL+"/api/v2/packages", "application/json", bytes.NewBufferString("invalid json"))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Install Package Missing Fields", func(t *testing.T) {
		request := map[string]string{
			"language": "python",
			// missing version
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/packages", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// TestExecuteAPIErrorHandling tests comprehensive error scenarios
func TestExecuteAPIErrorHandling(t *testing.T) {
	t.Run("Invalid JSON Request", func(t *testing.T) {
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBufferString("invalid json"))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Missing Language Field", func(t *testing.T) {
		request := map[string]interface{}{
			"version": "3.12.0",
			"files": []map[string]string{
				{"content": "print('test')"},
			},
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Missing Version Field", func(t *testing.T) {
		request := map[string]interface{}{
			"language": "python",
			"files": []map[string]string{
				{"content": "print('test')"},
			},
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Python Syntax Error Handling", func(t *testing.T) {
		request := ExecutionRequest{
			Language: "python",
			Version:  "3.12.0",
			Files: []File{
				{Content: "print('unclosed string"},
			},
		}

		reqBody, _ := json.Marshal(request)
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode) // Should return OK but with error in output

		var result ExecutionResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		// Should have non-zero exit code and error in stderr
		assert.NotEqual(t, 0, result.Run.Code)
		assert.NotEmpty(t, result.Run.Stderr)
	})

	t.Run("Memory Limit Test", func(t *testing.T) {
		memLimit := int64(10 * 1024 * 1024) // 10MB limit
		request := ExecutionRequest{
			Language:       "python",
			Version:        "3.12.0",
			RunMemoryLimit: &memLimit,
			Files: []File{
				{Content: "data = 'x' * (50 * 1024 * 1024)"}, // Try to allocate 50MB
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

		// Should fail due to memory limit
		assert.NotEqual(t, 0, result.Run.Code)
	})
}

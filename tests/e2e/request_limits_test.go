package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequestBodyLimit verifies that oversized POST/DELETE bodies are rejected with 413
func TestRequestBodyLimit(t *testing.T) {
	// Create a string comfortably larger than the 1MB default limit
	big := bytes.Repeat([]byte("A"), 2*1024*1024) // 2MB

	t.Run("Execute Body Too Large", func(t *testing.T) {
		// Valid request skeleton with a huge file content to exceed limit
		reqObj := map[string]interface{}{
			"language": "python",
			"version":  "3.12.0",
			"files": []map[string]string{
				{"content": string(big)},
			},
		}

		body, _ := json.Marshal(reqObj)
		resp, err := http.Post(APIBaseURL+"/api/v2/execute", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})

	t.Run("Packages POST Body Too Large", func(t *testing.T) {
		// Add a large padding field to exceed limit; BodyLimit should 413 on Content-Length before decode
		reqObj := map[string]interface{}{
			"language": "python",
			"version":  "3.12.0",
			"pad":      string(big),
		}

		body, _ := json.Marshal(reqObj)
		resp, err := http.Post(APIBaseURL+"/api/v2/packages", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})

	t.Run("Packages DELETE Body Too Large", func(t *testing.T) {
		reqObj := map[string]interface{}{
			"language": "python",
			"version":  "3.12.0",
			"pad":      string(big),
		}

		body, _ := json.Marshal(reqObj)
		req, err := http.NewRequest(http.MethodDelete, APIBaseURL+"/api/v2/packages", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})
}

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeExecution(t *testing.T) {
	tests := []struct {
		name        string
		language    string
		version     string
		code        string
		expected    string
		shouldError bool
	}{
		{
			name:     "Python Hello World",
			language: "python",
			version:  "3.12.0",
			code:     "print('Hello from CodeRunr Python!')",
			expected: "Hello from CodeRunr Python!\n",
		},
		{
			name:     "Python with NumPy",
			language: "python",
			version:  "3.12.0",
			code:     "import numpy as np; print(f'NumPy version: {np.__version__}')",
			expected: "NumPy version:",
		},
		{
			name:     "Go Hello World",
			language: "go",
			version:  "1.16.2",
			code:     "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello from CodeRunr Go!\")\n}",
			expected: "Hello from CodeRunr Go!\n",
		},
		{
			name:     "Java Hello World",
			language: "java",
			version:  "15.0.2",
			code:     "public class Main {\n    public static void main(String[] args) {\n        System.out.println(\"Hello from CodeRunr Java!\");\n    }\n}",
			expected: "Hello from CodeRunr Java!\n",
		},
		{
			name:        "Python Syntax Error",
			language:    "python",
			version:     "3.12.0",
			code:        "print('missing quote)",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := ExecutionRequest{
				Language: tt.language,
				Version:  tt.version,
				Files: []File{
					{Content: tt.code},
				},
			}

			result := executeCode(t, request)

			if tt.shouldError {
				assert.NotEqual(t, 0, result.Run.Code, "Expected non-zero exit code for error case")
				assert.NotEmpty(t, result.Run.Stderr, "Expected stderr for error case")
			} else {
				assert.Equal(t, 0, result.Run.Code, "Expected zero exit code")
				if strings.Contains(tt.expected, ":") && !strings.HasSuffix(tt.expected, "\n") {
					// For partial matches like "NumPy version:"
					assert.Contains(t, result.Run.Stdout, tt.expected)
				} else {
					assert.Equal(t, tt.expected, result.Run.Stdout)
				}
			}

			// Verify response structure
			assert.Equal(t, tt.language, result.Language)
			assert.Equal(t, tt.version, result.Version)
			assert.NotEmpty(t, result.Run.Output)
		})
	}
}

func TestCodeExecutionWithMultipleFiles(t *testing.T) {
	t.Run("Go with multiple files", func(t *testing.T) {
		t.Skip("Go multi-file projects are not supported due to package import limitations")

		request := ExecutionRequest{
			Language: "go",
			Version:  "1.16.2",
			Files: []File{
				{
					Name:    "main.go",
					Content: "package main\n\nimport (\n\t\"fmt\"\n\t\"./utils\"\n)\n\nfunc main() {\n\tfmt.Println(utils.GetMessage())\n}",
				},
				{
					Name:    "utils/utils.go",
					Content: "package utils\n\nfunc GetMessage() string {\n\treturn \"Hello from utils package!\"\n}",
				},
			},
		}

		result := executeCode(t, request)
		assert.Equal(t, 0, result.Run.Code)
		assert.Contains(t, result.Run.Stdout, "Hello from utils package!")
	})
}

func TestCodeExecutionPerformance(t *testing.T) {
	t.Run("Execution Time Limits", func(t *testing.T) {
		request := ExecutionRequest{
			Language: "python",
			Version:  "3.12.0",
			Files: []File{
				{Content: "import time; time.sleep(0.1); print('Done')"},
			},
		}

		result := executeCode(t, request)
		assert.Equal(t, 0, result.Run.Code)
		assert.Equal(t, "Done\n", result.Run.Stdout)

		// Check performance metrics
		assert.Greater(t, result.Run.Memory, int64(0), "Memory usage should be recorded")
		assert.Greater(t, result.Run.CPUTime, int64(0), "CPU time should be recorded")
		assert.Greater(t, result.Run.WallTime, int64(0), "Wall time should be recorded")
	})
}

// Helper function to execute code and return result
func executeCode(t *testing.T, request ExecutionRequest) ExecutionResult {
	reqBody, err := json.Marshal(request)
	require.NoError(t, err)

	resp, err := http.Post(
		APIBaseURL+"/api/v2/execute",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "API should return 200 OK")

	var result ExecutionResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	return result
}

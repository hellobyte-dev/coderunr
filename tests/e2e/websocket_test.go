package e2e

import (
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WebSocket message types matching the API
type WSMessage struct {
	Type     string      `json:"type"`
	Stream   string      `json:"stream,omitempty"`
	Data     string      `json:"data,omitempty"`
	Stage    string      `json:"stage,omitempty"`
	Signal   string      `json:"signal,omitempty"`
	Error    string      `json:"error,omitempty"`
	Code     *int        `json:"code,omitempty"`
	Language string      `json:"language,omitempty"`
	Version  string      `json:"version,omitempty"`
	Payload  interface{} `json:"payload,omitempty"`
}

func TestWebSocketAPI(t *testing.T) {
	// Skip WebSocket tests if services are not running
	if !checkServicesRunning() {
		t.Skip("Services not running, skipping WebSocket tests")
	}

	t.Run("WebSocket Basic Execution", func(t *testing.T) {
		conn := connectWebSocket(t)
		defer conn.Close()

		// Send init message
		initMsg := WSMessage{
			Type: "init",
			Payload: map[string]interface{}{
				"language": "python",
				"version":  "3.12.0",
				"files": []map[string]string{
					{"content": "print('Hello WebSocket!')"},
				},
			},
		}

		err := conn.WriteJSON(initMsg)
		require.NoError(t, err)

		// Read messages until completion
		foundOutput := false
		foundExit := false
		timeout := time.After(10 * time.Second)

		for !foundOutput || !foundExit {
			select {
			case <-timeout:
				if !foundOutput {
					t.Fatal("Timeout waiting for output")
				}
				if !foundExit {
					t.Fatal("Timeout waiting for exit")
				}
			default:
				var msg WSMessage
				if err := conn.ReadJSON(&msg); err != nil {
					t.Fatalf("Read error: %v", err)
				}

				t.Logf("Received: Type=%s, Data=%q, Code=%v, Error=%s", msg.Type, msg.Data, msg.Code, msg.Error)

				if msg.Type == "data" && msg.Stream == "stdout" && msg.Data == "Hello WebSocket!" {
					foundOutput = true
				}
				if msg.Type == "exit" && msg.Code != nil && *msg.Code == 0 {
					foundExit = true
				}
			}
		}

		assert.True(t, foundOutput, "Should receive output")
		assert.True(t, foundExit, "Should receive exit with code 0")
	})

	t.Run("WebSocket Error Handling", func(t *testing.T) {
		conn := connectWebSocket(t)
		defer conn.Close()

		// Send invalid message type
		invalidMsg := WSMessage{
			Type: "invalid_type",
		}

		err := conn.WriteJSON(invalidMsg)
		require.NoError(t, err)

		// Should receive error message
		var errorMsg WSMessage
		err = conn.ReadJSON(&errorMsg)
		require.NoError(t, err)
		assert.Equal(t, "error", errorMsg.Type)
		assert.Contains(t, errorMsg.Error, "Unknown message type")
	})

	t.Run("WebSocket Invalid Language", func(t *testing.T) {
		conn := connectWebSocket(t)
		defer conn.Close()

		// Send init with invalid language
		initMsg := WSMessage{
			Type: "init",
			Payload: map[string]interface{}{
				"language": "nonexistent_language",
				"version":  "1.0.0",
				"files": []map[string]string{
					{"content": "print('test')"},
				},
			},
		}

		err := conn.WriteJSON(initMsg)
		require.NoError(t, err)

		// Should receive error message
		var errorMsg WSMessage
		err = conn.ReadJSON(&errorMsg)
		require.NoError(t, err)
		assert.Equal(t, "error", errorMsg.Type)
		assert.Contains(t, errorMsg.Error, "Runtime not found")
	})

	t.Run("WebSocket Python Syntax Error", func(t *testing.T) {
		conn := connectWebSocket(t)
		defer conn.Close()

		// Send init with syntax error
		initMsg := WSMessage{
			Type: "init",
			Payload: map[string]interface{}{
				"language": "python",
				"version":  "3.12.0",
				"files": []map[string]string{
					{"content": "print('unclosed string"},
				},
			},
		}

		err := conn.WriteJSON(initMsg)
		require.NoError(t, err)

		// Read until exit with non-zero code
		foundError := false
		timeout := time.After(10 * time.Second)

		for !foundError {
			select {
			case <-timeout:
				t.Fatal("Timeout waiting for syntax error")
			default:
				var msg WSMessage
				if err := conn.ReadJSON(&msg); err != nil {
					t.Fatalf("Read error: %v", err)
				}

				if msg.Type == "exit" && msg.Code != nil && *msg.Code != 0 {
					foundError = true
				}
			}
		}

		assert.True(t, foundError, "Should receive exit with non-zero code for syntax error")
	})
}

// Helper function to connect to WebSocket
func connectWebSocket(t *testing.T) *websocket.Conn {
	u := url.URL{Scheme: "ws", Host: "localhost:2000", Path: "/api/v2/connect"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err, "Failed to connect to WebSocket")

	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	return conn
}

// Helper function to check if services are running
func checkServicesRunning() bool {
	// Simply try to connect, and if it succeeds, services are running
	u := url.URL{Scheme: "ws", Host: "localhost:2000", Path: "/api/v2/connect"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

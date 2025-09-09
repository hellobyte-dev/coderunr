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
	Message  string      `json:"message,omitempty"`
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
		foundRuntime := false
		foundInitAck := false
		foundStageStart := false
		foundOutput := false
		foundStageEnd := false
		timeout := time.After(10 * time.Second)

		for !foundOutput || !foundStageEnd {
			select {
			case <-timeout:
				if !foundRuntime {
					t.Fatal("Timeout waiting for runtime")
				}
				if !foundInitAck {
					t.Fatal("Timeout waiting for init_ack")
				}
				if !foundStageStart {
					t.Fatal("Timeout waiting for stage_start")
				}
				if !foundOutput {
					t.Fatal("Timeout waiting for output")
				}
				if !foundStageEnd {
					t.Fatal("Timeout waiting for stage_end")
				}
			default:
				var msg WSMessage
				if err := conn.ReadJSON(&msg); err != nil {
					t.Fatalf("Read error: %v", err)
				}

				t.Logf("Received: Type=%s, Data=%q, Code=%v, Error=%s", msg.Type, msg.Data, msg.Code, msg.Error)

				if msg.Type == "runtime" {
					foundRuntime = true
				}
				if msg.Type == "init_ack" {
					foundInitAck = true
				}
				if msg.Type == "stage_start" && msg.Stage == "run" {
					foundStageStart = true
				}
				if msg.Type == "data" && msg.Stream == "stdout" && msg.Data == "Hello WebSocket!" {
					foundOutput = true
				}
				if msg.Type == "stage_end" && msg.Stage == "run" && msg.Code != nil && *msg.Code == 0 {
					foundStageEnd = true
				}
			}
		}

		assert.True(t, foundOutput, "Should receive output")
		assert.True(t, foundStageEnd, "Should receive stage_end with code 0")
	})

	t.Run("WebSocket Output Limit", func(t *testing.T) {
		conn := connectWebSocket(t)
		defer conn.Close()

		// Python program that prints many lines to exceed default output limit (~1024 bytes)
		code := `for i in range(100):
    print('X'*100)`

		initMsg := WSMessage{
			Type: "init",
			Payload: map[string]interface{}{
				"language": "python",
				"version":  "3.12.0",
				"files": []map[string]string{
					{"content": code},
				},
			},
		}

		require.NoError(t, conn.WriteJSON(initMsg))

		gotLimitErr := false
		gotStageEnd := false
		deadline := time.After(10 * time.Second)

		for !gotStageEnd {
			select {
			case <-deadline:
				t.Fatal("Timeout waiting for output limit handling")
			default:
				var msg WSMessage
				require.NoError(t, conn.ReadJSON(&msg))
				if msg.Type == "error" {
					if msg.Message != "" {
						if assert.Contains(t, msg.Message, "limit exceeded") {
							gotLimitErr = true
						}
					} else if msg.Error != "" {
						if assert.Contains(t, msg.Error, "limit exceeded") {
							gotLimitErr = true
						}
					}
				}
				if msg.Type == "stage_end" && msg.Stage == "run" {
					gotStageEnd = true
				}
			}
		}

		assert.True(t, gotLimitErr, "Should receive output limit exceeded error")
		assert.True(t, gotStageEnd, "Should receive stage_end after limit exceeded")
	})

	t.Run("WebSocket Init With Top-Level Fields", func(t *testing.T) {
		conn := connectWebSocket(t)
		defer conn.Close()

		// Send init with top-level fields instead of payload
		initMsg := map[string]interface{}{
			"type":     "init",
			"language": "python",
			"version":  "3.12.0",
			"files": []map[string]string{
				{"content": "print('Hello TL!')"},
			},
		}

		require.NoError(t, conn.WriteJSON(initMsg))

		// Expect runtime -> init_ack -> stage_start(run) -> stdout -> stage_end 0
		gotOut := false
		gotRuntime := false
		gotInitAck := false
		gotStageStart := false
		gotStageEnd := false
		deadline := time.After(10 * time.Second)
		for !gotOut || !gotStageEnd {
			select {
			case <-deadline:
				t.Fatal("Timeout waiting for messages")
			default:
				var msg WSMessage
				require.NoError(t, conn.ReadJSON(&msg))
				if msg.Type == "runtime" {
					gotRuntime = true
				}
				if msg.Type == "init_ack" {
					gotInitAck = true
				}
				if msg.Type == "stage_start" && msg.Stage == "run" {
					gotStageStart = true
				}
				if msg.Type == "data" && msg.Stream == "stdout" && msg.Data == "Hello TL!" {
					gotOut = true
				}
				if msg.Type == "stage_end" && msg.Stage == "run" && msg.Code != nil && *msg.Code == 0 {
					gotStageEnd = true
				}
			}
		}
		assert.True(t, gotRuntime)
		assert.True(t, gotInitAck)
		assert.True(t, gotStageStart)
		assert.True(t, gotOut)
		assert.True(t, gotStageEnd)
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
		// Accept either message or error field for compatibility
		if errorMsg.Message != "" {
			assert.Contains(t, errorMsg.Message, "Unknown message type")
		} else {
			assert.Contains(t, errorMsg.Error, "Unknown message type")
		}
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
		if errorMsg.Message != "" {
			assert.Contains(t, errorMsg.Message, "Runtime not found")
		} else {
			assert.Contains(t, errorMsg.Error, "Runtime not found")
		}
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

		// Read until stage_end with non-zero code
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

				if msg.Type == "stage_end" && msg.Stage == "run" && msg.Code != nil && *msg.Code != 0 {
					foundError = true
				}
			}
		}

		assert.True(t, foundError, "Should receive stage_end with non-zero code for syntax error")
	})

	t.Run("WebSocket Event Ordering With Init Ack", func(t *testing.T) {
		conn := connectWebSocket(t)
		defer conn.Close()

		initMsg := WSMessage{
			Type: "init",
			Payload: map[string]interface{}{
				"language": "python",
				"version":  "3.12.0",
				"files": []map[string]string{
					{"content": "print('order')"},
				},
			},
		}
		require.NoError(t, conn.WriteJSON(initMsg))

		// Expect strict ordering: runtime -> init_ack -> stage_start(run)
		var seq []string
		deadline := time.After(5 * time.Second)
		for len(seq) < 3 {
			select {
			case <-deadline:
				t.Fatalf("Timeout waiting for sequence, got: %v", seq)
			default:
				var msg WSMessage
				require.NoError(t, conn.ReadJSON(&msg))
				if msg.Type == "runtime" || msg.Type == "init_ack" || (msg.Type == "stage_start" && msg.Stage == "run") {
					seq = append(seq, msg.Type)
				}
			}
		}
		assert.Equal(t, []string{"runtime", "init_ack", "stage_start"}, seq)
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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coderunr/api/internal/job"
	"github.com/coderunr/api/internal/runtime"
	"github.com/coderunr/api/internal/types"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// WebSocketConnection represents a WebSocket connection
type WebSocketConnection struct {
	conn       *websocket.Conn
	job        *job.Job
	eventBus   chan types.WebSocketMessage
	jobManager *job.Manager
	logger     *logrus.Entry
	mutex      sync.Mutex
	closed     bool
}

// HandleWebSocket handles WebSocket connections for interactive execution
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.WithError(err).Error("WebSocket upgrade failed")
		return
	}

	wsConn := &WebSocketConnection{
		conn:       conn,
		eventBus:   make(chan types.WebSocketMessage, 100),
		jobManager: h.jobManager,
		logger:     h.logger.WithField("component", "websocket"),
		closed:     false,
	}

	// Set connection timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Start event sender goroutine
	go wsConn.eventSender()

	// Set up initialization timeout
	initTimeout := time.NewTimer(1 * time.Second)
	defer initTimeout.Stop()

	go func() {
		<-initTimeout.C
		if wsConn.job == nil {
			wsConn.sendError("Initialization timeout")
			wsConn.close(4001, "Initialization Timeout")
		}
	}()

	// Handle incoming messages
	wsConn.handleMessages(r.Context())
}

// handleMessages handles incoming WebSocket messages
func (wsConn *WebSocketConnection) handleMessages(ctx context.Context) {
	defer wsConn.close(1000, "Connection closed")

	for {
		var msg types.WebSocketMessage
		if err := wsConn.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				wsConn.logger.WithError(err).Error("WebSocket read error")
			}
			break
		}

		// Reset read deadline
		wsConn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		if err := wsConn.handleMessage(ctx, msg); err != nil {
			wsConn.sendError(err.Error())
			break
		}
	}
}

// handleMessage handles a single WebSocket message
func (wsConn *WebSocketConnection) handleMessage(ctx context.Context, msg types.WebSocketMessage) error {
	switch msg.Type {
	case "init":
		return wsConn.handleInit(ctx, msg)
	case "data":
		return wsConn.handleData(msg)
	case "signal":
		return wsConn.handleSignal(msg)
	default:
		return wsConn.sendError("Unknown message type: " + msg.Type)
	}
}

// handleInit handles job initialization
func (wsConn *WebSocketConnection) handleInit(ctx context.Context, msg types.WebSocketMessage) error {
	if wsConn.job != nil {
		wsConn.close(4000, "Already Initialized")
		return nil
	}

	// Parse job request from message payload
	requestBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return wsConn.sendError("Invalid request payload")
	}

	var request types.JobRequest
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		return wsConn.sendError("Invalid job request")
	}

	// Validate request
	if err := wsConn.validateJobRequest(&request); err != nil {
		return wsConn.sendError(err.Error())
	}

	// Find runtime
	rt, err := runtime.GetLatestRuntimeMatchingLanguageVersion(request.Language, request.Version)
	if err != nil {
		return wsConn.sendError("Runtime not found: " + request.Language + "-" + request.Version)
	}

	// Create job
	wsConn.job = wsConn.jobManager.NewJob(rt, &request)

	// Send runtime info
	wsConn.sendMessage(types.WebSocketMessage{
		Type: "runtime",
		Payload: map[string]string{
			"language": rt.Language,
			"version":  rt.Version.String(),
		},
	})

	// Execute job in background
	go wsConn.executeJob(ctx)

	return nil
}

// handleData handles stdin data
func (wsConn *WebSocketConnection) handleData(msg types.WebSocketMessage) error {
	if wsConn.job == nil {
		wsConn.close(4003, "Not yet initialized")
		return nil
	}

	if msg.Stream != "stdin" {
		wsConn.close(4004, "Can only write to stdin")
		return nil
	}

	// TODO: Implement stdin writing to running process
	// This would require modifying the job execution to support real-time stdin
	return nil
}

// handleSignal handles process signals
func (wsConn *WebSocketConnection) handleSignal(msg types.WebSocketMessage) error {
	if wsConn.job == nil {
		wsConn.close(4003, "Not yet initialized")
		return nil
	}

	// Validate signal
	validSignals := []string{"SIGTERM", "SIGKILL", "SIGINT"}
	valid := false
	for _, sig := range validSignals {
		if msg.Signal == sig {
			valid = true
			break
		}
	}

	if !valid {
		wsConn.close(4005, "Invalid signal")
		return nil
	}

	// TODO: Implement signal sending to running process
	return nil
}

// executeJob executes the job and sends events
func (wsConn *WebSocketConnection) executeJob(ctx context.Context) {
	defer func() {
		wsConn.close(4999, "Job Completed")
	}()

	// Execute the job
	result, err := wsConn.job.Execute(ctx)
	if err != nil {
		wsConn.sendError("Execution failed: " + err.Error())
		return
	}

	// Send compilation stage if it exists
	if result.Compile != nil {
		wsConn.sendStageResult("compile", result.Compile)
	}

	// Send run stage
	if result.Run != nil {
		wsConn.sendStageResult("run", result.Run)
	}

	// Send completion
	wsConn.sendMessage(types.WebSocketMessage{
		Type:    "complete",
		Payload: result,
	})
}

// sendStageResult sends stage execution result
func (wsConn *WebSocketConnection) sendStageResult(stage string, result *types.StageResult) {
	// Send stage start
	wsConn.sendMessage(types.WebSocketMessage{
		Type:  "stage",
		Stage: stage,
	})

	// Send stdout data
	if result.Stdout != "" {
		wsConn.sendMessage(types.WebSocketMessage{
			Type:   "data",
			Stream: "stdout",
			Data:   result.Stdout,
		})
	}

	// Send stderr data
	if result.Stderr != "" {
		wsConn.sendMessage(types.WebSocketMessage{
			Type:   "data",
			Stream: "stderr",
			Data:   result.Stderr,
		})
	}

	// Send exit information
	wsConn.sendMessage(types.WebSocketMessage{
		Type:  "exit",
		Stage: stage,
		Payload: map[string]interface{}{
			"code":   result.Code,
			"signal": result.Signal,
		},
	})
}

// eventSender sends events to the WebSocket client
func (wsConn *WebSocketConnection) eventSender() {
	for event := range wsConn.eventBus {
		wsConn.mutex.Lock()
		if wsConn.closed {
			wsConn.mutex.Unlock()
			break
		}

		wsConn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := wsConn.conn.WriteJSON(event); err != nil {
			wsConn.logger.WithError(err).Error("Failed to send WebSocket message")
			wsConn.mutex.Unlock()
			break
		}
		wsConn.mutex.Unlock()
	}
}

// sendMessage sends a message to the client
func (wsConn *WebSocketConnection) sendMessage(msg types.WebSocketMessage) {
	select {
	case wsConn.eventBus <- msg:
	default:
		wsConn.logger.Warn("Event bus full, dropping message")
	}
}

// sendError sends an error message
func (wsConn *WebSocketConnection) sendError(message string) error {
	wsConn.sendMessage(types.WebSocketMessage{
		Type:  "error",
		Error: message,
	})
	return nil
}

// close closes the WebSocket connection
func (wsConn *WebSocketConnection) close(code int, message string) {
	wsConn.mutex.Lock()
	defer wsConn.mutex.Unlock()

	if wsConn.closed {
		return
	}

	wsConn.closed = true
	close(wsConn.eventBus)

	wsConn.conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(code, message),
		time.Now().Add(time.Second))

	wsConn.conn.Close()
}

// validateJobRequest validates the job request for WebSocket
func (wsConn *WebSocketConnection) validateJobRequest(request *types.JobRequest) error {
	if request.Language == "" {
		return wsConn.sendError("language is required")
	}

	if request.Version == "" {
		return wsConn.sendError("version is required")
	}

	if len(request.Files) == 0 {
		return wsConn.sendError("files array is required")
	}

	for i, file := range request.Files {
		if file.Content == "" {
			return wsConn.sendError("files[" + string(rune(i)) + "].content is required")
		}
	}

	return nil
}

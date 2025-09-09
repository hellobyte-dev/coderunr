package handler

import (
	"context"
	"encoding/json"
	"fmt"
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

	// Set up initialization timeout (more tolerant for network/JSON delays)
	initTimeout := time.NewTimer(5 * time.Second)
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
		// Read raw message to support both payload and top-level init formats
		_, data, err := wsConn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				wsConn.logger.WithError(err).Error("WebSocket read error")
			}
			break
		}

		// Reset read deadline
		wsConn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Determine message type
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			wsConn.sendError("Invalid message JSON")
			break
		}
		msgType, _ := raw["type"].(string)

		switch msgType {
		case "init":
			if err := wsConn.handleInitRaw(ctx, raw); err != nil {
				wsConn.sendError(err.Error())
				return
			}
		case "data", "signal":
			var msg types.WebSocketMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				wsConn.sendError("Invalid message fields")
				return
			}
			if err := wsConn.handleMessage(ctx, msg); err != nil {
				wsConn.sendError(err.Error())
				return
			}
		default:
			// For unknown message types, send an error but keep connection open
			// so clients can continue without being disconnected.
			wsConn.sendError("Unknown message type: " + msgType)
			continue
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

	// Send runtime info then init_ack to acknowledge initialization
	wsConn.sendMessage(types.WebSocketMessage{
		Type:     "runtime",
		Language: rt.Language,
		Version:  rt.Version.String(),
	})
	wsConn.sendMessage(types.WebSocketMessage{Type: "init_ack"})

	// Execute job in background
	go wsConn.executeJob(ctx)

	return nil
}

// handleInitRaw handles init from a raw JSON map supporting both payload and top-level fields
func (wsConn *WebSocketConnection) handleInitRaw(ctx context.Context, raw map[string]interface{}) error {
	if wsConn.job != nil {
		wsConn.close(4000, "Already Initialized")
		return nil
	}

	// Determine the request map (payload or top-level)
	var reqMap map[string]interface{}
	if p, ok := raw["payload"]; ok {
		if m, ok := p.(map[string]interface{}); ok {
			reqMap = m
		}
	}
	if reqMap == nil {
		reqMap = raw
	}

	// Build JobRequest
	request, err := buildJobRequestFromMap(reqMap)
	if err != nil {
		return wsConn.sendError(err.Error())
	}

	// Validate
	if err := wsConn.validateJobRequest(request); err != nil {
		return wsConn.sendError(err.Error())
	}

	// Find runtime
	rt, err := runtime.GetLatestRuntimeMatchingLanguageVersion(request.Language, request.Version)
	if err != nil {
		return wsConn.sendError("Runtime not found: " + request.Language + "-" + request.Version)
	}

	wsConn.job = wsConn.jobManager.NewJob(rt, request)

	// Send runtime info (top-level fields) then init_ack
	wsConn.sendMessage(types.WebSocketMessage{Type: "runtime", Language: rt.Language, Version: rt.Version.String()})
	wsConn.sendMessage(types.WebSocketMessage{Type: "init_ack"})

	go wsConn.executeJob(ctx)
	return nil
}

// buildJobRequestFromMap converts an init map into a JobRequest
func buildJobRequestFromMap(m map[string]interface{}) (*types.JobRequest, error) {
	jr := &types.JobRequest{}
	if v, ok := m["language"].(string); ok {
		jr.Language = v
	}
	if v, ok := m["version"].(string); ok {
		jr.Version = v
	}
	if v, ok := m["stdin"].(string); ok {
		jr.Stdin = v
	}
	if v, ok := m["args"].([]interface{}); ok {
		args := make([]string, 0, len(v))
		for _, a := range v {
			if s, ok := a.(string); ok {
				args = append(args, s)
			}
		}
		jr.Args = args
	}

	// files: accept multiple slice element types
	if rawFiles, ok := m["files"]; ok {
		switch vv := rawFiles.(type) {
		case []interface{}:
			files := make([]types.CodeFile, 0, len(vv))
			for _, f := range vv {
				if fm, ok := f.(map[string]interface{}); ok {
					cf := types.CodeFile{}
					if s, ok := fm["name"].(string); ok {
						cf.Name = s
					}
					if s, ok := fm["content"].(string); ok {
						cf.Content = s
					} else {
						return nil, fmt.Errorf("files[].content must be string")
					}
					if s, ok := fm["encoding"].(string); ok {
						cf.Encoding = s
					}
					files = append(files, cf)
				}
			}
			jr.Files = files
		case []map[string]interface{}:
			files := make([]types.CodeFile, 0, len(vv))
			for _, fm := range vv {
				cf := types.CodeFile{}
				if s, ok := fm["name"].(string); ok {
					cf.Name = s
				}
				if s, ok := fm["content"].(string); ok {
					cf.Content = s
				} else {
					return nil, fmt.Errorf("files[].content must be string")
				}
				if s, ok := fm["encoding"].(string); ok {
					cf.Encoding = s
				}
				files = append(files, cf)
			}
			jr.Files = files
		case []map[string]string:
			files := make([]types.CodeFile, 0, len(vv))
			for _, fm := range vv {
				cf := types.CodeFile{}
				if s, ok := fm["name"]; ok {
					cf.Name = s
				}
				if s, ok := fm["content"]; ok {
					cf.Content = s
				} else {
					return nil, fmt.Errorf("files[].content must be string")
				}
				if s, ok := fm["encoding"]; ok {
					cf.Encoding = s
				}
				files = append(files, cf)
			}
			jr.Files = files
		}
	}

	// numeric helpers
	toIntPtr := func(key string) *int {
		if val, ok := m[key]; ok {
			switch x := val.(type) {
			case float64:
				xi := int(x)
				return &xi
			case int:
				xi := x
				return &xi
			}
		}
		return nil
	}
	toInt64Ptr := func(key string) *int64 {
		if val, ok := m[key]; ok {
			switch x := val.(type) {
			case float64:
				xi := int64(x)
				return &xi
			case int:
				xi := int64(x)
				return &xi
			}
		}
		return nil
	}

	jr.CompileTimeout = toIntPtr("compile_timeout")
	jr.RunTimeout = toIntPtr("run_timeout")
	jr.CompileCPUTime = toIntPtr("compile_cpu_time")
	jr.RunCPUTime = toIntPtr("run_cpu_time")
	jr.CompileMemoryLimit = toInt64Ptr("compile_memory_limit")
	jr.RunMemoryLimit = toInt64Ptr("run_memory_limit")

	return jr, nil
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

	// Write to job's stdin channel
	if err := wsConn.job.WriteStdin(msg.Data); err != nil {
		wsConn.logger.WithError(err).Error("Failed to write to stdin")
		wsConn.sendError("Failed to write to stdin: " + err.Error())
		return err
	}

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

	// Send signal to running process
	if err := wsConn.job.SendSignal(msg.Signal); err != nil {
		wsConn.logger.WithError(err).Error("Failed to send signal")
		wsConn.sendError("Failed to send signal: " + err.Error())
		return err
	}

	return nil
}

// executeJob executes the job and sends events
func (wsConn *WebSocketConnection) executeJob(ctx context.Context) {
	defer func() {
		wsConn.close(4999, "Job Completed")
	}()

	// Start listening to job events
	go func() {
		for event := range wsConn.job.EventChannel {
			wsConn.handleJobEvent(event)
		}
	}()

	// Execute the job with streaming
	if err := wsConn.job.ExecuteStream(ctx); err != nil {
		wsConn.sendError("Execution failed: " + err.Error())
		return
	}
}

// handleJobEvent handles events from job execution
func (wsConn *WebSocketConnection) handleJobEvent(event types.StreamEvent) {
	switch event.Type {
	case "runtime":
		wsConn.sendMessage(types.WebSocketMessage{
			Type:     "runtime",
			Language: wsConn.job.Runtime.Language,
			Version:  wsConn.job.Runtime.Version.String(),
		})
	case "stage_start":
		wsConn.sendMessage(types.WebSocketMessage{Type: "stage_start", Stage: event.Stage})
	case "stage_end":
		// include exit code (always present as pointer)
		code := event.Code
		wsConn.sendMessage(types.WebSocketMessage{Type: "stage_end", Stage: event.Stage, Code: &code})
	case "data":
		wsConn.sendMessage(types.WebSocketMessage{
			Type:   "data",
			Stream: event.Stream,
			Data:   event.Data,
		})
	case "exit":
		wsConn.sendMessage(types.WebSocketMessage{
			Type:  "exit",
			Stage: event.Stage,
			Code:  &event.Code,
		})
	case "error":
		if event.Error != nil {
			wsConn.sendError(event.Error.Error())
		}
	}
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
	// Ensure we don't send on a closed channel; guard with mutex to avoid race with close()
	wsConn.mutex.Lock()
	if wsConn.closed {
		wsConn.mutex.Unlock()
		return
	}
	select {
	case wsConn.eventBus <- msg:
		// sent
	default:
		wsConn.logger.Warn("Event bus full, dropping message")
	}
	wsConn.mutex.Unlock()
}

// sendError sends an error message
func (wsConn *WebSocketConnection) sendError(message string) error {
	wsConn.sendMessage(types.WebSocketMessage{
		Type:    "error",
		Message: message,
		Error:   message, // keep for backward-compat with existing tests/clients
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

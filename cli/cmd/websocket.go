package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

type WSExecuteRequest struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type WSJobPayload struct {
	Language           string     `json:"language"`
	Version            string     `json:"version"`
	Files              []FileData `json:"files"`
	Args               []string   `json:"args,omitempty"`
	Stdin              string     `json:"stdin,omitempty"`
	CompileTimeout     *int       `json:"compile_timeout,omitempty"`
	RunTimeout         *int       `json:"run_timeout,omitempty"`
	CompileMemoryLimit *int64     `json:"compile_memory_limit,omitempty"`
	RunMemoryLimit     *int64     `json:"run_memory_limit,omitempty"`
}

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
	Message  string      `json:"message,omitempty"`
	Payload  interface{} `json:"payload,omitempty"`
}

func executeInteractiveWS(baseURL, language, version string, files []FileData, args []string,
	showStatus, verbose bool) error {

	// Convert HTTP URL to WebSocket URL
	wsURL, err := convertToWebSocketURL(baseURL)
	if err != nil {
		return fmt.Errorf("failed to convert URL: %w", err)
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"/api/v2/connect", nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	defer conn.Close()

	if verbose {
		fmt.Printf("Connected to WebSocket: %s\n", wsURL+"/api/v2/connect")
	}

	// Setup signal handling and context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Writer mutex to serialize writes
	var writeMu sync.Mutex
	writeJSON := func(v interface{}) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}

	// System signals forwarding
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	signalsCh := make(chan os.Signal, 4)
	signal.Notify(signalsCh, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	// Channel to receive messages
	messages := make(chan WSMessage, 10)

	// Start message reader goroutine
	go func() {
		defer close(messages)
		for {
			var msg WSMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
					fmt.Printf("WebSocket error: %v\n", err)
				}
				// Connection closed normally, exit quietly
				return
			}
			select {
			case messages <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Forward stdin to WS as data stream
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				_ = writeJSON(map[string]interface{}{
					"type":   "data",
					"stream": "stdin",
					"data":   string(buf[:n]),
				})
			}
			if err != nil {
				if err != io.EOF {
					// Non-fatal: just stop forwarding
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	// Forward OS signals to WS
	go func() {
		for {
			select {
			case sig := <-signalsCh:
				sigName := toSignalName(sig)
				_ = writeJSON(map[string]interface{}{
					"type":   "signal",
					"signal": sigName,
				})
			case <-ctx.Done():
				return
			}
		}
	}()

	// Send init request
	payload := WSJobPayload{
		Language: language,
		Version:  version,
		Files:    files,
		Args:     args,
	}

	request := WSExecuteRequest{
		Type:    "init",
		Payload: payload,
	}

	if err := writeJSON(request); err != nil {
		return fmt.Errorf("failed to send execute request: %w", err)
	}

	if verbose {
		reqJSON, _ := json.Marshal(request)
		fmt.Printf("Sent init request for %s %s: %s\n", language, version, string(reqJSON))
	}

	// Process messages
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	for {
		select {
		case <-interrupt:
			// Send SIGINT to remote instead of immediate close
			_ = writeJSON(map[string]interface{}{
				"type":   "signal",
				"signal": "SIGINT",
			})
			// Do not return; let server handle termination

		case msg, ok := <-messages:
			if !ok {
				// Connection closed, job completed
				if verbose {
					fmt.Println("Connection closed, execution completed")
				}
				return nil
			}

			switch msg.Type {
			case "data":
				// Handle data messages with stream and data fields
				switch msg.Stream {
				case "stdout":
					fmt.Print(msg.Data)
				case "stderr":
					fmt.Print(msg.Data)
				default:
					if verbose && msg.Stream != "" {
						fmt.Printf("Unknown stream: %s\n", msg.Stream)
					}
				}

			case "exit": // backward compatibility
				if showStatus || verbose {
					bold.Printf("\n== %s Exit ==\n", strings.Title(msg.Stage))

					if msg.Code != nil {
						if *msg.Code == 0 {
							fmt.Print("Exit Code: ")
							green.Printf("%d\n", *msg.Code)
						} else {
							fmt.Print("Exit Code: ")
							red.Printf("%d\n", *msg.Code)
						}
					}

					if msg.Signal != "" {
						fmt.Print("Signal: ")
						yellow.Printf("%s\n", msg.Signal)
					}
				}

			case "stage_start":
				if showStatus || verbose {
					bold.Printf("== %s ==\n", strings.Title(msg.Stage))
				}

			case "stage_end":
				if showStatus || verbose {
					bold.Printf("\n== %s Exit ==\n", strings.Title(msg.Stage))
					if msg.Code != nil {
						if *msg.Code == 0 {
							fmt.Print("Exit Code: ")
							green.Printf("%d\n", *msg.Code)
						} else {
							fmt.Print("Exit Code: ")
							red.Printf("%d\n", *msg.Code)
						}
					}
					if msg.Signal != "" {
						fmt.Print("Signal: ")
						yellow.Printf("%s\n", msg.Signal)
					}
				}

			case "runtime":
				if verbose {
					fmt.Printf("Runtime: %s %s\n", msg.Language, msg.Version)
				}

			case "stage": // compatibility
				if showStatus || verbose {
					bold.Printf("== %s ==\n", strings.Title(msg.Stage))
				}

			case "init_ack":
				if showStatus || verbose {
					bold.Printf("== Initialization Acknowledged ==\n")
				}

			case "error":
				// Prefer unified {message} field
				errMsg := msg.Message
				if errMsg == "" {
					errMsg = msg.Error
				}
				red.Printf("Error: %s\n", errMsg)
				return fmt.Errorf("execution error: %s", errMsg)

			default:
				if verbose {
					fmt.Printf("Unknown message type: %s\n", msg.Type)
				}
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func convertToWebSocketURL(httpURL string) (string, error) {
	u, err := url.Parse(httpURL)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	return u.String(), nil
}

func toSignalName(sig os.Signal) string {
	// Map common signals to names
	switch s := sig.(type) {
	case syscall.Signal:
		switch s {
		case syscall.SIGINT:
			return "SIGINT"
		case syscall.SIGTERM:
			return "SIGTERM"
		case syscall.SIGQUIT:
			return "SIGQUIT"
		case syscall.SIGHUP:
			return "SIGHUP"
		default:
			return fmt.Sprintf("SIG%s", s.String())
		}
	default:
		return sig.String()
	}
}

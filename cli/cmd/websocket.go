package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

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

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

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

	if err := conn.WriteJSON(request); err != nil {
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
			fmt.Println("\nReceived interrupt signal, closing connection...")

			// Send close message
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

			// Wait for close
			time.Sleep(time.Second)
			return nil

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

			case "exit":
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

			case "stage":
				if showStatus || verbose {
					bold.Printf("== %s ==\n", strings.Title(msg.Stage))
				}

			case "error":
				red.Printf("Error: %s\n", msg.Error)
				if msg.Message != "" {
					red.Printf("Message: %s\n", msg.Message)
				}
				return fmt.Errorf("execution error: %s", msg.Error)

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

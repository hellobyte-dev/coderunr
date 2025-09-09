package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type ExecuteRequest struct {
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

type FileData struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	Encoding string `json:"encoding,omitempty"`
}

type ExecuteResponse struct {
	Language string      `json:"language"`
	Version  string      `json:"version"`
	Run      StageResult `json:"run"`
	Compile  StageResult `json:"compile,omitempty"`
}

type StageResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Code     *int   `json:"code"`
	Signal   string `json:"signal,omitempty"`
	Memory   int64  `json:"memory"`
	CPUTime  int64  `json:"cpu_time"`
	WallTime int64  `json:"wall_time"`
}

func NewExecuteCommand() *cobra.Command {
	var (
		languageVersion string
		readStdin       bool
		runTimeout      int
		compileTimeout  int
		additionalFiles []string
		interactive     bool
		status          bool
		args            []string
	)

	cmd := &cobra.Command{
		Use:     "execute <language> <file> [args...]",
		Aliases: []string{"run", "exec"},
		Short:   "Execute code file with specified language",
		Long: `Execute a code file using CodeRunr execution engine.

Examples:
  # Execute Python script
  coderunr execute python script.py

  # Execute with specific version
  coderunr execute python script.py -l 3.9.4

  # Execute with arguments
  coderunr execute go main.go -- arg1 arg2

  # Execute interactively with WebSocket
  coderunr execute python script.py -t

  # Execute with additional files
  coderunr execute python main.py -f utils.py -f config.json`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			language := cmdArgs[0]
			filename := cmdArgs[1]
			if len(cmdArgs) > 2 {
				args = cmdArgs[2:]
			}

			// Read main file
			files, err := readFiles(append([]string{filename}, additionalFiles...))
			if err != nil {
				return fmt.Errorf("failed to read files: %w", err)
			}

			// Read stdin if requested
			var stdin string
			if readStdin {
				stdinBytes, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				stdin = string(stdinBytes)
			}

			url, _ := cmd.Flags().GetString("url")
			verbose, _ := cmd.Flags().GetBool("verbose")

			if interactive {
				return executeInteractive(url, language, languageVersion, files, args, status, verbose)
			}
			return executeNonInteractive(url, language, languageVersion, files, args, stdin,
				runTimeout, compileTimeout, verbose)
		},
	}

	cmd.Flags().StringVarP(&languageVersion, "language-version", "l", "*", "Language version to use")
	cmd.Flags().BoolVarP(&readStdin, "stdin", "i", false, "Read input from stdin")
	cmd.Flags().IntVarP(&runTimeout, "run-timeout", "r", 3000, "Run timeout in milliseconds")
	cmd.Flags().IntVarP(&compileTimeout, "compile-timeout", "c", 10000, "Compile timeout in milliseconds")
	cmd.Flags().StringSliceVarP(&additionalFiles, "files", "f", nil, "Additional files to include")
	cmd.Flags().BoolVarP(&interactive, "interactive", "t", false, "Run interactively using WebSocket")
	cmd.Flags().BoolVarP(&status, "status", "s", false, "Show additional status information")

	return cmd
}

func readFiles(filenames []string) ([]FileData, error) {
	var files []FileData

	for _, filename := range filenames {
		content, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
		}

		// Detect encoding - simple check for binary content
		encoding := "utf8"
		if !isUTF8(content) {
			encoding = "base64"
		}

		files = append(files, FileData{
			Name:     filepath.Base(filename),
			Content:  string(content),
			Encoding: encoding,
		})
	}

	return files, nil
}

func isUTF8(data []byte) bool {
	// Simple heuristic: check for null bytes or replacement characters
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}

func executeNonInteractive(url, language, version string, files []FileData, args []string,
	stdin string, runTimeout, compileTimeout int, verbose bool) error {

	request := ExecuteRequest{
		Language: language,
		Version:  version,
		Files:    files,
		Args:     args,
		Stdin:    stdin,
	}

	if runTimeout != 3000 {
		request.RunTimeout = &runTimeout
	}
	if compileTimeout != 10000 {
		request.CompileTimeout = &compileTimeout
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(url+"/api/v2/execute", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("execution failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response ExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return printExecutionResult(response, verbose)
}

func printExecutionResult(response ExecuteResponse, verbose bool) error {
	// Print compile stage if present
	if response.Compile.Stdout != "" || response.Compile.Stderr != "" || response.Compile.Code != nil || response.Compile.Signal != "" || response.Compile.Memory != 0 || response.Compile.CPUTime != 0 || response.Compile.WallTime != 0 {
		printStage("Compile", response.Compile, verbose)
	}

	// Print run stage
	printStage("Run", response.Run, verbose)

	return nil
}

func printStage(stageName string, result StageResult, verbose bool) {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)

	bold.Printf("== %s ==\n", stageName)

	if result.Stdout != "" {
		bold.Println("STDOUT")
		fmt.Print(indentLines(result.Stdout))
	}

	if result.Stderr != "" {
		bold.Println("STDERR")
		fmt.Print(indentLines(result.Stderr))
	}

	if verbose || (result.Code != nil && *result.Code != 0) {
		if result.Code != nil {
			if *result.Code == 0 {
				fmt.Print("Exit Code: ")
				green.Printf("%d\n", *result.Code)
			} else {
				fmt.Print("Exit Code: ")
				red.Printf("%d\n", *result.Code)
			}
		}
	}

	if result.Signal != "" {
		fmt.Print("Signal: ")
		yellow.Printf("%s\n", result.Signal)
	}

	if verbose {
		fmt.Printf("Memory: %d bytes\n", result.Memory)
		fmt.Printf("CPU Time: %d ms\n", result.CPUTime)
		fmt.Printf("Wall Time: %d ms\n", result.WallTime)
	}

	fmt.Println()
}

func indentLines(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n") + "\n"
}

// executeInteractive is implemented in websocket.go
func executeInteractive(url, language, version string, files []FileData, args []string,
	status, verbose bool) error {
	return executeInteractiveWS(url, language, version, files, args, status, verbose)
}

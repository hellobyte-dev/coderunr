package types

import (
	"time"

	"github.com/Masterminds/semver/v3"
)

// JobState represents the state of a job execution
type JobState int

const (
	JobStateReady JobState = iota
	JobStatePrimed
	JobStateExecuted
)

// CodeFile represents a source code file
type CodeFile struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// Timeouts represents timeout configurations
type Timeouts struct {
	Compile time.Duration `json:"compile"`
	Run     time.Duration `json:"run"`
}

// CPUTimes represents CPU time limits
type CPUTimes struct {
	Compile time.Duration `json:"compile"`
	Run     time.Duration `json:"run"`
}

// MemoryLimits represents memory limit configurations
type MemoryLimits struct {
	Compile int64 `json:"compile"`
	Run     int64 `json:"run"`
}

// Runtime represents a language runtime environment
type Runtime struct {
	Language        string          `json:"language"`
	Version         *semver.Version `json:"version"`
	Aliases         []string        `json:"aliases"`
	PkgDir          string          `json:"pkgdir"`
	Runtime         string          `json:"runtime"`
	Timeouts        Timeouts        `json:"timeouts"`
	CPUTimes        CPUTimes        `json:"cpu_times"`
	MemoryLimits    MemoryLimits    `json:"memory_limits"`
	MaxProcessCount int             `json:"max_process_count"`
	MaxOpenFiles    int             `json:"max_open_files"`
	MaxFileSize     int64           `json:"max_file_size"`
	OutputMaxSize   int             `json:"output_max_size"`
	Compiled        bool            `json:"compiled"`
	EnvVars         []string        `json:"env_vars"`
}

// StageResult represents the result of a compilation or execution stage
type StageResult struct {
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Output   string        `json:"output"`
	Code     int           `json:"code"`
	Signal   string        `json:"signal,omitempty"`
	Memory   int64         `json:"memory"`
	Message  string        `json:"message,omitempty"`
	Status   string        `json:"status,omitempty"`
	CPUTime  time.Duration `json:"cpu_time"`
	WallTime time.Duration `json:"wall_time"`
}

// ExecutionResult represents the complete result of job execution
type ExecutionResult struct {
	Compile  *StageResult `json:"compile,omitempty"`
	Run      *StageResult `json:"run"`
	Language string       `json:"language"`
	Version  string       `json:"version"`
}

// JobRequest represents an incoming job execution request
type JobRequest struct {
	Language           string     `json:"language" validate:"required"`
	Version            string     `json:"version" validate:"required"`
	Files              []CodeFile `json:"files" validate:"required,dive"`
	Args               []string   `json:"args,omitempty"`
	Stdin              string     `json:"stdin,omitempty"`
	CompileMemoryLimit *int64     `json:"compile_memory_limit,omitempty"`
	RunMemoryLimit     *int64     `json:"run_memory_limit,omitempty"`
	RunTimeout         *int       `json:"run_timeout,omitempty"`
	CompileTimeout     *int       `json:"compile_timeout,omitempty"`
	RunCPUTime         *int       `json:"run_cpu_time,omitempty"`
	CompileCPUTime     *int       `json:"compile_cpu_time,omitempty"`
}

// IsolateBox represents an isolate sandbox
type IsolateBox struct {
	ID           int    `json:"id"`
	MetadataPath string `json:"metadata_path"`
	Dir          string `json:"dir"`
}

// Package represents a language package
type Package struct {
	Language string          `json:"language"`
	Version  *semver.Version `json:"version"`
	Download string          `json:"download"`
	Checksum string          `json:"checksum"`
}

// PackageInfo represents package information for API responses
type PackageInfo struct {
	Language        string `json:"language"`
	LanguageVersion string `json:"language_version"`
	Installed       bool   `json:"installed"`
}

// RuntimeInfo represents runtime information for API responses
type RuntimeInfo struct {
	Language string   `json:"language"`
	Version  string   `json:"version"`
	Aliases  []string `json:"aliases"`
	Runtime  string   `json:"runtime,omitempty"`
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
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

// StreamEvent represents a streaming execution event
type StreamEvent struct {
	Type   string
	Stream string
	Data   string
	Stage  string
	Signal string
	Code   int
	Error  error
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

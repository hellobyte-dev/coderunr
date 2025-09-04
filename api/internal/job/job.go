package job

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coderunr/api/internal/config"
	"github.com/coderunr/api/internal/types"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	IsolatePath = "/usr/local/bin/isolate"
	MaxBoxID    = 999
)

var (
	boxIDCounter   int32
	remainingSlots int32
	jobQueue       = make(chan func(), 1000)
	queueMutex     sync.Mutex
	queueCondition = sync.NewCond(&queueMutex)
)

// Manager handles job execution
type Manager struct {
	config *config.Config
	logger *logrus.Entry
}

// NewManager creates a new job manager
func NewManager(cfg *config.Config) *Manager {
	atomic.StoreInt32(&remainingSlots, int32(cfg.MaxConcurrentJobs))

	manager := &Manager{
		config: cfg,
		logger: logrus.WithField("component", "job"),
	}

	// Start job queue processor
	go manager.processJobQueue()

	return manager
}

// Job represents a code execution job
type Job struct {
	ID           string
	Runtime      *types.Runtime
	Files        []types.CodeFile
	Args         []string
	Stdin        string
	Timeouts     types.Timeouts
	CPUTimes     types.CPUTimes
	MemoryLimits types.MemoryLimits
	State        types.JobState
	dirtyBoxes   []*types.IsolateBox
	logger       *logrus.Entry
	manager      *Manager
}

// NewJob creates a new job from a request
func (m *Manager) NewJob(runtime *types.Runtime, request *types.JobRequest) *Job {
	jobID := uuid.New().String()

	// Process files
	files := make([]types.CodeFile, len(request.Files))
	for i, file := range request.Files {
		encoding := file.Encoding
		if encoding == "" {
			encoding = "utf8"
		}

		// Validate encoding
		if encoding != "utf8" && encoding != "base64" && encoding != "hex" {
			encoding = "utf8"
		}

		name := file.Name
		if name == "" {
			name = fmt.Sprintf("file%d.code", i)
		}

		files[i] = types.CodeFile{
			Name:     name,
			Content:  file.Content,
			Encoding: encoding,
		}
	}

	// Process stdin
	stdin := request.Stdin
	if stdin != "" && !strings.HasSuffix(stdin, "\n") {
		stdin += "\n"
	}

	// Apply request-specific limits or use runtime defaults
	timeouts := types.Timeouts{
		Compile: runtime.Timeouts.Compile,
		Run:     runtime.Timeouts.Run,
	}

	cpuTimes := types.CPUTimes{
		Compile: runtime.CPUTimes.Compile,
		Run:     runtime.CPUTimes.Run,
	}

	memoryLimits := types.MemoryLimits{
		Compile: runtime.MemoryLimits.Compile,
		Run:     runtime.MemoryLimits.Run,
	}

	// Override with request values if provided
	if request.CompileTimeout != nil {
		timeouts.Compile = time.Duration(*request.CompileTimeout) * time.Millisecond
	}
	if request.RunTimeout != nil {
		timeouts.Run = time.Duration(*request.RunTimeout) * time.Millisecond
	}
	if request.CompileCPUTime != nil {
		cpuTimes.Compile = time.Duration(*request.CompileCPUTime) * time.Millisecond
	}
	if request.RunCPUTime != nil {
		cpuTimes.Run = time.Duration(*request.RunCPUTime) * time.Millisecond
	}
	if request.CompileMemoryLimit != nil {
		memoryLimits.Compile = *request.CompileMemoryLimit
	}
	if request.RunMemoryLimit != nil {
		memoryLimits.Run = *request.RunMemoryLimit
	}

	return &Job{
		ID:           jobID,
		Runtime:      runtime,
		Files:        files,
		Args:         request.Args,
		Stdin:        stdin,
		Timeouts:     timeouts,
		CPUTimes:     cpuTimes,
		MemoryLimits: memoryLimits,
		State:        types.JobStateReady,
		dirtyBoxes:   []*types.IsolateBox{},
		logger:       logrus.WithField("job_id", jobID),
		manager:      m,
	}
}

// Execute executes the job and returns the result
func (j *Job) Execute(ctx context.Context) (*types.ExecutionResult, error) {
	defer j.cleanup()

	// Wait for available slot
	if err := j.waitForSlot(); err != nil {
		return nil, fmt.Errorf("failed to acquire job slot: %w", err)
	}
	defer j.releaseSlot()

	j.logger.Info("Executing job")

	// Prime the job (create isolate box and prepare files)
	box, err := j.prime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prime job: %w", err)
	}

	result := &types.ExecutionResult{
		Language: j.Runtime.Language,
		Version:  j.Runtime.Version.String(),
	}

	// Compile stage (if needed)
	if j.Runtime.Compiled {
		j.logger.Debug("Running compile stage")
		compileResult, err := j.safeCall(ctx, box, "compile", j.getCodeFileNames(),
			j.Timeouts.Compile, j.CPUTimes.Compile, j.MemoryLimits.Compile)
		if err != nil {
			return nil, fmt.Errorf("compile stage failed: %w", err)
		}
		result.Compile = compileResult

		// If compilation failed, don't run
		if compileResult.Code != 0 {
			return result, nil
		}

		// Create new box for run stage
		if newBox, err := j.createIsolateBox(); err != nil {
			return nil, fmt.Errorf("failed to create run box: %w", err)
		} else {
			// Move compiled files to new box
			oldSubmissionDir := filepath.Join(box.Dir, "submission")
			newSubmissionDir := filepath.Join(newBox.Dir, "submission")
			if err := os.Rename(oldSubmissionDir, newSubmissionDir); err != nil {
				return nil, fmt.Errorf("failed to move compiled files: %w", err)
			}
			box = newBox
		}
	}

	// Run stage
	j.logger.Debug("Running execution stage")
	args := []string{j.Files[0].Name}
	args = append(args, j.Args...)

	runResult, err := j.safeCall(ctx, box, "run", args,
		j.Timeouts.Run, j.CPUTimes.Run, j.MemoryLimits.Run)
	if err != nil {
		return nil, fmt.Errorf("run stage failed: %w", err)
	}
	result.Run = runResult

	j.State = types.JobStateExecuted
	return result, nil
}

// prime prepares the job for execution
func (j *Job) prime(ctx context.Context) (*types.IsolateBox, error) {
	j.logger.Info("Priming job")

	// Create isolate box
	box, err := j.createIsolateBox()
	if err != nil {
		return nil, err
	}

	// Create submission directory and write files
	submissionDir := filepath.Join(box.Dir, "submission")
	if err := os.MkdirAll(submissionDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create submission directory: %w", err)
	}

	for _, file := range j.Files {
		if err := j.writeFile(submissionDir, file); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", file.Name, err)
		}
	}

	j.State = types.JobStatePrimed
	j.logger.Debug("Job primed successfully")
	return box, nil
}

// createIsolateBox creates a new isolate sandbox
func (j *Job) createIsolateBox() (*types.IsolateBox, error) {
	boxID := int(atomic.AddInt32(&boxIDCounter, 1) % MaxBoxID)
	metadataPath := fmt.Sprintf("/tmp/%d-metadata.txt", boxID)

	cmd := exec.Command(IsolatePath, "--init", "--cg", fmt.Sprintf("-b%d", boxID))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("isolate init failed: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return nil, fmt.Errorf("received empty output from isolate --init")
	}

	box := &types.IsolateBox{
		ID:           boxID,
		MetadataPath: metadataPath,
		Dir:          outputStr + "/box",
	}

	j.dirtyBoxes = append(j.dirtyBoxes, box)
	return box, nil
}

// writeFile writes a file to the submission directory
func (j *Job) writeFile(submissionDir string, file types.CodeFile) error {
	// Prevent path traversal
	if strings.Contains(file.Name, "..") {
		return fmt.Errorf("invalid file name: %s", file.Name)
	}

	filePath := filepath.Join(submissionDir, file.Name)
	relPath, err := filepath.Rel(submissionDir, filePath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("path traversal detected: %s", file.Name)
	}

	// Decode file content
	var content []byte
	switch file.Encoding {
	case "base64":
		content, err = base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return fmt.Errorf("failed to decode base64: %w", err)
		}
	case "hex":
		content, err = hex.DecodeString(file.Content)
		if err != nil {
			return fmt.Errorf("failed to decode hex: %w", err)
		}
	default: // utf8
		content = []byte(file.Content)
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// safeCall executes a stage (compile or run) safely within isolate
func (j *Job) safeCall(ctx context.Context, box *types.IsolateBox, stage string, args []string,
	timeout, cpuTime time.Duration, memoryLimit int64) (*types.StageResult, error) {

	// Build isolate command
	isolateArgs := []string{
		"--run",
		fmt.Sprintf("-b%d", box.ID),
		fmt.Sprintf("--meta=%s", box.MetadataPath),
		"--cg",
		"-s",
		"-c", "/box/submission",
		"-E", "HOME=/tmp",
	}

	// Add environment variables
	for _, envVar := range j.Runtime.EnvVars {
		isolateArgs = append(isolateArgs, "-E", envVar)
	}

	// Add coderunr language env var
	isolateArgs = append(isolateArgs, "-E", fmt.Sprintf("CODERUNR_LANGUAGE=%s", j.Runtime.Language))

	// Add directories
	isolateArgs = append(isolateArgs, fmt.Sprintf("--dir=%s", j.Runtime.PkgDir))
	isolateArgs = append(isolateArgs, "--dir=/etc:noexec")

	// Add resource limits
	isolateArgs = append(isolateArgs, fmt.Sprintf("--processes=%d", j.Runtime.MaxProcessCount))
	isolateArgs = append(isolateArgs, fmt.Sprintf("--open-files=%d", j.Runtime.MaxOpenFiles))
	isolateArgs = append(isolateArgs, fmt.Sprintf("--fsize=%d", j.Runtime.MaxFileSize/1000))
	isolateArgs = append(isolateArgs, fmt.Sprintf("--wall-time=%d", int(timeout.Seconds())))
	isolateArgs = append(isolateArgs, fmt.Sprintf("--time=%d", int(cpuTime.Seconds())))
	isolateArgs = append(isolateArgs, "--extra-time=0")

	// Add memory limit if specified
	if memoryLimit >= 0 {
		isolateArgs = append(isolateArgs, fmt.Sprintf("--cg-mem=%d", memoryLimit/1000))
	}

	// Add networking option
	if !j.manager.config.DisableNetworking {
		isolateArgs = append(isolateArgs, "--share-net")
	}

	// Add execution command
	isolateArgs = append(isolateArgs, "--", "/bin/bash", filepath.Join(j.Runtime.PkgDir, stage))
	isolateArgs = append(isolateArgs, args...)

	// Create command with context
	cmd := exec.CommandContext(ctx, IsolatePath, isolateArgs...)

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start isolate: %w", err)
	}

	// Write stdin and close
	go func() {
		defer stdin.Close()
		if j.Stdin != "" {
			stdin.Write([]byte(j.Stdin))
		}
	}()

	// Read output with size limits
	var stdoutBuf, stderrBuf bytes.Buffer
	var outputBuf bytes.Buffer

	go j.readWithLimit(stdout, &stdoutBuf, &outputBuf)
	go j.readWithLimit(stderr, &stderrBuf, &outputBuf)

	// Wait for command to finish
	err = cmd.Wait()

	// Parse metadata
	metadata, parseErr := j.parseMetadata(box.MetadataPath)
	if parseErr != nil {
		j.logger.WithError(parseErr).Warn("Failed to parse metadata")
	}

	result := &types.StageResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
		Output: outputBuf.String(),
		Code:   cmd.ProcessState.ExitCode(),
	}

	// Apply metadata if available
	if metadata != nil {
		result.Memory = metadata.Memory
		result.CPUTime = metadata.CPUTime
		result.WallTime = metadata.WallTime
		result.Status = metadata.Status
		result.Message = metadata.Message
		result.Signal = metadata.Signal
	}

	// Override signal for certain statuses
	if result.Status == "TO" || result.Status == "OL" || result.Status == "EL" {
		result.Signal = "SIGKILL"
	}

	return result, nil
}

// readWithLimit reads from a reader with size limit
func (j *Job) readWithLimit(reader io.Reader, targetBuf, outputBuf *bytes.Buffer) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text() + "\n"

		if targetBuf.Len()+len(line) <= j.Runtime.OutputMaxSize {
			targetBuf.WriteString(line)
			outputBuf.WriteString(line)
		} else {
			break // Stop reading if limit exceeded
		}
	}
}

// parseMetadata parses the isolate metadata file
func (j *Job) parseMetadata(metadataPath string) (*isolateMetadata, error) {
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	metadata := &isolateMetadata{}
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]
		switch key {
		case "cg-mem":
			if mem, err := strconv.ParseInt(value, 10, 64); err == nil {
				metadata.Memory = mem * 1000
			}
		case "exitcode":
			if code, err := strconv.Atoi(value); err == nil {
				metadata.ExitCode = code
			}
		case "exitsig":
			if sig, err := strconv.Atoi(value); err == nil {
				metadata.Signal = signalToString(sig)
			}
		case "message":
			metadata.Message = value
		case "status":
			metadata.Status = value
		case "time":
			if t, err := strconv.ParseFloat(value, 64); err == nil {
				metadata.CPUTime = time.Duration(t * float64(time.Second))
			}
		case "time-wall":
			if t, err := strconv.ParseFloat(value, 64); err == nil {
				metadata.WallTime = time.Duration(t * float64(time.Second))
			}
		}
	}

	return metadata, nil
}

// isolateMetadata represents metadata from isolate
type isolateMetadata struct {
	Memory   int64
	ExitCode int
	Signal   string
	Message  string
	Status   string
	CPUTime  time.Duration
	WallTime time.Duration
}

// getCodeFileNames returns the names of code files
func (j *Job) getCodeFileNames() []string {
	names := make([]string, len(j.Files))
	for i, file := range j.Files {
		names[i] = file.Name
	}
	return names
}

// waitForSlot waits for an available job slot
func (j *Job) waitForSlot() error {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	for atomic.LoadInt32(&remainingSlots) <= 0 {
		j.logger.Info("Waiting for available job slot")
		queueCondition.Wait()
	}

	atomic.AddInt32(&remainingSlots, -1)
	return nil
}

// releaseSlot releases a job slot
func (j *Job) releaseSlot() {
	atomic.AddInt32(&remainingSlots, 1)
	queueCondition.Signal()
}

// cleanup cleans up job resources
func (j *Job) cleanup() {
	j.logger.Info("Cleaning up job")

	for _, box := range j.dirtyBoxes {
		cmd := exec.Command(IsolatePath, "--cleanup", "--cg", fmt.Sprintf("-b%d", box.ID))
		if err := cmd.Run(); err != nil {
			j.logger.WithError(err).Errorf("Failed to cleanup isolate box %d", box.ID)
		}

		if err := os.Remove(box.MetadataPath); err != nil {
			j.logger.WithError(err).Errorf("Failed to remove metadata file %s", box.MetadataPath)
		}
	}
}

// processJobQueue processes the job queue (placeholder for future use)
func (m *Manager) processJobQueue() {
	// This can be extended later for more sophisticated job queuing
	for range jobQueue {
		// Process queued jobs
	}
}

// signalToString converts signal number to string
func signalToString(sig int) string {
	signals := map[int]string{
		1: "SIGHUP", 2: "SIGINT", 3: "SIGQUIT", 4: "SIGILL", 5: "SIGTRAP",
		6: "SIGABRT", 7: "SIGBUS", 8: "SIGFPE", 9: "SIGKILL", 10: "SIGUSR1",
		11: "SIGSEGV", 12: "SIGUSR2", 13: "SIGPIPE", 14: "SIGALRM", 15: "SIGTERM",
		// Add more as needed
	}

	if name, exists := signals[sig]; exists {
		return name
	}
	return fmt.Sprintf("SIG%d", sig)
}

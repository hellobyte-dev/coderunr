package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/coderunr/api/internal/job"
	"github.com/coderunr/api/internal/runtime"
	"github.com/coderunr/api/internal/types"
	"github.com/sirupsen/logrus"
)

// Handler contains the dependencies for HTTP handlers
type Handler struct {
	jobManager     *job.Manager
	runtimeManager *runtime.Manager
	logger         *logrus.Logger
}

// NewHandler creates a new handler instance
func NewHandler(jobManager *job.Manager, runtimeManager *runtime.Manager, logger *logrus.Logger) *Handler {
	return &Handler{
		jobManager:     jobManager,
		runtimeManager: runtimeManager,
		logger:         logger,
	}
}

// GetVersion returns the API version
func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"message": "CodeRunr v1.0.0-go",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ExecuteCode executes code synchronously
func (h *Handler) ExecuteCode(w http.ResponseWriter, r *http.Request) {
	var request types.JobRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&request); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			h.sendError(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		h.sendError(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validateJobRequest(&request); err != nil {
		h.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Find runtime
	runtime, err := runtime.GetLatestRuntimeMatchingLanguageVersion(request.Language, request.Version)
	if err != nil {
		h.sendError(w, fmt.Sprintf("%s-%s runtime is unknown", request.Language, request.Version), http.StatusBadRequest)
		return
	}

	// Validate runtime constraints
	if err := h.validateConstraints(&request, runtime); err != nil {
		h.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create and execute job
	job := h.jobManager.NewJob(runtime, &request)
	result, err := job.Execute(r.Context())
	if err != nil {
		h.logger.WithError(err).Error("Job execution failed")
		h.sendError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Handle backward compatibility (Piston behavior)
	if result.Run == nil && result.Compile != nil {
		result.Run = result.Compile
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// GetRuntimes returns available runtimes
func (h *Handler) GetRuntimes(w http.ResponseWriter, r *http.Request) {
	runtimes := runtime.GetRuntimes()

	response := make([]types.RuntimeInfo, len(runtimes))
	for i, rt := range runtimes {
		runtimeName := rt.Runtime
		if runtimeName == "" {
			runtimeName = rt.Language
		}

		response[i] = types.RuntimeInfo{
			Language: rt.Language,
			Version:  rt.Version.String(),
			Aliases:  rt.Aliases,
			Runtime:  runtimeName,
			Platform: rt.Platform,
			OS:       rt.OS,
			Arch:     rt.Arch,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// validateJobRequest validates the incoming job request
func (h *Handler) validateJobRequest(request *types.JobRequest) error {
	if request.Language == "" {
		return fmt.Errorf("language is required as a string")
	}

	if request.Version == "" {
		return fmt.Errorf("version is required as a string")
	}

	if len(request.Files) == 0 {
		return fmt.Errorf("files is required as an array")
	}

	for i, file := range request.Files {
		if file.Content == "" {
			return fmt.Errorf("files[%d].content is required as a string", i)
		}
	}

	return nil
}

// validateConstraints validates resource constraints against runtime limits
func (h *Handler) validateConstraints(request *types.JobRequest, rt *types.Runtime) error {
	// Check if files include at least one utf8 encoded file (except for 'file' language)
	if rt.Language != "file" {
		hasUTF8 := false
		for _, file := range request.Files {
			if file.Encoding == "" || file.Encoding == "utf8" {
				hasUTF8 = true
				break
			}
		}
		if !hasUTF8 {
			return fmt.Errorf("files must include at least one utf8 encoded file")
		}
	}

	// Validate constraints
	constraints := []struct {
		name        string
		value       *int
		configLimit int64
	}{
		{"compile_timeout", request.CompileTimeout, rt.Timeouts.Compile.Milliseconds()},
		{"run_timeout", request.RunTimeout, rt.Timeouts.Run.Milliseconds()},
		{"compile_cpu_time", request.CompileCPUTime, rt.CPUTimes.Compile.Milliseconds()},
		{"run_cpu_time", request.RunCPUTime, rt.CPUTimes.Run.Milliseconds()},
	}

	for _, constraint := range constraints {
		if constraint.value == nil {
			continue
		}

		if constraint.configLimit <= 0 {
			continue
		}

		if int64(*constraint.value) > constraint.configLimit {
			return fmt.Errorf("%s cannot exceed the configured limit of %d",
				constraint.name, constraint.configLimit)
		}

		if *constraint.value < 0 {
			return fmt.Errorf("%s must be non-negative", constraint.name)
		}
	}

	// Validate memory constraints
	memoryConstraints := []struct {
		name        string
		value       *int64
		configLimit int64
	}{
		{"compile_memory_limit", request.CompileMemoryLimit, rt.MemoryLimits.Compile},
		{"run_memory_limit", request.RunMemoryLimit, rt.MemoryLimits.Run},
	}

	for _, constraint := range memoryConstraints {
		if constraint.value == nil {
			continue
		}

		if constraint.configLimit <= 0 {
			continue
		}

		if *constraint.value > constraint.configLimit {
			return fmt.Errorf("%s cannot exceed the configured limit of %d",
				constraint.name, constraint.configLimit)
		}

		if *constraint.value < 0 {
			return fmt.Errorf("%s must be non-negative", constraint.name)
		}
	}

	return nil
}

// sendError sends an error response
func (h *Handler) sendError(w http.ResponseWriter, message string, statusCode int) {
	response := types.ErrorResponse{
		Message: message,
		Code:    statusCode,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// sendJSON sends a JSON response
func (h *Handler) sendJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.WithError(err).Error("Failed to encode JSON response")
	}
}

// parseIntParam parses an integer parameter from URL
func parseIntParam(r *http.Request, param string) (int, error) {
	value := r.URL.Query().Get(param)
	if value == "" {
		return 0, fmt.Errorf("missing parameter: %s", param)
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer parameter %s: %s", param, value)
	}

	return intValue, nil
}

// parseBoolParam parses a boolean parameter from URL
func parseBoolParam(r *http.Request, param string, defaultValue bool) bool {
	value := r.URL.Query().Get(param)
	if value == "" {
		return defaultValue
	}

	return strings.ToLower(value) == "true" || value == "1"
}

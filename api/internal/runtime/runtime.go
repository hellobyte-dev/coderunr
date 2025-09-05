package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/coderunr/api/internal/config"
	"github.com/coderunr/api/internal/types"
	"github.com/sirupsen/logrus"
)

var (
	runtimes []types.Runtime
	mutex    sync.RWMutex
	logger   = logrus.WithField("component", "runtime")
)

// Manager handles runtime operations
type Manager struct {
	config *config.Config
}

// NewManager creates a new runtime manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// LoadPackages loads all installed packages from the data directory
func (m *Manager) LoadPackages() error {
	packagesDir := filepath.Join(m.config.DataDirectory, "packages")

	if _, err := os.Stat(packagesDir); os.IsNotExist(err) {
		logger.Warn("Packages directory does not exist, creating it")
		if err := os.MkdirAll(packagesDir, 0755); err != nil {
			return fmt.Errorf("failed to create packages directory: %w", err)
		}
		return nil
	}

	languages, err := os.ReadDir(packagesDir)
	if err != nil {
		return fmt.Errorf("failed to read packages directory: %w", err)
	}

	for _, lang := range languages {
		if !lang.IsDir() {
			continue
		}

		langDir := filepath.Join(packagesDir, lang.Name())
		versions, err := os.ReadDir(langDir)
		if err != nil {
			logger.WithError(err).Warnf("Failed to read language directory: %s", langDir)
			continue
		}

		for _, version := range versions {
			if !version.IsDir() {
				continue
			}

			packageDir := filepath.Join(langDir, version.Name())
			if err := m.loadPackage(packageDir); err != nil {
				logger.WithError(err).Warnf("Failed to load package: %s", packageDir)
				continue
			}
		}
	}

	logger.Infof("Loaded %d runtimes", len(runtimes))
	return nil
}

// LoadPackage loads a single package from the given directory (exported version)
func (m *Manager) LoadPackage(packageDir string) error {
	return m.loadPackage(packageDir)
}

// loadPackage loads a single package from the given directory
func (m *Manager) loadPackage(packageDir string) error {
	// Check if package is installed
	installedFile := filepath.Join(packageDir, ".ppman-installed")
	if _, err := os.Stat(installedFile); os.IsNotExist(err) {
		return nil // Package not installed, skip
	}

	// Read package info
	infoFile := filepath.Join(packageDir, "pkg-info.json")
	infoData, err := os.ReadFile(infoFile)
	if err != nil {
		return fmt.Errorf("failed to read pkg-info.json: %w", err)
	}

	var info struct {
		Language      string   `json:"language"`
		Version       string   `json:"version"`
		BuildPlatform string   `json:"build_platform"`
		Aliases       []string `json:"aliases"`
		Provides      []struct {
			Language       string                 `json:"language"`
			Aliases        []string               `json:"aliases"`
			LimitOverrides map[string]interface{} `json:"limit_overrides"`
		} `json:"provides"`
		LimitOverrides map[string]interface{} `json:"limit_overrides"`
	}

	if err := json.Unmarshal(infoData, &info); err != nil {
		return fmt.Errorf("failed to parse pkg-info.json: %w", err)
	}

	version, err := semver.NewVersion(info.Version)
	if err != nil {
		return fmt.Errorf("failed to parse version %s: %w", info.Version, err)
	}

	// Check if package has compile script
	compiled := false
	compileScript := filepath.Join(packageDir, "compile")
	if _, err := os.Stat(compileScript); err == nil {
		compiled = true
	}

	// Load environment variables
	envVars, err := m.loadEnvVars(packageDir)
	if err != nil {
		logger.WithError(err).Warnf("Failed to load environment variables for %s", packageDir)
		envVars = []string{}
	}

	mutex.Lock()
	defer mutex.Unlock()

	// Handle provides field (multiple languages in one package)
	if len(info.Provides) > 0 {
		for _, provide := range info.Provides {
			runtime := types.Runtime{
				Language:        provide.Language,
				Version:         version,
				Aliases:         provide.Aliases,
				PkgDir:          packageDir,
				Runtime:         info.Language,
				Timeouts:        m.computeTimeouts(provide.Language, provide.LimitOverrides),
				CPUTimes:        m.computeCPUTimes(provide.Language, provide.LimitOverrides),
				MemoryLimits:    m.computeMemoryLimits(provide.Language, provide.LimitOverrides),
				MaxProcessCount: m.computeIntLimit(provide.Language, "max_process_count", provide.LimitOverrides),
				MaxOpenFiles:    m.computeIntLimit(provide.Language, "max_open_files", provide.LimitOverrides),
				MaxFileSize:     m.computeInt64Limit(provide.Language, "max_file_size", provide.LimitOverrides),
				OutputMaxSize:   m.computeIntLimit(provide.Language, "output_max_size", provide.LimitOverrides),
				Compiled:        compiled,
				EnvVars:         envVars,
			}
			runtimes = append(runtimes, runtime)
		}
	} else {
		runtime := types.Runtime{
			Language:        info.Language,
			Version:         version,
			Aliases:         info.Aliases,
			PkgDir:          packageDir,
			Runtime:         info.Language,
			Timeouts:        m.computeTimeouts(info.Language, info.LimitOverrides),
			CPUTimes:        m.computeCPUTimes(info.Language, info.LimitOverrides),
			MemoryLimits:    m.computeMemoryLimits(info.Language, info.LimitOverrides),
			MaxProcessCount: m.computeIntLimit(info.Language, "max_process_count", info.LimitOverrides),
			MaxOpenFiles:    m.computeIntLimit(info.Language, "max_open_files", info.LimitOverrides),
			MaxFileSize:     m.computeInt64Limit(info.Language, "max_file_size", info.LimitOverrides),
			OutputMaxSize:   m.computeIntLimit(info.Language, "output_max_size", info.LimitOverrides),
			Compiled:        compiled,
			EnvVars:         envVars,
		}
		runtimes = append(runtimes, runtime)
	}

	logger.Debugf("Loaded package %s-%s", info.Language, info.Version)
	return nil
}

// loadEnvVars loads environment variables from the .env file
func (m *Manager) loadEnvVars(packageDir string) ([]string, error) {
	envFile := filepath.Join(packageDir, ".env")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		return []string{}, nil
	}

	content, err := os.ReadFile(envFile)
	if err != nil {
		return nil, err
	}

	envContent := strings.TrimSpace(string(content))
	if envContent == "" {
		return []string{}, nil
	}

	return strings.Split(envContent, "\n"), nil
}

// GetRuntimes returns all loaded runtimes
func GetRuntimes() []types.Runtime {
	mutex.RLock()
	defer mutex.RUnlock()

	result := make([]types.Runtime, len(runtimes))
	copy(result, runtimes)
	return result
}

// GetLatestRuntimeMatchingLanguageVersion finds the latest runtime matching language and version
func GetLatestRuntimeMatchingLanguageVersion(language, version string) (*types.Runtime, error) {
	constraint, err := semver.NewConstraint(version)
	if err != nil {
		return nil, fmt.Errorf("invalid version constraint: %w", err)
	}

	mutex.RLock()
	defer mutex.RUnlock()

	var candidates []types.Runtime
	for _, rt := range runtimes {
		// Check if language matches (either language name or alias)
		if rt.Language == language || contains(rt.Aliases, language) {
			if constraint.Check(rt.Version) {
				candidates = append(candidates, rt)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no runtime found for %s-%s", language, version)
	}

	// Find the latest version
	latest := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Version.GreaterThan(latest.Version) {
			latest = candidate
		}
	}

	return &latest, nil
}

// GetRuntimeByNameAndVersion finds a runtime by exact name and version
func GetRuntimeByNameAndVersion(runtime, version string) (*types.Runtime, error) {
	constraint, err := semver.NewConstraint(version)
	if err != nil {
		return nil, fmt.Errorf("invalid version constraint: %w", err)
	}

	mutex.RLock()
	defer mutex.RUnlock()

	for _, rt := range runtimes {
		if (rt.Runtime == runtime || (rt.Runtime == "" && rt.Language == runtime)) &&
			constraint.Check(rt.Version) {
			return &rt, nil
		}
	}

	return nil, fmt.Errorf("runtime not found: %s-%s", runtime, version)
}

// computeTimeouts computes timeout limits for a language
func (m *Manager) computeTimeouts(language string, overrides map[string]interface{}) types.Timeouts {
	return types.Timeouts{
		Compile: m.computeDurationLimit(language, "compile_timeout", overrides, m.config.CompileTimeout),
		Run:     m.computeDurationLimit(language, "run_timeout", overrides, m.config.RunTimeout),
	}
}

// computeCPUTimes computes CPU time limits for a language
func (m *Manager) computeCPUTimes(language string, overrides map[string]interface{}) types.CPUTimes {
	return types.CPUTimes{
		Compile: m.computeDurationLimit(language, "compile_cpu_time", overrides, m.config.CompileCPUTime),
		Run:     m.computeDurationLimit(language, "run_cpu_time", overrides, m.config.RunCPUTime),
	}
}

// computeMemoryLimits computes memory limits for a language
func (m *Manager) computeMemoryLimits(language string, overrides map[string]interface{}) types.MemoryLimits {
	return types.MemoryLimits{
		Compile: m.computeInt64Limit(language, "compile_memory_limit", overrides),
		Run:     m.computeInt64Limit(language, "run_memory_limit", overrides),
	}
}

// computeDurationLimit computes a duration limit with overrides
func (m *Manager) computeDurationLimit(language, limitName string, overrides map[string]interface{}, defaultValue time.Duration) time.Duration {
	// Check global config overrides first
	if value, exists := m.config.GetLimitOverride(language, limitName); exists {
		if duration, ok := value.(time.Duration); ok {
			return duration
		}
		if ms, ok := value.(int); ok {
			return time.Duration(ms) * time.Millisecond
		}
	}

	// Check package-specific overrides
	if overrides != nil {
		if value, exists := overrides[limitName]; exists {
			if ms, ok := value.(float64); ok {
				return time.Duration(ms) * time.Millisecond
			}
			if ms, ok := value.(int); ok {
				return time.Duration(ms) * time.Millisecond
			}
		}
	}

	return defaultValue
}

// computeIntLimit computes an integer limit with overrides
func (m *Manager) computeIntLimit(language, limitName string, overrides map[string]interface{}) int {
	// Check global config overrides first
	if value, exists := m.config.GetLimitOverride(language, limitName); exists {
		if intValue, ok := value.(int); ok {
			return intValue
		}
	}

	// Check package-specific overrides
	if overrides != nil {
		if value, exists := overrides[limitName]; exists {
			if intValue, ok := value.(float64); ok {
				return int(intValue)
			}
			if intValue, ok := value.(int); ok {
				return intValue
			}
		}
	}

	// Return config defaults
	switch limitName {
	case "max_process_count":
		return m.config.MaxProcessCount
	case "max_open_files":
		return m.config.MaxOpenFiles
	case "output_max_size":
		return m.config.OutputMaxSize
	default:
		return 0
	}
}

// computeInt64Limit computes an int64 limit with overrides
func (m *Manager) computeInt64Limit(language, limitName string, overrides map[string]interface{}) int64 {
	// Check global config overrides first
	if value, exists := m.config.GetLimitOverride(language, limitName); exists {
		if intValue, ok := value.(int64); ok {
			return intValue
		}
		if intValue, ok := value.(int); ok {
			return int64(intValue)
		}
	}

	// Check package-specific overrides
	if overrides != nil {
		if value, exists := overrides[limitName]; exists {
			if intValue, ok := value.(float64); ok {
				return int64(intValue)
			}
			if intValue, ok := value.(int); ok {
				return int64(intValue)
			}
		}
	}

	// Return config defaults
	switch limitName {
	case "compile_memory_limit":
		return m.config.CompileMemoryLimit
	case "run_memory_limit":
		return m.config.RunMemoryLimit
	case "max_file_size":
		return m.config.MaxFileSize
	default:
		return -1
	}
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

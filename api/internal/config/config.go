package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	// Server configuration
	LogLevel      string `mapstructure:"log_level"`
	BindAddress   string `mapstructure:"bind_address"`
	DataDirectory string `mapstructure:"data_directory"`

	// Job execution limits
	MaxConcurrentJobs  int           `mapstructure:"max_concurrent_jobs"`
	CompileTimeout     time.Duration `mapstructure:"compile_timeout"`
	RunTimeout         time.Duration `mapstructure:"run_timeout"`
	CompileCPUTime     time.Duration `mapstructure:"compile_cpu_time"`
	RunCPUTime         time.Duration `mapstructure:"run_cpu_time"`
	CompileMemoryLimit int64         `mapstructure:"compile_memory_limit"`
	RunMemoryLimit     int64         `mapstructure:"run_memory_limit"`

	// Process limits
	MaxProcessCount int   `mapstructure:"max_process_count"`
	MaxOpenFiles    int   `mapstructure:"max_open_files"`
	MaxFileSize     int64 `mapstructure:"max_file_size"`
	OutputMaxSize   int   `mapstructure:"output_max_size"`

	// Security settings
	DisableNetworking bool `mapstructure:"disable_networking"`
	RunnerUIDMin      int  `mapstructure:"runner_uid_min"`
	RunnerUIDMax      int  `mapstructure:"runner_uid_max"`
	RunnerGIDMin      int  `mapstructure:"runner_gid_min"`
	RunnerGIDMax      int  `mapstructure:"runner_gid_max"`

	// Package management
	RepoURL string `mapstructure:"repo_url"`

	// Limit overrides (JSON map)
	LimitOverrides map[string]map[string]interface{} `mapstructure:"limit_overrides"`
}

// Load loads configuration from environment variables and config files
func Load() (*Config, error) {
	// Set default values
	viper.SetDefault("log_level", "INFO")
	viper.SetDefault("bind_address", getEnvOrDefault("PORT", "2000"))
	viper.SetDefault("data_directory", "/coderunr")
	viper.SetDefault("max_concurrent_jobs", 64)
	viper.SetDefault("compile_timeout", "10s")
	viper.SetDefault("run_timeout", "3s")
	viper.SetDefault("compile_cpu_time", "10s")
	viper.SetDefault("run_cpu_time", "3s")
	viper.SetDefault("compile_memory_limit", -1)
	viper.SetDefault("run_memory_limit", -1)
	viper.SetDefault("max_process_count", 64)
	viper.SetDefault("max_open_files", 2048)
	viper.SetDefault("max_file_size", 10000000) // 10MB
	viper.SetDefault("output_max_size", 1024)
	viper.SetDefault("disable_networking", true)
	viper.SetDefault("runner_uid_min", 1001)
	viper.SetDefault("runner_uid_max", 1500)
	viper.SetDefault("runner_gid_min", 1001)
	viper.SetDefault("runner_gid_max", 1500)
	viper.SetDefault("repo_url", "https://github.com/engineer-man/piston/releases/download/pkgs/index")
	viper.SetDefault("limit_overrides", map[string]map[string]interface{}{})

	// Set environment variable prefix
	viper.SetEnvPrefix("CODERUNR")
	viper.AutomaticEnv()

	// Try to read config file
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/coderunr/")
	viper.AddConfigPath("$HOME/.coderunr/")

	// Read config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// validate validates the configuration
func validate(config *Config) error {
	// Check if data directory exists
	if _, err := os.Stat(config.DataDirectory); os.IsNotExist(err) {
		return fmt.Errorf("data directory does not exist: %s", config.DataDirectory)
	}

	// Validate log level
	if _, err := logrus.ParseLevel(config.LogLevel); err != nil {
		return fmt.Errorf("invalid log level: %s", config.LogLevel)
	}

	// Validate numeric ranges
	if config.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("max_concurrent_jobs must be positive")
	}

	if config.RunnerUIDMin >= config.RunnerUIDMax {
		return fmt.Errorf("runner_uid_min must be less than runner_uid_max")
	}

	if config.RunnerGIDMin >= config.RunnerGIDMax {
		return fmt.Errorf("runner_gid_min must be less than runner_gid_max")
	}

	return nil
}

// getEnvOrDefault gets environment variable or returns default value
func getEnvOrDefault(env, defaultValue string) string {
	if value := os.Getenv(env); value != "" {
		return value
	}
	return "0.0.0.0:" + defaultValue
}

// GetBindAddress returns the complete bind address
func (c *Config) GetBindAddress() string {
	if c.BindAddress == "" {
		return "0.0.0.0:2000"
	}
	return c.BindAddress
}

// GetLogLevel returns the parsed log level
func (c *Config) GetLogLevel() logrus.Level {
	level, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		return logrus.InfoLevel
	}
	return level
}

// GetLimitOverride returns the limit override for a specific language and limit type
func (c *Config) GetLimitOverride(language, limitType string) (interface{}, bool) {
	if langOverrides, exists := c.LimitOverrides[language]; exists {
		if value, exists := langOverrides[limitType]; exists {
			return value, true
		}
	}
	return nil, false
}

// GetIntEnv gets an integer environment variable with fallback
func GetIntEnv(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return fallback
}

// GetBoolEnv gets a boolean environment variable with fallback
func GetBoolEnv(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return fallback
}

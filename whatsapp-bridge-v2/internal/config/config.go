// Package config provides configuration management using Viper.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// defaultDataDir returns the default directory for storing WhatsApp data.
// Uses ~/.whatsapp-mcp/ so data is in a fixed location regardless of CWD.
func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./store"
	}
	return filepath.Join(home, ".whatsapp-mcp")
}

// Config holds all configuration for the WhatsApp bridge.
type Config struct {
	// Paths
	SessionPath string        `mapstructure:"session_path"`
	StorePath   string        `mapstructure:"store_path"`

	// Connection
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`

	// Health & Reconnection
	KeepaliveInterval   time.Duration `mapstructure:"keepalive_interval"`
	ReconnectMaxRetries int           `mapstructure:"reconnect_max_retries"`
	ReconnectBaseDelay  time.Duration `mapstructure:"reconnect_base_delay"`
	ReconnectMaxDelay   time.Duration `mapstructure:"reconnect_max_delay"`

	// Logging
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`

	// Metrics
	MetricsEnabled bool `mapstructure:"metrics_enabled"`
	MetricsPort    int  `mapstructure:"metrics_port"`

	// MCP
	MCPEnabled bool `mapstructure:"mcp_enabled"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	dataDir := defaultDataDir()
	return &Config{
		SessionPath:         filepath.Join(dataDir, "whatsapp.db"),
		StorePath:           filepath.Join(dataDir, "messages.db"),
		ConnectTimeout:      30 * time.Second,
		KeepaliveInterval:   30 * time.Second,
		ReconnectMaxRetries: 10,
		ReconnectBaseDelay:  1 * time.Second,
		ReconnectMaxDelay:   5 * time.Minute,
		LogLevel:            "info",
		LogFormat:           "json",
		MetricsEnabled:      true,
		MetricsPort:         9090,
		MCPEnabled:          true,
	}
}

// LoadConfig loads configuration from file, environment, and defaults.
// Priority: CLI flags > Environment > Config file > Defaults
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	defaults := DefaultConfig()
	v.SetDefault("session_path", defaults.SessionPath)
	v.SetDefault("store_path", defaults.StorePath)
	v.SetDefault("connect_timeout", defaults.ConnectTimeout)
	v.SetDefault("keepalive_interval", defaults.KeepaliveInterval)
	v.SetDefault("reconnect_max_retries", defaults.ReconnectMaxRetries)
	v.SetDefault("reconnect_base_delay", defaults.ReconnectBaseDelay)
	v.SetDefault("reconnect_max_delay", defaults.ReconnectMaxDelay)
	v.SetDefault("log_level", defaults.LogLevel)
	v.SetDefault("log_format", defaults.LogFormat)
	v.SetDefault("metrics_enabled", defaults.MetricsEnabled)
	v.SetDefault("metrics_port", defaults.MetricsPort)
	v.SetDefault("mcp_enabled", defaults.MCPEnabled)

	// Environment variables with WABRIDGE_ prefix
	v.SetEnvPrefix("WABRIDGE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Load from config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			// Ignore if the default config.yaml simply doesn't exist — use built-in defaults.
			// Only fail if the user explicitly provided a path that can't be read.
			isNotFound := errors.Is(err, os.ErrNotExist)
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !isNotFound {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	// Validate metrics port
	if c.MetricsPort < 0 || c.MetricsPort > 65535 {
		return fmt.Errorf("invalid metrics port: %d (must be 0-65535)", c.MetricsPort)
	}

	// Validate keepalive interval
	if c.KeepaliveInterval <= 0 {
		return fmt.Errorf("keepalive interval must be positive")
	}

	// Validate reconnect settings
	if c.ReconnectMaxRetries < 0 {
		return fmt.Errorf("reconnect max retries must be non-negative")
	}

	if c.ReconnectBaseDelay <= 0 {
		return fmt.Errorf("reconnect base delay must be positive")
	}

	if c.ReconnectMaxDelay <= 0 {
		return fmt.Errorf("reconnect max delay must be positive")
	}

	if c.ReconnectBaseDelay > c.ReconnectMaxDelay {
		return fmt.Errorf("reconnect base delay must be less than or equal to max delay")
	}

	return nil
}

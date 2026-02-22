package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".whatsapp-mcp", "whatsapp.db"), cfg.SessionPath)
	assert.Equal(t, filepath.Join(home, ".whatsapp-mcp", "messages.db"), cfg.StorePath)
	assert.Equal(t, 30*time.Second, cfg.ConnectTimeout)
	assert.Equal(t, 30*time.Second, cfg.KeepaliveInterval)
	assert.Equal(t, 10, cfg.ReconnectMaxRetries)
	assert.Equal(t, 1*time.Second, cfg.ReconnectBaseDelay)
	assert.Equal(t, 5*time.Minute, cfg.ReconnectMaxDelay)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.True(t, cfg.MetricsEnabled)
	assert.Equal(t, 9090, cfg.MetricsPort)
	assert.True(t, cfg.MCPEnabled)
}

func TestLoadConfig_FromFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
session_path: /custom/session.db
store_path: /custom/store.db
connect_timeout: 60s
keepalive_interval: 45s
reconnect_max_retries: 5
reconnect_base_delay: 2s
reconnect_max_delay: 10m
log_level: debug
log_format: text
metrics_enabled: false
metrics_port: 8080
mcp_enabled: false
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "/custom/session.db", cfg.SessionPath)
	assert.Equal(t, "/custom/store.db", cfg.StorePath)
	assert.Equal(t, 60*time.Second, cfg.ConnectTimeout)
	assert.Equal(t, 45*time.Second, cfg.KeepaliveInterval)
	assert.Equal(t, 5, cfg.ReconnectMaxRetries)
	assert.Equal(t, 2*time.Second, cfg.ReconnectBaseDelay)
	assert.Equal(t, 10*time.Minute, cfg.ReconnectMaxDelay)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "text", cfg.LogFormat)
	assert.False(t, cfg.MetricsEnabled)
	assert.Equal(t, 8080, cfg.MetricsPort)
	assert.False(t, cfg.MCPEnabled)
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	// Create temp config file with defaults
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
log_level: info
metrics_port: 9090
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set env vars to override
	os.Setenv("WABRIDGE_LOG_LEVEL", "debug")
	os.Setenv("WABRIDGE_METRICS_PORT", "8888")
	defer os.Unsetenv("WABRIDGE_LOG_LEVEL")
	defer os.Unsetenv("WABRIDGE_METRICS_PORT")

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Env vars should override file values
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 8888, cfg.MetricsPort)
}

func TestLoadConfig_NoFile(t *testing.T) {
	// Should use defaults when no file exists
	cfg, err := LoadConfig("")
	require.NoError(t, err)

	// Should have default values
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".whatsapp-mcp", "whatsapp.db"), cfg.SessionPath)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "invalid log level",
			modify: func(c *Config) {
				c.LogLevel = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid metrics port",
			modify: func(c *Config) {
				c.MetricsPort = -1
			},
			wantErr: true,
		},
		{
			name: "zero keepalive interval",
			modify: func(c *Config) {
				c.KeepaliveInterval = 0
			},
			wantErr: true,
		},
		{
			name: "negative reconnect retries",
			modify: func(c *Config) {
				c.ReconnectMaxRetries = -1
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)

			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

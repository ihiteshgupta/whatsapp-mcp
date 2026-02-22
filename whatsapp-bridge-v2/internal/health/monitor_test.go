package health

import (
	"context"
	"testing"
	"time"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/config"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMonitor(t *testing.T) {
	cfg := config.DefaultConfig()
	sm := state.NewMachine()

	m := NewMonitor(cfg, sm)
	require.NotNil(t, m)
	assert.Equal(t, cfg.KeepaliveInterval, m.keepaliveInterval)
}

func TestMonitor_GetStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	sm := state.NewMachine()

	m := NewMonitor(cfg, sm)
	m.Start()
	defer m.Stop()

	status := m.GetStatus()

	assert.Equal(t, string(state.StateDisconnected), status.State)
	assert.False(t, status.Connected)
	assert.GreaterOrEqual(t, status.UptimeSeconds, int64(0))
}

func TestMonitor_RecordMessage(t *testing.T) {
	cfg := config.DefaultConfig()
	sm := state.NewMachine()

	m := NewMonitor(cfg, sm)

	m.RecordMessageReceived()
	m.RecordMessageReceived()
	m.RecordMessageSent()

	status := m.GetStatus()
	assert.Equal(t, int64(2), status.MessagesReceived)
	assert.Equal(t, int64(1), status.MessagesSent)
}

func TestMonitor_ReconnectBackoff(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ReconnectBaseDelay = 100 * time.Millisecond
	cfg.ReconnectMaxDelay = 1 * time.Second
	cfg.ReconnectMaxRetries = 5

	sm := state.NewMachine()
	m := NewMonitor(cfg, sm)

	// First delay should be positive (backoff has randomization)
	delay := m.GetNextReconnectDelay()
	assert.Greater(t, delay, time.Duration(0))
	assert.LessOrEqual(t, delay, cfg.ReconnectMaxDelay)

	// Get more delays
	_ = m.GetNextReconnectDelay()
	_ = m.GetNextReconnectDelay()

	// Reset backoff
	m.ResetReconnectBackoff()
	delayAfterReset := m.GetNextReconnectDelay()
	assert.Greater(t, delayAfterReset, time.Duration(0))
	assert.LessOrEqual(t, delayAfterReset, cfg.ReconnectMaxDelay)
}

func TestMonitor_MaxRetriesExceeded(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ReconnectBaseDelay = 1 * time.Millisecond
	cfg.ReconnectMaxDelay = 10 * time.Millisecond
	cfg.ReconnectMaxRetries = 3

	sm := state.NewMachine()
	m := NewMonitor(cfg, sm)

	// Exhaust retries
	for i := 0; i < cfg.ReconnectMaxRetries+1; i++ {
		_ = m.GetNextReconnectDelay()
	}

	assert.True(t, m.IsMaxRetriesExceeded())
}

func TestMonitor_LastMessageTime(t *testing.T) {
	cfg := config.DefaultConfig()
	sm := state.NewMachine()
	m := NewMonitor(cfg, sm)

	before := time.Now()
	m.RecordMessageReceived()
	after := time.Now()

	lastMsg := m.GetLastMessageTime()
	assert.True(t, lastMsg.After(before) || lastMsg.Equal(before))
	assert.True(t, lastMsg.Before(after) || lastMsg.Equal(after))
}

func TestMonitor_StateUpdates(t *testing.T) {
	cfg := config.DefaultConfig()
	sm := state.NewMachine()
	m := NewMonitor(cfg, sm)
	m.Start()
	defer m.Stop()

	ctx := context.Background()

	// Initial state
	status := m.GetStatus()
	assert.Equal(t, string(state.StateDisconnected), status.State)

	// Transition to connecting
	_ = sm.Fire(ctx, state.TriggerConnect)

	// Give the monitor time to pick up the change
	time.Sleep(10 * time.Millisecond)

	status = m.GetStatus()
	assert.Equal(t, string(state.StateConnecting), status.State)
}

func TestMonitor_IncrementReconnectCount(t *testing.T) {
	cfg := config.DefaultConfig()
	sm := state.NewMachine()
	m := NewMonitor(cfg, sm)

	assert.Equal(t, 0, m.GetReconnectCount())

	m.IncrementReconnectCount()
	m.IncrementReconnectCount()

	assert.Equal(t, 2, m.GetReconnectCount())
}

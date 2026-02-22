// Package health provides health monitoring and self-healing for the bridge.
package health

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/config"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
)

// Status represents the health status of the bridge.
type Status struct {
	State            string    `json:"state"`
	Connected        bool      `json:"connected"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	LastMessage      time.Time `json:"last_message"`
	ReconnectCount   int       `json:"reconnect_count"`
	MessagesReceived int64     `json:"messages_received"`
	MessagesSent     int64     `json:"messages_sent"`
}

// Monitor tracks bridge health and manages reconnection.
type Monitor struct {
	config       *config.Config
	stateMachine *state.Machine
	log          *slog.Logger

	keepaliveInterval time.Duration
	reconnectBackoff  *backoff.ExponentialBackOff
	maxRetries        int
	retryCount        int

	startTime        time.Time
	lastMessage      time.Time
	reconnectCount   int
	messagesReceived atomic.Int64
	messagesSent     atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// NewMonitor creates a new health monitor.
func NewMonitor(cfg *config.Config, sm *state.Machine) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = cfg.ReconnectBaseDelay
	bo.MaxInterval = cfg.ReconnectMaxDelay
	bo.MaxElapsedTime = 0 // Never stop based on elapsed time
	bo.Reset()

	return &Monitor{
		config:            cfg,
		stateMachine:      sm,
		log:               slog.Default(),
		keepaliveInterval: cfg.KeepaliveInterval,
		reconnectBackoff:  bo,
		maxRetries:        cfg.ReconnectMaxRetries,
		startTime:         time.Now(),
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start begins the health monitoring.
func (m *Monitor) Start() {
	m.startTime = time.Now()
	m.log.Info("health monitor started", "keepalive_interval", m.keepaliveInterval)
}

// Stop stops the health monitoring.
func (m *Monitor) Stop() {
	m.cancel()
	m.wg.Wait()
	m.log.Info("health monitor stopped")
}

// GetStatus returns the current health status.
func (m *Monitor) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	currentState, _ := m.stateMachine.State(context.Background())
	connected := currentState == state.StateReady

	return Status{
		State:            string(currentState),
		Connected:        connected,
		UptimeSeconds:    int64(time.Since(m.startTime).Seconds()),
		LastMessage:      m.lastMessage,
		ReconnectCount:   m.reconnectCount,
		MessagesReceived: m.messagesReceived.Load(),
		MessagesSent:     m.messagesSent.Load(),
	}
}

// RecordMessageReceived records an incoming message.
func (m *Monitor) RecordMessageReceived() {
	m.messagesReceived.Add(1)
	m.mu.Lock()
	m.lastMessage = time.Now()
	m.mu.Unlock()
}

// RecordMessageSent records an outgoing message.
func (m *Monitor) RecordMessageSent() {
	m.messagesSent.Add(1)
}

// GetLastMessageTime returns the time of the last message.
func (m *Monitor) GetLastMessageTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastMessage
}

// GetNextReconnectDelay returns the next reconnect delay using exponential backoff.
func (m *Monitor) GetNextReconnectDelay() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.retryCount++
	return m.reconnectBackoff.NextBackOff()
}

// ResetReconnectBackoff resets the backoff to initial values.
func (m *Monitor) ResetReconnectBackoff() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reconnectBackoff.Reset()
	m.retryCount = 0
}

// IsMaxRetriesExceeded returns true if max reconnection retries have been exceeded.
func (m *Monitor) IsMaxRetriesExceeded() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.retryCount > m.maxRetries
}

// IncrementReconnectCount increments the total reconnection count.
func (m *Monitor) IncrementReconnectCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconnectCount++
}

// GetReconnectCount returns the total number of reconnections.
func (m *Monitor) GetReconnectCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reconnectCount
}

// ScheduleReconnect schedules a reconnection attempt with backoff.
func (m *Monitor) ScheduleReconnect(callback func()) {
	if m.IsMaxRetriesExceeded() {
		m.log.Error("max reconnection retries exceeded")
		return
	}

	delay := m.GetNextReconnectDelay()
	m.log.Info("scheduling reconnect", "delay", delay, "attempt", m.retryCount)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		select {
		case <-time.After(delay):
			m.IncrementReconnectCount()
			callback()
		case <-m.ctx.Done():
			return
		}
	}()
}

// OnConnectionRestored should be called when connection is restored.
func (m *Monitor) OnConnectionRestored() {
	m.ResetReconnectBackoff()
	m.log.Info("connection restored, backoff reset")
}

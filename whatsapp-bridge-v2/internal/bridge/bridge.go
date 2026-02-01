package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/config"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/state"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/store"
)

// Bridge is the core WhatsApp bridge that manages connection, state, and events.
type Bridge struct {
	client       WhatsAppClient
	stateMachine *state.Machine
	store        *store.SQLiteStore
	config       *config.Config
	log          *slog.Logger

	events         chan Event
	eventListeners []func(Event)
	stateListeners []func(from, to state.State)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// NewBridge creates a new WhatsApp bridge.
func NewBridge(cfg *config.Config, storeDB *store.SQLiteStore, client WhatsAppClient) *Bridge {
	ctx, cancel := context.WithCancel(context.Background())

	b := &Bridge{
		client:       client,
		stateMachine: state.NewMachine(),
		store:        storeDB,
		config:       cfg,
		log:          slog.Default(),
		events:       make(chan Event, 100),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Register state transition callback
	b.stateMachine.OnTransition(func(ctx context.Context, from, to state.State, trigger state.Trigger) {
		b.log.Info("state transition", "from", from, "to", to, "trigger", trigger)

		// Persist state
		if err := b.store.State.SaveState(ctx, to); err != nil {
			b.log.Error("failed to save state", "error", err)
		}

		// Log transition
		if err := b.store.State.LogTransition(ctx, from, to, string(trigger)); err != nil {
			b.log.Error("failed to log transition", "error", err)
		}

		// Notify listeners
		b.mu.RLock()
		listeners := make([]func(from, to state.State), len(b.stateListeners))
		copy(listeners, b.stateListeners)
		b.mu.RUnlock()

		for _, listener := range listeners {
			listener(from, to)
		}
	})

	// Start event processor
	b.wg.Add(1)
	go b.processEvents()

	return b
}

// Connect initiates connection to WhatsApp.
func (b *Bridge) Connect(ctx context.Context) error {
	if err := b.stateMachine.Fire(ctx, state.TriggerConnect); err != nil {
		return fmt.Errorf("failed to transition to connecting: %w", err)
	}

	// Connect the client
	if err := b.client.Connect(); err != nil {
		_ = b.stateMachine.Fire(ctx, state.TriggerFatalError)
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Check if we need QR or already logged in
	if b.client.IsLoggedIn() {
		_ = b.stateMachine.Fire(ctx, state.TriggerAuthenticated)
	} else {
		_ = b.stateMachine.Fire(ctx, state.TriggerQRRequired)
	}

	return nil
}

// Disconnect disconnects from WhatsApp.
func (b *Bridge) Disconnect() {
	b.client.Disconnect()
	_ = b.stateMachine.Fire(context.Background(), state.TriggerDisconnect)
}

// Stop gracefully stops the bridge.
func (b *Bridge) Stop() {
	_ = b.stateMachine.Fire(context.Background(), state.TriggerShutdown)
	b.cancel()
	close(b.events)
	b.wg.Wait()
}

// CurrentState returns the current state of the bridge.
func (b *Bridge) CurrentState() state.State {
	s, _ := b.stateMachine.State(context.Background())
	return s
}

// IsReady returns true if the bridge is ready to send/receive messages.
func (b *Bridge) IsReady() bool {
	return b.CurrentState() == state.StateReady
}

// SendMessage sends a text message.
func (b *Bridge) SendMessage(ctx context.Context, jid string, text string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}

	msgID, err := b.client.SendMessage(ctx, jid, text)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	return msgID, nil
}

// SendMedia sends a media message.
func (b *Bridge) SendMedia(ctx context.Context, jid string, data []byte, mimeType string, filename string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}

	msgID, err := b.client.SendMedia(ctx, jid, data, mimeType, filename)
	if err != nil {
		return "", fmt.Errorf("failed to send media: %w", err)
	}

	return msgID, nil
}

// EmitEvent adds an event to the processing queue.
func (b *Bridge) EmitEvent(evt Event) {
	select {
	case b.events <- evt:
	default:
		b.log.Warn("event queue full, dropping event", "type", evt.Type)
	}
}

// OnEvent registers a callback for all events.
func (b *Bridge) OnEvent(handler func(Event)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.eventListeners = append(b.eventListeners, handler)
}

// OnStateChange registers a callback for state changes.
func (b *Bridge) OnStateChange(handler func(from, to state.State)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stateListeners = append(b.stateListeners, handler)
}

// processEvents is the event processing goroutine.
func (b *Bridge) processEvents() {
	defer b.wg.Done()

	for evt := range b.events {
		b.handleEvent(evt)
	}
}

func (b *Bridge) handleEvent(evt Event) {
	b.log.Debug("processing event", "type", evt.Type)

	// Notify listeners
	b.mu.RLock()
	listeners := make([]func(Event), len(b.eventListeners))
	copy(listeners, b.eventListeners)
	b.mu.RUnlock()

	for _, listener := range listeners {
		listener(evt)
	}

	// Handle specific event types
	switch evt.Type {
	case EventMessage:
		b.handleMessage(evt)
	case EventQRCode:
		// QR code display is handled by listeners
	}
}

func (b *Bridge) handleMessage(evt Event) {
	payload, ok := evt.Payload.(MessagePayload)
	if !ok {
		b.log.Error("invalid message payload")
		return
	}

	// Store the message
	msg := &store.Message{
		ID:        payload.ID,
		ChatJID:   payload.ChatJID,
		Sender:    payload.Sender,
		Content:   payload.Content,
		IsFromMe:  payload.IsFromMe,
		MediaType: payload.MediaType,
		Timestamp: payload.Timestamp,
	}

	if err := b.store.Messages.Store(context.Background(), msg); err != nil {
		b.log.Error("failed to store message", "error", err)
	}

	// Update chat last message time
	if err := b.store.Chats.UpdateLastMessage(context.Background(), payload.ChatJID, payload.Timestamp); err != nil {
		b.log.Error("failed to update chat", "error", err)
	}
}

// GetStateMachine returns the state machine for direct manipulation (testing).
func (b *Bridge) GetStateMachine() *state.Machine {
	return b.stateMachine
}

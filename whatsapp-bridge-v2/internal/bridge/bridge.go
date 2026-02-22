package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/config"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/store"
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

	// Register event handler to persist incoming messages and history syncs to the store.
	// This must be done before connecting so no events are missed.
	b.registerWhatsAppEventHandler()

	// Connect the client
	if err := b.client.Connect(ctx); err != nil {
		// Don't fire fatal error on clean context cancellation (normal shutdown path)
		if ctx.Err() == nil {
			if smErr := b.stateMachine.Fire(context.Background(), state.TriggerFatalError); smErr != nil {
				b.log.Error("state transition failed", "trigger", state.TriggerFatalError, "error", smErr)
			}
		}
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Check if we need QR or already logged in
	if b.client.IsLoggedIn() {
		if err := b.stateMachine.Fire(ctx, state.TriggerAuthenticated); err != nil {
			b.log.Error("state transition failed", "trigger", state.TriggerAuthenticated, "error", err)
		}
		if err := b.stateMachine.Fire(ctx, state.TriggerSyncComplete); err != nil {
			b.log.Error("state transition failed", "trigger", state.TriggerSyncComplete, "error", err)
		}
	} else {
		if err := b.stateMachine.Fire(ctx, state.TriggerQRRequired); err != nil {
			b.log.Error("state transition failed", "trigger", state.TriggerQRRequired, "error", err)
		}
	}

	return nil
}

// Disconnect disconnects from WhatsApp.
func (b *Bridge) Disconnect() {
	b.client.Disconnect()
	if err := b.stateMachine.Fire(context.Background(), state.TriggerDisconnect); err != nil {
		b.log.Error("state transition failed", "trigger", state.TriggerDisconnect, "error", err)
	}
}

// Stop gracefully stops the bridge.
func (b *Bridge) Stop() {
	if err := b.stateMachine.Fire(context.Background(), state.TriggerShutdown); err != nil {
		b.log.Error("state transition failed", "trigger", state.TriggerShutdown, "error", err)
	}
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

// SendMedia is not used directly; use SendImage, SendVideo, SendAudio, or SendDocument instead.
func (b *Bridge) SendMedia(ctx context.Context, jid string, data []byte, mimeType string, filename string) (string, error) {
	return "", fmt.Errorf("use SendImage, SendVideo, SendAudio, or SendDocument instead")
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

// --- Delegate methods to WhatsApp client ---

func (b *Bridge) ReplyToMessage(ctx context.Context, chatJID, messageID, text string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.ReplyToMessage(ctx, chatJID, messageID, text)
}

func (b *Bridge) ForwardMessage(ctx context.Context, sourceChatJID, messageID, targetJID string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.ForwardMessage(ctx, sourceChatJID, messageID, targetJID)
}

func (b *Bridge) EditMessage(ctx context.Context, chatJID, messageID, newContent string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.EditMessage(ctx, chatJID, messageID, newContent)
}

func (b *Bridge) DeleteMessage(ctx context.Context, chatJID, messageID string, forEveryone bool) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.DeleteMessage(ctx, chatJID, messageID, forEveryone)
}

func (b *Bridge) ReactToMessage(ctx context.Context, chatJID, messageID, emoji string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.ReactToMessage(ctx, chatJID, messageID, emoji)
}

func (b *Bridge) SendImage(ctx context.Context, jid, imagePath, caption string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SendImage(ctx, jid, imagePath, caption)
}

func (b *Bridge) SendVideo(ctx context.Context, jid, videoPath, caption string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SendVideo(ctx, jid, videoPath, caption)
}

func (b *Bridge) SendAudio(ctx context.Context, jid, audioPath string, asVoice bool) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SendAudio(ctx, jid, audioPath, asVoice)
}

func (b *Bridge) SendDocument(ctx context.Context, jid, filePath, filename string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SendDocument(ctx, jid, filePath, filename)
}

func (b *Bridge) SendLocation(ctx context.Context, jid string, lat, lon float64, name, address string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SendLocation(ctx, jid, lat, lon, name, address)
}

func (b *Bridge) SendContactCard(ctx context.Context, jid, contactJID string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SendContactCard(ctx, jid, contactJID)
}

func (b *Bridge) DownloadMedia(ctx context.Context, chatJID, messageID, savePath string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.DownloadMedia(ctx, chatJID, messageID, savePath)
}

func (b *Bridge) ArchiveChat(ctx context.Context, jid string, archive bool) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.ArchiveChat(ctx, jid, archive)
}

func (b *Bridge) PinChat(ctx context.Context, jid string, pin bool) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.PinChat(ctx, jid, pin)
}

func (b *Bridge) MuteChat(ctx context.Context, jid string, mute bool, duration string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.MuteChat(ctx, jid, mute, duration)
}

func (b *Bridge) MarkChatRead(ctx context.Context, jid string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.MarkChatRead(ctx, jid)
}

func (b *Bridge) DeleteChat(ctx context.Context, jid string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.DeleteChat(ctx, jid)
}

func (b *Bridge) BlockContact(ctx context.Context, jid string, block bool) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.BlockContact(ctx, jid, block)
}

func (b *Bridge) CheckPhoneRegistered(ctx context.Context, phone string) (bool, error) {
	if !b.IsReady() {
		return false, fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.CheckPhoneRegistered(ctx, phone)
}

func (b *Bridge) CreateGroup(ctx context.Context, name string, participants []string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.CreateGroup(ctx, name, participants)
}

func (b *Bridge) GetGroupInfo(ctx context.Context, jid string) (interface{}, error) {
	if !b.IsReady() {
		return nil, fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.GetGroupInfo(ctx, jid)
}

func (b *Bridge) LeaveGroup(ctx context.Context, jid string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.LeaveGroup(ctx, jid)
}

func (b *Bridge) AddGroupMembers(ctx context.Context, groupJID string, participants []string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.AddGroupMembers(ctx, groupJID, participants)
}

func (b *Bridge) RemoveGroupMembers(ctx context.Context, groupJID string, participants []string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.RemoveGroupMembers(ctx, groupJID, participants)
}

func (b *Bridge) PromoteAdmin(ctx context.Context, groupJID string, participants []string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.PromoteAdmin(ctx, groupJID, participants)
}

func (b *Bridge) DemoteAdmin(ctx context.Context, groupJID string, participants []string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.DemoteAdmin(ctx, groupJID, participants)
}

func (b *Bridge) SetGroupName(ctx context.Context, groupJID, name string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SetGroupName(ctx, groupJID, name)
}

func (b *Bridge) SetGroupTopic(ctx context.Context, groupJID, topic string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SetGroupTopic(ctx, groupJID, topic)
}

func (b *Bridge) SetGroupPhoto(ctx context.Context, groupJID, imagePath string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SetGroupPhoto(ctx, groupJID, imagePath)
}

func (b *Bridge) GetInviteLink(ctx context.Context, groupJID string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.GetInviteLink(ctx, groupJID)
}

func (b *Bridge) RevokeInviteLink(ctx context.Context, groupJID string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.RevokeInviteLink(ctx, groupJID)
}

func (b *Bridge) JoinViaInvite(ctx context.Context, inviteLink string) (string, error) {
	if !b.IsReady() {
		return "", fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.JoinViaInvite(ctx, inviteLink)
}

func (b *Bridge) SubscribePresence(ctx context.Context, jid string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SubscribePresence(ctx, jid)
}

func (b *Bridge) SendTyping(ctx context.Context, jid string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SendTyping(ctx, jid)
}

func (b *Bridge) SendRecording(ctx context.Context, jid string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SendRecording(ctx, jid)
}

func (b *Bridge) SetOnline(ctx context.Context) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SetOnline(ctx)
}

func (b *Bridge) SetOffline(ctx context.Context) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.SetOffline(ctx)
}

func (b *Bridge) PostTextStatus(ctx context.Context, text, backgroundColor string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.PostTextStatus(ctx, text, backgroundColor)
}

func (b *Bridge) PostImageStatus(ctx context.Context, imagePath, caption string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.PostImageStatus(ctx, imagePath, caption)
}

func (b *Bridge) DeleteStatus(ctx context.Context, statusID string) error {
	if !b.IsReady() {
		return fmt.Errorf("bridge not ready, current state: %s", b.CurrentState())
	}
	return b.client.DeleteStatus(ctx, statusID)
}

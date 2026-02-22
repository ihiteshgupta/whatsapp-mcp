// Package whatsapp provides the WhatsApp bridge client using whatsmeow.
package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
)

// Common errors
var (
	ErrNotConnected     = errors.New("not connected to WhatsApp")
	ErrNotLoggedIn      = errors.New("not logged in")
	ErrQRTimeout        = errors.New("QR code timeout")
	ErrInvalidRecipient = errors.New("invalid recipient")
	ErrInvalidGroup     = errors.New("invalid group JID")
	ErrNoParticipants   = errors.New("no participants provided")
)

// Client wraps the whatsmeow client with additional functionality.
type Client struct {
	client    *whatsmeow.Client
	container *sqlstore.Container
	log       *slog.Logger
	stateMgr  *state.Machine

	mu          sync.RWMutex
	qrChan      chan string
	eventChan   chan interface{}
	handlers    []func(interface{})
	isConnected bool
}

// Config holds configuration for the WhatsApp client.
type Config struct {
	StorePath string
	LogLevel  string
	StateMgr  *state.Machine
}

// NewClient creates a new WhatsApp client.
func NewClient(ctx context.Context, cfg *Config, log *slog.Logger) (*Client, error) {
	// Ensure store directory exists
	storeDir := filepath.Dir(cfg.StorePath)
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	// Create database logger adapter
	dbLog := &slogAdapter{log: log.With("component", "whatsmeow-db")}

	// Open database
	container, err := sqlstore.New(ctx, "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", cfg.StorePath), dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &Client{
		container: container,
		log:       log,
		stateMgr:  cfg.StateMgr,
		qrChan:    make(chan string, 10),
		eventChan: make(chan interface{}, 100),
	}, nil
}

// Connect establishes a connection to WhatsApp.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()

	if c.client != nil && c.client.IsConnected() {
		c.mu.Unlock()
		return nil // Already connected
	}

	// Get or create device store
	deviceStore, err := c.container.GetFirstDevice(ctx)
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to get device store: %w", err)
	}

	// Create client logger adapter
	clientLog := &slogAdapter{log: c.log.With("component", "whatsmeow")}

	// Create whatsmeow client
	c.client = whatsmeow.NewClient(deviceStore, clientLog)

	// Register event handler
	c.client.AddEventHandler(c.handleEvent)

	// Transition to connecting state
	if c.stateMgr != nil {
		_ = c.stateMgr.Fire(ctx, state.TriggerConnect)
	}

	needsQR := c.client.Store.ID == nil

	// Release the lock before any blocking operations to avoid deadlock:
	// pairWithQR loops calling IsReady() which needs RLock, and handleEvent
	// needs Lock for PairSuccess — both would block if we held the write lock.
	c.mu.Unlock()

	if needsQR {
		c.log.Info("No session found, QR code required")
		return c.pairWithQR(ctx)
	}

	// Connect with existing session
	if err := c.client.Connect(); err != nil {
		if c.stateMgr != nil {
			_ = c.stateMgr.Fire(ctx, state.TriggerFatalError)
		}
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.mu.Lock()
	c.isConnected = true
	c.mu.Unlock()
	return nil
}

// pairWithQR initiates QR code pairing.
func (c *Client) pairWithQR(ctx context.Context) error {
	// Transition to QR pending state
	if c.stateMgr != nil {
		_ = c.stateMgr.Fire(ctx, state.TriggerQRRequired)
	}

	// Connect - this will trigger QR events via the event handler
	if err := c.client.Connect(); err != nil {
		return fmt.Errorf("failed to connect for QR: %w", err)
	}

	// Wait for connection or context cancellation
	// QR codes will be sent to c.qrChan via handleEvent
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if c.IsReady() {
				c.log.Info("Successfully connected after QR pairing")
				if c.stateMgr != nil {
					_ = c.stateMgr.Fire(ctx, state.TriggerQRScanned)
					_ = c.stateMgr.Fire(ctx, state.TriggerAuthenticated)
				}
				return nil
			}
		}
	}
}

// Disconnect closes the WhatsApp connection.
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		c.client.Disconnect()
		c.isConnected = false
	}
}

// IsConnected returns true if connected to WhatsApp.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client != nil && c.client.IsConnected()
}

// IsLoggedIn returns true if we have an authenticated session.
func (c *Client) IsLoggedIn() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client != nil && c.client.Store.ID != nil
}

// IsReady returns true if the client is connected and logged in.
func (c *Client) IsReady() bool {
	return c.IsConnected() && c.IsLoggedIn()
}

// CurrentState returns the current state machine state.
func (c *Client) CurrentState() state.State {
	if c.stateMgr != nil {
		return c.stateMgr.MustState()
	}
	return state.StateDisconnected
}

// GetQRChannel returns a channel for receiving QR codes.
func (c *Client) GetQRChannel() <-chan string {
	return c.qrChan
}

// AddEventHandler adds an event handler for WhatsApp events.
func (c *Client) AddEventHandler(handler func(interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers = append(c.handlers, handler)
}

// handleEvent processes events from whatsmeow.
func (c *Client) handleEvent(evt interface{}) {
	// Log the event type
	c.log.Debug("WhatsApp event", "type", fmt.Sprintf("%T", evt))

	// Handle QR events specially - send to qrChan
	// Only send the first code (currently active). WhatsApp sends multiple rotation
	// codes in one event; sending all of them causes the file to be overwritten with
	// a future code that isn't valid yet. Whatsmeow fires a new QR event on rotation.
	if qr, ok := evt.(*events.QR); ok {
		c.log.Info("QR code received via event handler")
		if len(qr.Codes) > 0 {
			select {
			case c.qrChan <- qr.Codes[0]:
			default:
				c.log.Warn("QR channel full, dropping code")
			}
		}
	}

	// Handle successful pairing
	if _, ok := evt.(*events.PairSuccess); ok {
		c.log.Info("Pairing successful!")
		c.mu.Lock()
		c.isConnected = true
		c.mu.Unlock()
	}

	// Handle connection events
	if connected, ok := evt.(*events.Connected); ok {
		c.log.Info("Connected to WhatsApp", "info", connected)
		c.mu.Lock()
		c.isConnected = true
		c.mu.Unlock()
	}

	// Send to event channel
	select {
	case c.eventChan <- evt:
	default:
		c.log.Warn("Event channel full, dropping event")
	}

	// Call registered handlers
	c.mu.RLock()
	handlers := make([]func(interface{}), len(c.handlers))
	copy(handlers, c.handlers)
	c.mu.RUnlock()

	for _, handler := range handlers {
		handler(evt)
	}
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	c.Disconnect()
	if c.container != nil {
		return c.container.Close()
	}
	return nil
}

// GetRawClient returns the underlying whatsmeow client.
// Use with caution - prefer using the wrapper methods.
func (c *Client) GetRawClient() *whatsmeow.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

// --- Messaging Operations ---

// SendMessage sends a text message to a JID.
func (c *Client) SendMessage(ctx context.Context, jid string, text string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	recipient, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %w", err)
	}

	resp, err := c.client.SendMessage(ctx, recipient, &waE2E.Message{
		Conversation: &text,
	})
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	return resp.ID, nil
}

// ReplyToMessage sends a reply to a specific message.
func (c *Client) ReplyToMessage(ctx context.Context, chatJID, messageID, text string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	recipient, err := types.ParseJID(chatJID)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %w", err)
	}

	// Create message with context info for reply
	resp, err := c.client.SendMessage(ctx, recipient, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: &text,
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:      &messageID,
				Participant:   ptrString(chatJID),
				QuotedMessage: &waE2E.Message{Conversation: ptrString("")},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to reply: %w", err)
	}

	return resp.ID, nil
}

// ForwardMessage forwards a message to another chat.
// Note: WhatsApp forward is essentially resending the message with forward metadata.
// Since we need the original message content, this requires integration with the message store.
func (c *Client) ForwardMessage(ctx context.Context, sourceChatJID, messageID, targetJID string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	target, err := types.ParseJID(targetJID)
	if err != nil {
		return "", fmt.Errorf("invalid target JID: %w", err)
	}

	// For now, we can only forward by sending a new message indicating it's forwarded
	// Full implementation would require fetching the original message from store
	// and rebuilding it with ContextInfo.IsForwarded = true

	// Return not implemented for now since we need message store integration
	_ = target
	return "", errors.New("forward_message is not yet implemented; use send_message to send text instead")
}

// EditMessage edits a previously sent message.
func (c *Client) EditMessage(ctx context.Context, chatJID, messageID, newContent string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	recipient, err := types.ParseJID(chatJID)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	_, err = c.client.SendMessage(ctx, recipient, c.client.BuildEdit(recipient, messageID, &waE2E.Message{
		Conversation: &newContent,
	}))
	return err
}

// DeleteMessage deletes a message.
func (c *Client) DeleteMessage(ctx context.Context, chatJID, messageID string, forEveryone bool) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	recipient, err := types.ParseJID(chatJID)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	if forEveryone {
		_, err = c.client.SendMessage(ctx, recipient, c.client.BuildRevoke(recipient, types.EmptyJID, messageID))
	} else {
		return errors.New("delete for self only is not supported by whatsmeow; use forEveryone=true to retract the message")
	}

	return err
}

// ReactToMessage adds an emoji reaction to a message.
func (c *Client) ReactToMessage(ctx context.Context, chatJID, messageID, emoji string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	recipient, err := types.ParseJID(chatJID)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	_, err = c.client.SendMessage(ctx, recipient, c.client.BuildReaction(recipient, types.EmptyJID, messageID, emoji))
	return err
}

// --- Chat Operations ---

// ArchiveChat archives or unarchives a chat.
func (c *Client) ArchiveChat(ctx context.Context, jid string, archive bool) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	target, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	// Use the built-in helper
	return c.client.SendAppState(ctx, appstate.BuildArchive(target, archive, time.Time{}, nil))
}

// PinChat pins or unpins a chat.
func (c *Client) PinChat(ctx context.Context, jid string, pin bool) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	target, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	return c.client.SendAppState(ctx, appstate.BuildPin(target, pin))
}

// MuteChat mutes or unmutes a chat.
func (c *Client) MuteChat(ctx context.Context, jid string, mute bool, duration string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	target, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	var muteDuration time.Duration
	if mute {
		switch duration {
		case "8h":
			muteDuration = 8 * time.Hour
		case "1w", "1week":
			muteDuration = 7 * 24 * time.Hour
		case "forever", "":
			muteDuration = 0 // Forever
		default:
			// Try parsing as duration
			d, err := time.ParseDuration(duration)
			if err != nil {
				muteDuration = 0 // Default to forever
			} else {
				muteDuration = d
			}
		}
	}

	return c.client.SendAppState(ctx, appstate.BuildMute(target, mute, muteDuration))
}

// MarkChatRead marks a chat as read.
func (c *Client) MarkChatRead(ctx context.Context, jid string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	chatJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	return c.client.MarkRead(ctx, []types.MessageID{}, time.Now(), chatJID, types.EmptyJID)
}

// DeleteChat deletes a chat.
func (c *Client) DeleteChat(ctx context.Context, jid string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	target, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	return c.client.SendAppState(ctx, appstate.BuildDeleteChat(target, time.Time{}, nil, false))
}

// --- Contact Operations ---

// BlockContact blocks or unblocks a contact.
func (c *Client) BlockContact(ctx context.Context, jid string, block bool) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	contactJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	if block {
		_, err = c.client.UpdateBlocklist(ctx, contactJID, events.BlocklistChangeActionBlock)
	} else {
		_, err = c.client.UpdateBlocklist(ctx, contactJID, events.BlocklistChangeActionUnblock)
	}

	return err
}

// CheckPhoneRegistered checks if a phone number is registered on WhatsApp.
func (c *Client) CheckPhoneRegistered(ctx context.Context, phone string) (bool, error) {
	if !c.IsReady() {
		return false, ErrNotConnected
	}

	resp, err := c.client.IsOnWhatsApp(ctx, []string{phone})
	if err != nil {
		return false, fmt.Errorf("failed to check phone: %w", err)
	}

	if len(resp) > 0 {
		return resp[0].IsIn, nil
	}

	return false, nil
}

// --- Presence Operations ---

// SubscribePresence subscribes to presence updates for a contact.
func (c *Client) SubscribePresence(ctx context.Context, jid string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	contactJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	return c.client.SubscribePresence(ctx, contactJID)
}

// SendTyping sends a typing indicator.
func (c *Client) SendTyping(ctx context.Context, jid string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	chatJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	return c.client.SendChatPresence(ctx, chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
}

// SendRecording sends a recording indicator.
func (c *Client) SendRecording(ctx context.Context, jid string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	chatJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	return c.client.SendChatPresence(ctx, chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaAudio)
}

// SetOnline sets presence to online.
func (c *Client) SetOnline(ctx context.Context) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	return c.client.SendPresence(ctx, types.PresenceAvailable)
}

// SetOffline sets presence to offline.
func (c *Client) SetOffline(ctx context.Context) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	return c.client.SendPresence(ctx, types.PresenceUnavailable)
}

// --- Group Operations ---

// CreateGroup creates a new WhatsApp group.
func (c *Client) CreateGroup(ctx context.Context, name string, participants []string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	jids := make([]types.JID, len(participants))
	for i, p := range participants {
		jid, err := types.ParseJID(p)
		if err != nil {
			return "", fmt.Errorf("invalid participant JID %s: %w", p, err)
		}
		jids[i] = jid
	}

	groupInfo, err := c.client.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: jids,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create group: %w", err)
	}

	return groupInfo.JID.String(), nil
}

// GetGroupInfo returns information about a group.
func (c *Client) GetGroupInfo(ctx context.Context, jid string) (interface{}, error) {
	if !c.IsReady() {
		return nil, ErrNotConnected
	}

	groupJID, err := types.ParseJID(jid)
	if err != nil {
		return nil, fmt.Errorf("invalid JID: %w", err)
	}

	info, err := c.client.GetGroupInfo(ctx, groupJID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group info: %w", err)
	}

	return info, nil
}

// LeaveGroup leaves a group.
func (c *Client) LeaveGroup(ctx context.Context, jid string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	groupJID, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	return c.client.LeaveGroup(ctx, groupJID)
}

// AddGroupMembers adds members to a group.
func (c *Client) AddGroupMembers(ctx context.Context, groupJID string, participants []string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	jids := make([]types.JID, len(participants))
	for i, p := range participants {
		pjid, err := types.ParseJID(p)
		if err != nil {
			return fmt.Errorf("invalid participant JID %s: %w", p, err)
		}
		jids[i] = pjid
	}

	_, err = c.client.UpdateGroupParticipants(ctx, jid, jids, whatsmeow.ParticipantChangeAdd)
	return err
}

// RemoveGroupMembers removes members from a group.
func (c *Client) RemoveGroupMembers(ctx context.Context, groupJID string, participants []string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	jids := make([]types.JID, len(participants))
	for i, p := range participants {
		pjid, err := types.ParseJID(p)
		if err != nil {
			return fmt.Errorf("invalid participant JID %s: %w", p, err)
		}
		jids[i] = pjid
	}

	_, err = c.client.UpdateGroupParticipants(ctx, jid, jids, whatsmeow.ParticipantChangeRemove)
	return err
}

// PromoteAdmin promotes members to admin.
func (c *Client) PromoteAdmin(ctx context.Context, groupJID string, participants []string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	jids := make([]types.JID, len(participants))
	for i, p := range participants {
		pjid, err := types.ParseJID(p)
		if err != nil {
			return fmt.Errorf("invalid participant JID %s: %w", p, err)
		}
		jids[i] = pjid
	}

	_, err = c.client.UpdateGroupParticipants(ctx, jid, jids, whatsmeow.ParticipantChangePromote)
	return err
}

// DemoteAdmin demotes admins to regular members.
func (c *Client) DemoteAdmin(ctx context.Context, groupJID string, participants []string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	jids := make([]types.JID, len(participants))
	for i, p := range participants {
		pjid, err := types.ParseJID(p)
		if err != nil {
			return fmt.Errorf("invalid participant JID %s: %w", p, err)
		}
		jids[i] = pjid
	}

	_, err = c.client.UpdateGroupParticipants(ctx, jid, jids, whatsmeow.ParticipantChangeDemote)
	return err
}

// SetGroupName changes the group name.
func (c *Client) SetGroupName(ctx context.Context, groupJID, name string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	return c.client.SetGroupName(ctx, jid, name)
}

// SetGroupTopic changes the group description/topic.
func (c *Client) SetGroupTopic(ctx context.Context, groupJID, topic string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	return c.client.SetGroupTopic(ctx, jid, "", "", topic)
}

// SetGroupPhoto changes the group photo.
func (c *Client) SetGroupPhoto(ctx context.Context, groupJID, imagePath string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return fmt.Errorf("invalid group JID: %w", err)
	}

	if err := validateFilePath(imagePath); err != nil {
		return err
	}

	data, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}

	_, err = c.client.SetGroupPhoto(ctx, jid, data)
	return err
}

// GetInviteLink gets the group invite link.
func (c *Client) GetInviteLink(ctx context.Context, groupJID string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return "", fmt.Errorf("invalid group JID: %w", err)
	}

	link, err := c.client.GetGroupInviteLink(ctx, jid, false)
	if err != nil {
		return "", fmt.Errorf("failed to get invite link: %w", err)
	}

	return link, nil
}

// RevokeInviteLink revokes the current invite link and returns a new one.
func (c *Client) RevokeInviteLink(ctx context.Context, groupJID string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return "", fmt.Errorf("invalid group JID: %w", err)
	}

	link, err := c.client.GetGroupInviteLink(ctx, jid, true) // true = reset
	if err != nil {
		return "", fmt.Errorf("failed to revoke invite link: %w", err)
	}

	return link, nil
}

// JoinViaInvite joins a group via invite link.
func (c *Client) JoinViaInvite(ctx context.Context, inviteLink string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	groupJID, err := c.client.JoinGroupWithLink(ctx, inviteLink)
	if err != nil {
		return "", fmt.Errorf("failed to join group: %w", err)
	}

	return groupJID.String(), nil
}

// --- Status Operations ---

// PostTextStatus posts a text status.
// Note: WhatsApp status posting uses a special broadcast JID.
func (c *Client) PostTextStatus(ctx context.Context, text, backgroundColor string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	// Status broadcast JID
	statusJID := types.StatusBroadcastJID

	// Default background color if not provided
	if backgroundColor == "" {
		backgroundColor = "#075E54" // WhatsApp green
	}

	// Parse background color to int32 (ARGB format)
	bgColor := parseColorToARGB(backgroundColor)

	// Build and send text status message
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:           proto.String(text),
			BackgroundArgb: proto.Uint32(bgColor),
			TextArgb:       proto.Uint32(0xFFFFFFFF), // White text
		},
	}

	_, err := c.client.SendMessage(ctx, statusJID, msg)
	if err != nil {
		return fmt.Errorf("failed to post text status: %w", err)
	}

	return nil
}

// PostImageStatus posts an image status.
func (c *Client) PostImageStatus(ctx context.Context, imagePath, caption string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	// Status broadcast JID
	statusJID := types.StatusBroadcastJID

	if err := validateFilePath(imagePath); err != nil {
		return err
	}

	// Read image file
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image file: %w", err)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = "image/jpeg"
	}

	// Upload to WhatsApp servers
	uploaded, err := c.client.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// Build and send image status message
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		},
	}

	_, err = c.client.SendMessage(ctx, statusJID, msg)
	if err != nil {
		return fmt.Errorf("failed to post image status: %w", err)
	}

	return nil
}

// DeleteStatus deletes a status update.
// Note: Status deletion uses the same revoke mechanism as regular messages.
func (c *Client) DeleteStatus(ctx context.Context, statusID string) error {
	if !c.IsReady() {
		return ErrNotConnected
	}

	statusJID := types.StatusBroadcastJID

	// Build revoke message for status
	_, err := c.client.SendMessage(ctx, statusJID, c.client.BuildRevoke(statusJID, types.EmptyJID, statusID))
	if err != nil {
		return fmt.Errorf("failed to delete status: %w", err)
	}

	return nil
}

// parseColorToARGB parses a hex color string to ARGB uint32.
func parseColorToARGB(hex string) uint32 {
	// Remove # prefix if present
	hex = strings.TrimPrefix(hex, "#")

	// Default to WhatsApp green if parsing fails
	defaultColor := uint32(0xFF075E54)

	if len(hex) == 6 {
		var r, g, b uint8
		_, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
		if err != nil {
			return defaultColor
		}
		return uint32(0xFF)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
	}

	return defaultColor
}

// --- Media Operations ---

// SendImage sends an image to a chat.
func (c *Client) SendImage(ctx context.Context, jid, imagePath, caption string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	recipient, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %w", err)
	}

	if err := validateFilePath(imagePath); err != nil {
		return "", err
	}

	// Read image file
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		mimeType = "image/jpeg" // Default fallback
	}

	// Upload to WhatsApp servers
	uploaded, err := c.client.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	// Build and send image message
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		},
	}

	resp, err := c.client.SendMessage(ctx, recipient, msg)
	if err != nil {
		return "", fmt.Errorf("failed to send image: %w", err)
	}

	return resp.ID, nil
}

// SendVideo sends a video to a chat.
func (c *Client) SendVideo(ctx context.Context, jid, videoPath, caption string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	recipient, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %w", err)
	}

	if err := validateFilePath(videoPath); err != nil {
		return "", err
	}

	// Read video file
	data, err := os.ReadFile(videoPath)
	if err != nil {
		return "", fmt.Errorf("failed to read video file: %w", err)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "video/") {
		mimeType = "video/mp4" // Default fallback
	}

	// Upload to WhatsApp servers
	uploaded, err := c.client.Upload(ctx, data, whatsmeow.MediaVideo)
	if err != nil {
		return "", fmt.Errorf("failed to upload video: %w", err)
	}

	// Build and send video message
	msg := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		},
	}

	resp, err := c.client.SendMessage(ctx, recipient, msg)
	if err != nil {
		return "", fmt.Errorf("failed to send video: %w", err)
	}

	return resp.ID, nil
}

// SendAudio sends an audio file.
func (c *Client) SendAudio(ctx context.Context, jid, audioPath string, asVoice bool) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	recipient, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %w", err)
	}

	if err := validateFilePath(audioPath); err != nil {
		return "", err
	}

	// Read audio file
	data, err := os.ReadFile(audioPath)
	if err != nil {
		return "", fmt.Errorf("failed to read audio file: %w", err)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "audio/") {
		if asVoice {
			mimeType = "audio/ogg; codecs=opus" // Voice message format
		} else {
			mimeType = "audio/mpeg" // Default for audio files
		}
	}

	// Upload to WhatsApp servers
	uploaded, err := c.client.Upload(ctx, data, whatsmeow.MediaAudio)
	if err != nil {
		return "", fmt.Errorf("failed to upload audio: %w", err)
	}

	// Build and send audio message
	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			Mimetype:      proto.String(mimeType),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
			PTT:           proto.Bool(asVoice), // Push-to-talk (voice message)
		},
	}

	resp, err := c.client.SendMessage(ctx, recipient, msg)
	if err != nil {
		return "", fmt.Errorf("failed to send audio: %w", err)
	}

	return resp.ID, nil
}

// SendDocument sends a document.
func (c *Client) SendDocument(ctx context.Context, jid, filePath, filename string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	recipient, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %w", err)
	}

	if err := validateFilePath(filePath); err != nil {
		return "", err
	}

	// Read document file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read document file: %w", err)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(data)

	// Use provided filename or extract from path
	if filename == "" {
		filename = filepath.Base(filePath)
	}

	// Upload to WhatsApp servers
	uploaded, err := c.client.Upload(ctx, data, whatsmeow.MediaDocument)
	if err != nil {
		return "", fmt.Errorf("failed to upload document: %w", err)
	}

	// Build and send document message
	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			FileName:      proto.String(filename),
			Mimetype:      proto.String(mimeType),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		},
	}

	resp, err := c.client.SendMessage(ctx, recipient, msg)
	if err != nil {
		return "", fmt.Errorf("failed to send document: %w", err)
	}

	return resp.ID, nil
}

// SendLocation sends a location.
func (c *Client) SendLocation(ctx context.Context, jid string, lat, lon float64, name, address string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	recipient, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %w", err)
	}

	resp, err := c.client.SendMessage(ctx, recipient, &waE2E.Message{
		LocationMessage: &waE2E.LocationMessage{
			DegreesLatitude:  &lat,
			DegreesLongitude: &lon,
			Name:             &name,
			Address:          &address,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to send location: %w", err)
	}

	return resp.ID, nil
}

// SendContactCard sends a contact card.
func (c *Client) SendContactCard(ctx context.Context, jid, contactJID string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	recipient, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("invalid recipient JID: %w", err)
	}

	contactInfo, err := types.ParseJID(contactJID)
	if err != nil {
		return "", fmt.Errorf("invalid contact JID: %w", err)
	}

	// Use the contact JID to build display name
	displayName := contactInfo.User
	phoneNumber := "+" + contactInfo.User

	// Try to get contact name from store if available
	if c.client.Store != nil {
		if contact, err := c.client.Store.Contacts.GetContact(ctx, contactInfo); err == nil && contact.FullName != "" {
			displayName = contact.FullName
		} else if contact.PushName != "" {
			displayName = contact.PushName
		}
	}

	// Build vCard
	vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nFN:%s\nTEL;type=CELL;type=VOICE;waid=%s:%s\nEND:VCARD",
		displayName, contactInfo.User, phoneNumber)

	// Build and send contact message
	msg := &waE2E.Message{
		ContactMessage: &waE2E.ContactMessage{
			DisplayName: proto.String(displayName),
			Vcard:       proto.String(vcard),
		},
	}

	resp, err := c.client.SendMessage(ctx, recipient, msg)
	if err != nil {
		return "", fmt.Errorf("failed to send contact card: %w", err)
	}

	return resp.ID, nil
}

// DownloadMedia downloads media from a message.
// Note: This requires having the message with media info stored locally.
func (c *Client) DownloadMedia(ctx context.Context, chatJID, messageID, savePath string) (string, error) {
	if !c.IsReady() {
		return "", ErrNotConnected
	}

	// For now, return not implemented since we need the message info
	// from our store to download the media, which requires integration
	// with the message repository
	return "", errors.New("download_media is not yet implemented")
}

func validateFilePath(path string) error {
	cleanPath := filepath.Clean(path)

	if cleanPath == "." || cleanPath == "" {
		return errors.New("path is required")
	}

	forbiddenPrefixes := []string{
		"/etc",
		"/proc",
		"/sys",
		"/bin",
		"/sbin",
		"/usr",
		"/boot",
		"/dev",
		"/lib",
		"/lib64",
		"/root",
		"~/.ssh",
	}
	for _, prefix := range forbiddenPrefixes {
		if cleanPath == prefix || strings.HasPrefix(cleanPath, prefix+string(filepath.Separator)) {
			return fmt.Errorf("path %q is not allowed", cleanPath)
		}
	}

	if strings.Contains(cleanPath, "..") {
		return errors.New("path traversal is not allowed")
	}

	if _, err := os.Stat(cleanPath); err != nil {
		return err
	}

	return nil
}

// --- Helper functions ---

func ptrString(s string) *string {
	return &s
}

// slogAdapter adapts slog.Logger to whatsmeow's log interface.
type slogAdapter struct {
	log *slog.Logger
}

func (s *slogAdapter) Debugf(msg string, args ...interface{}) {
	s.log.Debug(fmt.Sprintf(msg, args...))
}

func (s *slogAdapter) Infof(msg string, args ...interface{}) {
	s.log.Info(fmt.Sprintf(msg, args...))
}

func (s *slogAdapter) Warnf(msg string, args ...interface{}) {
	s.log.Warn(fmt.Sprintf(msg, args...))
}

func (s *slogAdapter) Errorf(msg string, args ...interface{}) {
	s.log.Error(fmt.Sprintf(msg, args...))
}

func (s *slogAdapter) Sub(module string) waLog.Logger {
	return &slogAdapter{log: s.log.With("module", module)}
}

// Ensure slogAdapter implements waLog.Logger
var _ waLog.Logger = (*slogAdapter)(nil)

// Package bridge provides the core WhatsApp bridge functionality.
package bridge

import (
	"time"
)

// EventType represents the type of bridge event.
type EventType int

const (
	EventMessage EventType = iota
	EventReceipt
	EventPresence
	EventGroupUpdate
	EventConnectionChange
	EventHistorySync
	EventCallOffer
	EventChatArchive
	EventQRCode
)

// String returns the string representation of the event type.
func (e EventType) String() string {
	switch e {
	case EventMessage:
		return "message"
	case EventReceipt:
		return "receipt"
	case EventPresence:
		return "presence"
	case EventGroupUpdate:
		return "group_update"
	case EventConnectionChange:
		return "connection_change"
	case EventHistorySync:
		return "history_sync"
	case EventCallOffer:
		return "call_offer"
	case EventChatArchive:
		return "chat_archive"
	case EventQRCode:
		return "qr_code"
	default:
		return "unknown"
	}
}

// Event represents a bridge event.
type Event struct {
	Type      EventType
	Payload   interface{}
	Timestamp time.Time
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(t EventType, payload interface{}) Event {
	return Event{
		Type:      t,
		Payload:   payload,
		Timestamp: time.Now(),
	}
}

// MessagePayload contains data for message events.
type MessagePayload struct {
	ID        string
	ChatJID   string
	Sender    string
	Content   string
	IsFromMe  bool
	MediaType string
	Timestamp time.Time
}

// QRCodePayload contains data for QR code events.
type QRCodePayload struct {
	Code string
}

// ConnectionPayload contains data for connection events.
type ConnectionPayload struct {
	Connected bool
	Reason    string
}

// Package store provides data persistence for the WhatsApp bridge.
package store

import (
	"time"

	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/state"
)

// Message represents a WhatsApp message.
type Message struct {
	ID           string
	ChatJID      string
	Sender       string
	Content      string
	Timestamp    time.Time
	IsFromMe     bool
	MediaType    string
	Filename     string
	MediaURL     string
	MediaKey     []byte
	QuotedID     string
	QuotedSender string
}

// Chat represents a WhatsApp chat.
type Chat struct {
	JID             string
	Name            string
	IsGroup         bool
	LastMessageTime time.Time
	UnreadCount     int
	Archived        bool
	Pinned          bool
	MutedUntil      *time.Time
}

// Contact represents a WhatsApp contact.
type Contact struct {
	JID          string
	Name         string
	PushName     string
	BusinessName string
	Blocked      bool
}

// Transition represents a state machine transition record.
type Transition struct {
	ID        int64
	FromState state.State
	ToState   state.State
	Trigger   string
	Timestamp time.Time
	Error     string
}

// Session represents the bridge session state.
type Session struct {
	ID        int64
	State     state.State
	CreatedAt time.Time
	UpdatedAt time.Time
}

// QueryOpts provides options for querying messages.
type QueryOpts struct {
	Limit     int
	Offset    int
	Before    *time.Time
	After     *time.Time
	MediaOnly bool
	FromMe    *bool
}

// DefaultQueryOpts returns default query options.
func DefaultQueryOpts() QueryOpts {
	return QueryOpts{
		Limit:  50,
		Offset: 0,
	}
}

// Package store provides data persistence for the WhatsApp bridge.
package store

import (
	"time"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
)

// Message represents a WhatsApp message.
type Message struct {
	ID           string    `json:"id"`
	ChatJID      string    `json:"chat_jid"`
	Sender       string    `json:"sender"`
	Content      string    `json:"content"`
	Timestamp    time.Time `json:"timestamp"`
	IsFromMe     bool      `json:"is_from_me"`
	MediaType    string    `json:"media_type,omitempty"`
	Filename     string    `json:"filename,omitempty"`
	MediaURL     string    `json:"media_url,omitempty"`
	MediaKey     []byte    `json:"-"`
	FileSHA256   []byte    `json:"-"`
	FileLength   uint64    `json:"file_length,omitempty"`
	QuotedID     string    `json:"quoted_id,omitempty"`
	QuotedSender string    `json:"quoted_sender,omitempty"`
	IsStarred    bool      `json:"is_starred"`
	IsDeleted    bool      `json:"is_deleted"`
	Reactions    []string  `json:"reactions,omitempty"`
}

// Chat represents a WhatsApp chat.
type Chat struct {
	JID             string     `json:"jid"`
	Name            string     `json:"name"`
	IsGroup         bool       `json:"is_group"`
	LastMessageTime time.Time  `json:"last_message_time"`
	UnreadCount     int        `json:"unread_count"`
	Archived        bool       `json:"archived"`
	Pinned          bool       `json:"pinned"`
	Muted           bool       `json:"muted"`
	MutedUntil      *time.Time `json:"muted_until,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Contact represents a WhatsApp contact.
type Contact struct {
	JID          string    `json:"jid"`
	Name         string    `json:"name"`
	PushName     string    `json:"push_name"`
	Phone        string    `json:"phone,omitempty"`
	BusinessName string    `json:"business_name,omitempty"`
	Blocked      bool      `json:"blocked"`
	IsSaved      bool      `json:"is_saved"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Group represents a WhatsApp group.
type Group struct {
	JID               string    `json:"jid"`
	Name              string    `json:"name"`
	Topic             string    `json:"topic,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	CreatedBy         string    `json:"created_by"`
	InviteLink        string    `json:"invite_link,omitempty"`
	IsAnnounce        bool      `json:"is_announce"`
	IsLocked          bool      `json:"is_locked"`
	ParticipantCount  int       `json:"participant_count"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// GroupParticipant represents a group member.
type GroupParticipant struct {
	GroupJID  string    `json:"group_jid"`
	UserJID   string    `json:"user_jid"`
	Role      string    `json:"role"` // member, admin, superadmin
	JoinedAt  time.Time `json:"joined_at"`
}

// StatusUpdate represents a WhatsApp status/story.
type StatusUpdate struct {
	ID        string    `json:"id"`
	SenderJID string    `json:"sender_jid"`
	MediaType string    `json:"media_type,omitempty"`
	Content   string    `json:"content,omitempty"`
	PostedAt  time.Time `json:"posted_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Viewed    bool      `json:"viewed"`
}

// Transition represents a state machine transition record.
type Transition struct {
	ID        int64       `json:"id"`
	FromState state.State `json:"from_state"`
	ToState   state.State `json:"to_state"`
	Trigger   string      `json:"trigger"`
	Timestamp time.Time   `json:"timestamp"`
	Error     string      `json:"error,omitempty"`
}

// Session represents the bridge session state.
type Session struct {
	ID        int64
	State     state.State
	CreatedAt time.Time
	UpdatedAt time.Time
}

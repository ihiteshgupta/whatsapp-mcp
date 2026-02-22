package store

import (
	"context"
	"errors"
	"time"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
)

// ErrNotFound is returned when a requested item is not found.
var ErrNotFound = errors.New("not found")

// QueryOpts provides options for list queries.
type QueryOpts struct {
	Limit  int
	Offset int
	Before string // For cursor-based pagination
}

// MessageRepository defines operations for message persistence.
type MessageRepository interface {
	Store(ctx context.Context, msg *Message) error
	List(ctx context.Context, chatJID string, limit int, before string) ([]Message, error)
	GetByID(ctx context.Context, chatJID, msgID string) (*Message, error)
	Search(ctx context.Context, query string, limit int) ([]Message, error)
	SetStarred(ctx context.Context, chatJID, msgID string, starred bool) error
	Delete(ctx context.Context, chatJID, msgID string) error
	Count(ctx context.Context, chatJID string) (int, error)
}

// ChatRepository defines operations for chat persistence.
type ChatRepository interface {
	Upsert(ctx context.Context, chat *Chat) error
	List(ctx context.Context, limit int) ([]Chat, error)
	GetByJID(ctx context.Context, jid string) (*Chat, error)
	UpdateLastMessage(ctx context.Context, jid string, t time.Time) error
	Archive(ctx context.Context, jid string, archived bool) error
	Pin(ctx context.Context, jid string, pinned bool) error
	Mute(ctx context.Context, jid string, muted bool, until *time.Time) error
	Delete(ctx context.Context, jid string) error
	Count(ctx context.Context) (int, error)
}

// ContactRepository defines operations for contact persistence.
type ContactRepository interface {
	Upsert(ctx context.Context, contact *Contact) error
	Search(ctx context.Context, query string, limit int) ([]Contact, error)
	GetByJID(ctx context.Context, jid string) (*Contact, error)
	Block(ctx context.Context, jid string, blocked bool) error
	GetBlocked(ctx context.Context) ([]Contact, error)
	Delete(ctx context.Context, jid string) error
	Count(ctx context.Context) (int, error)
}

// GroupRepository defines operations for group persistence.
type GroupRepository interface {
	Upsert(ctx context.Context, group *Group) error
	GetByJID(ctx context.Context, jid string) (*Group, error)
	UpdateParticipants(ctx context.Context, groupJID string, participants []GroupParticipant) error
	GetParticipants(ctx context.Context, groupJID string) ([]GroupParticipant, error)
	Delete(ctx context.Context, jid string) error
}

// StatusRepository defines operations for status persistence.
type StatusRepository interface {
	Store(ctx context.Context, status *StatusUpdate) error
	GetAll(ctx context.Context) ([]StatusUpdate, error)
	GetByContact(ctx context.Context, contactJID string) ([]StatusUpdate, error)
	Delete(ctx context.Context, statusID string) error
	DeleteExpired(ctx context.Context) error
}

// StateRepository defines operations for state persistence.
type StateRepository interface {
	GetState(ctx context.Context) (state.State, error)
	SaveState(ctx context.Context, s state.State) error
	LogTransition(ctx context.Context, from, to state.State, trigger string) error
	GetTransitionHistory(ctx context.Context, limit int) ([]Transition, error)
}

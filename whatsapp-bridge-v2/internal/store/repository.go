package store

import (
	"context"
	"time"

	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/state"
)

// MessageRepository defines operations for message persistence.
type MessageRepository interface {
	Store(ctx context.Context, msg *Message) error
	GetByChat(ctx context.Context, chatJID string, opts QueryOpts) ([]Message, error)
	GetByID(ctx context.Context, chatJID, msgID string) (*Message, error)
	Search(ctx context.Context, query string, opts QueryOpts) ([]Message, error)
	Delete(ctx context.Context, chatJID, msgID string) error
	Count(ctx context.Context, chatJID string) (int, error)
}

// ChatRepository defines operations for chat persistence.
type ChatRepository interface {
	Upsert(ctx context.Context, chat *Chat) error
	GetAll(ctx context.Context) ([]Chat, error)
	GetByJID(ctx context.Context, jid string) (*Chat, error)
	UpdateLastMessage(ctx context.Context, jid string, t time.Time) error
	Archive(ctx context.Context, jid string, archived bool) error
	Pin(ctx context.Context, jid string, pinned bool) error
	Mute(ctx context.Context, jid string, until *time.Time) error
	Delete(ctx context.Context, jid string) error
	Count(ctx context.Context) (int, error)
}

// ContactRepository defines operations for contact persistence.
type ContactRepository interface {
	Upsert(ctx context.Context, contact *Contact) error
	Search(ctx context.Context, query string) ([]Contact, error)
	GetByJID(ctx context.Context, jid string) (*Contact, error)
	Block(ctx context.Context, jid string, blocked bool) error
	GetBlocked(ctx context.Context) ([]Contact, error)
	Delete(ctx context.Context, jid string) error
	Count(ctx context.Context) (int, error)
}

// StateRepository defines operations for state persistence.
type StateRepository interface {
	GetState(ctx context.Context) (state.State, error)
	SaveState(ctx context.Context, s state.State) error
	LogTransition(ctx context.Context, from, to state.State, trigger string) error
	GetTransitionHistory(ctx context.Context, limit int) ([]Transition, error)
}

// Store aggregates all repositories.
type Store struct {
	Messages  MessageRepository
	Chats     ChatRepository
	Contacts  ContactRepository
	State     StateRepository
}

// NewStore creates a new Store with the given repositories.
func NewStore(messages MessageRepository, chats ChatRepository, contacts ContactRepository, stateRepo StateRepository) *Store {
	return &Store{
		Messages: messages,
		Chats:    chats,
		Contacts: contacts,
		State:    stateRepo,
	}
}

package store

import (
	"context"
	"testing"
	"time"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *SQLiteStore {
	store, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

// Message Repository Tests

func TestSQLiteMessageRepo_Store(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// First create a chat
	chat := &Chat{JID: "123@s.whatsapp.net", Name: "Test Chat"}
	err := store.Chats.Upsert(ctx, chat)
	require.NoError(t, err)

	msg := &Message{
		ID:        "msg1",
		ChatJID:   "123@s.whatsapp.net",
		Sender:    "456@s.whatsapp.net",
		Content:   "Hello World",
		Timestamp: time.Now(),
		IsFromMe:  false,
	}

	err = store.Messages.Store(ctx, msg)
	require.NoError(t, err)

	// Verify it was stored
	retrieved, err := store.Messages.GetByID(ctx, msg.ChatJID, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, msg.Content, retrieved.Content)
	assert.Equal(t, msg.Sender, retrieved.Sender)
}

func TestSQLiteMessageRepo_GetByChat(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Create chat
	chat := &Chat{JID: "123@s.whatsapp.net", Name: "Test Chat"}
	err := store.Chats.Upsert(ctx, chat)
	require.NoError(t, err)

	// Store multiple messages
	now := time.Now()
	for i := 0; i < 5; i++ {
		msg := &Message{
			ID:        "msg" + string(rune('0'+i)),
			ChatJID:   "123@s.whatsapp.net",
			Sender:    "456@s.whatsapp.net",
			Content:   "Message " + string(rune('0'+i)),
			Timestamp: now.Add(time.Duration(i) * time.Minute),
		}
		err := store.Messages.Store(ctx, msg)
		require.NoError(t, err)
	}

	// Get with limit
	messages, err := store.Messages.List(ctx, "123@s.whatsapp.net", 3, "")
	require.NoError(t, err)
	assert.Len(t, messages, 3)

	// Messages should be in descending order by timestamp
	assert.True(t, messages[0].Timestamp.After(messages[1].Timestamp))
}

func TestSQLiteMessageRepo_Search(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Create chat
	chat := &Chat{JID: "123@s.whatsapp.net", Name: "Test Chat"}
	err := store.Chats.Upsert(ctx, chat)
	require.NoError(t, err)

	// Store messages
	messages := []Message{
		{ID: "1", ChatJID: "123@s.whatsapp.net", Sender: "a", Content: "Hello World", Timestamp: time.Now()},
		{ID: "2", ChatJID: "123@s.whatsapp.net", Sender: "a", Content: "Goodbye World", Timestamp: time.Now()},
		{ID: "3", ChatJID: "123@s.whatsapp.net", Sender: "a", Content: "No match here", Timestamp: time.Now()},
	}
	for _, msg := range messages {
		err := store.Messages.Store(ctx, &msg)
		require.NoError(t, err)
	}

	// Search for "World"
	results, err := store.Messages.Search(ctx, "World", 50)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSQLiteMessageRepo_Delete(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Create chat and message
	chat := &Chat{JID: "123@s.whatsapp.net", Name: "Test Chat"}
	err := store.Chats.Upsert(ctx, chat)
	require.NoError(t, err)

	msg := &Message{ID: "msg1", ChatJID: "123@s.whatsapp.net", Sender: "a", Content: "Test", Timestamp: time.Now()}
	err = store.Messages.Store(ctx, msg)
	require.NoError(t, err)

	// Delete
	err = store.Messages.Delete(ctx, msg.ChatJID, msg.ID)
	require.NoError(t, err)

	// Verify deleted
	_, err = store.Messages.GetByID(ctx, msg.ChatJID, msg.ID)
	assert.Error(t, err)
}

// Chat Repository Tests

func TestSQLiteChatRepo_Upsert(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	chat := &Chat{
		JID:     "123@s.whatsapp.net",
		Name:    "Test Chat",
		IsGroup: false,
	}

	err := store.Chats.Upsert(ctx, chat)
	require.NoError(t, err)

	// Update the chat
	chat.Name = "Updated Chat"
	err = store.Chats.Upsert(ctx, chat)
	require.NoError(t, err)

	// Verify update
	retrieved, err := store.Chats.GetByJID(ctx, chat.JID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Chat", retrieved.Name)
}

func TestSQLiteChatRepo_GetAll(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Create chats
	chats := []Chat{
		{JID: "1@s.whatsapp.net", Name: "Chat 1"},
		{JID: "2@s.whatsapp.net", Name: "Chat 2"},
		{JID: "3@s.whatsapp.net", Name: "Chat 3"},
	}
	for _, c := range chats {
		err := store.Chats.Upsert(ctx, &c)
		require.NoError(t, err)
	}

	// Get all
	all, err := store.Chats.List(ctx, 50)
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestSQLiteChatRepo_Archive(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	chat := &Chat{JID: "123@s.whatsapp.net", Name: "Test"}
	err := store.Chats.Upsert(ctx, chat)
	require.NoError(t, err)

	// Archive
	err = store.Chats.Archive(ctx, chat.JID, true)
	require.NoError(t, err)

	// Verify
	retrieved, err := store.Chats.GetByJID(ctx, chat.JID)
	require.NoError(t, err)
	assert.True(t, retrieved.Archived)

	// Unarchive
	err = store.Chats.Archive(ctx, chat.JID, false)
	require.NoError(t, err)
	retrieved, _ = store.Chats.GetByJID(ctx, chat.JID)
	assert.False(t, retrieved.Archived)
}

func TestSQLiteChatRepo_Pin(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	chat := &Chat{JID: "123@s.whatsapp.net", Name: "Test"}
	err := store.Chats.Upsert(ctx, chat)
	require.NoError(t, err)

	// Pin
	err = store.Chats.Pin(ctx, chat.JID, true)
	require.NoError(t, err)

	retrieved, _ := store.Chats.GetByJID(ctx, chat.JID)
	assert.True(t, retrieved.Pinned)
}

func TestSQLiteChatRepo_Mute(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	chat := &Chat{JID: "123@s.whatsapp.net", Name: "Test"}
	err := store.Chats.Upsert(ctx, chat)
	require.NoError(t, err)

	// Mute until tomorrow
	muteUntil := time.Now().Add(24 * time.Hour)
	err = store.Chats.Mute(ctx, chat.JID, true, &muteUntil)
	require.NoError(t, err)

	retrieved, _ := store.Chats.GetByJID(ctx, chat.JID)
	require.NotNil(t, retrieved.MutedUntil)
	assert.WithinDuration(t, muteUntil, *retrieved.MutedUntil, time.Second)

	// Unmute
	err = store.Chats.Mute(ctx, chat.JID, false, nil)
	require.NoError(t, err)
	retrieved, _ = store.Chats.GetByJID(ctx, chat.JID)
	assert.Nil(t, retrieved.MutedUntil)
}

// Contact Repository Tests

func TestSQLiteContactRepo_Upsert(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	contact := &Contact{
		JID:      "123@s.whatsapp.net",
		Name:     "John Doe",
		PushName: "John",
	}

	err := store.Contacts.Upsert(ctx, contact)
	require.NoError(t, err)

	retrieved, err := store.Contacts.GetByJID(ctx, contact.JID)
	require.NoError(t, err)
	assert.Equal(t, "John Doe", retrieved.Name)
}

func TestSQLiteContactRepo_Search(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	contacts := []Contact{
		{JID: "1@s.whatsapp.net", Name: "John Doe"},
		{JID: "2@s.whatsapp.net", Name: "Jane Doe"},
		{JID: "3@s.whatsapp.net", Name: "Bob Smith"},
	}
	for _, c := range contacts {
		err := store.Contacts.Upsert(ctx, &c)
		require.NoError(t, err)
	}

	// Search for "Doe"
	results, err := store.Contacts.Search(ctx, "Doe", 20)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSQLiteContactRepo_Block(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	contact := &Contact{JID: "123@s.whatsapp.net", Name: "Test"}
	err := store.Contacts.Upsert(ctx, contact)
	require.NoError(t, err)

	// Block
	err = store.Contacts.Block(ctx, contact.JID, true)
	require.NoError(t, err)

	blocked, err := store.Contacts.GetBlocked(ctx)
	require.NoError(t, err)
	assert.Len(t, blocked, 1)

	// Unblock
	err = store.Contacts.Block(ctx, contact.JID, false)
	require.NoError(t, err)
	blocked, _ = store.Contacts.GetBlocked(ctx)
	assert.Len(t, blocked, 0)
}

// State Repository Tests

func TestSQLiteStateRepo_SaveAndGet(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Default state should be disconnected
	s, err := store.State.GetState(ctx)
	require.NoError(t, err)
	assert.Equal(t, state.StateDisconnected, s)

	// Save new state
	err = store.State.SaveState(ctx, state.StateReady)
	require.NoError(t, err)

	// Get updated state
	s, err = store.State.GetState(ctx)
	require.NoError(t, err)
	assert.Equal(t, state.StateReady, s)
}

func TestSQLiteStateRepo_LogTransition(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Log some transitions
	err := store.State.LogTransition(ctx, state.StateDisconnected, state.StateConnecting, "connect")
	require.NoError(t, err)
	err = store.State.LogTransition(ctx, state.StateConnecting, state.StateReady, "authenticated")
	require.NoError(t, err)

	// Get history
	history, err := store.State.GetTransitionHistory(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, history, 2)

	// Most recent first
	assert.Equal(t, state.StateConnecting, history[0].FromState)
	assert.Equal(t, state.StateReady, history[0].ToState)
}

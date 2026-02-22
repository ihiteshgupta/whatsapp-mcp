package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/config"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/health"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestHandler(t *testing.T) (*Handler, *store.SQLiteStore) {
	storeDB, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { storeDB.Close() })

	cfg := config.DefaultConfig()
	sm := state.NewMachine()
	hm := health.NewMonitor(cfg, sm)

	handler := NewHandler(storeDB, hm, nil, sm)
	return handler, storeDB
}

func TestNewHandler(t *testing.T) {
	handler, _ := setupTestHandler(t)
	assert.NotNil(t, handler)
}

func TestHandler_GetTools(t *testing.T) {
	handler, _ := setupTestHandler(t)
	tools := handler.GetTools()

	assert.GreaterOrEqual(t, len(tools), 5)

	// Check for essential tools
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames[ToolSendMessage])
	assert.True(t, toolNames[ToolListChats])
	assert.True(t, toolNames[ToolGetBridgeStatus])
}

func TestHandler_HandleGetBridgeStatus(t *testing.T) {
	handler, _ := setupTestHandler(t)
	ctx := context.Background()

	result, err := handler.HandleTool(ctx, ToolGetBridgeStatus, map[string]interface{}{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestHandler_HandleListChats(t *testing.T) {
	handler, storeDB := setupTestHandler(t)
	ctx := context.Background()

	// Add some chats
	err := storeDB.Chats.Upsert(ctx, &store.Chat{JID: "1@s.whatsapp.net", Name: "Chat 1"})
	require.NoError(t, err)
	err = storeDB.Chats.Upsert(ctx, &store.Chat{JID: "2@s.whatsapp.net", Name: "Chat 2"})
	require.NoError(t, err)

	result, err := handler.HandleTool(ctx, ToolListChats, map[string]interface{}{"limit": 10})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Parse result
	require.Len(t, result.Content, 1)
	var chats []store.Chat
	err = json.Unmarshal([]byte(result.Content[0].Text), &chats)
	require.NoError(t, err)
	assert.Len(t, chats, 2)
}

func TestHandler_HandleGetChat(t *testing.T) {
	handler, storeDB := setupTestHandler(t)
	ctx := context.Background()

	// Add a chat
	err := storeDB.Chats.Upsert(ctx, &store.Chat{JID: "test@s.whatsapp.net", Name: "Test Chat"})
	require.NoError(t, err)

	result, err := handler.HandleTool(ctx, ToolGetChat, map[string]interface{}{"jid": "test@s.whatsapp.net"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Parse result
	require.Len(t, result.Content, 1)
	var chat store.Chat
	err = json.Unmarshal([]byte(result.Content[0].Text), &chat)
	require.NoError(t, err)
	assert.Equal(t, "Test Chat", chat.Name)
}

func TestHandler_HandleGetChat_NotFound(t *testing.T) {
	handler, _ := setupTestHandler(t)
	ctx := context.Background()

	result, err := handler.HandleTool(ctx, ToolGetChat, map[string]interface{}{"jid": "nonexistent@s.whatsapp.net"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestHandler_HandleSearchContacts(t *testing.T) {
	handler, storeDB := setupTestHandler(t)
	ctx := context.Background()

	// Add contacts
	err := storeDB.Contacts.Upsert(ctx, &store.Contact{JID: "1@s.whatsapp.net", Name: "John Doe"})
	require.NoError(t, err)
	err = storeDB.Contacts.Upsert(ctx, &store.Contact{JID: "2@s.whatsapp.net", Name: "Jane Doe"})
	require.NoError(t, err)

	result, err := handler.HandleTool(ctx, ToolSearchContacts, map[string]interface{}{"query": "Doe"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Parse result
	require.Len(t, result.Content, 1)
	var contacts []store.Contact
	err = json.Unmarshal([]byte(result.Content[0].Text), &contacts)
	require.NoError(t, err)
	assert.Len(t, contacts, 2)
}

func TestHandler_HandleArchiveChat_RequiresBridge(t *testing.T) {
	handler, storeDB := setupTestHandler(t)
	ctx := context.Background()

	// Add a chat
	err := storeDB.Chats.Upsert(ctx, &store.Chat{JID: "test@s.whatsapp.net", Name: "Test"})
	require.NoError(t, err)

	// Archive should fail because bridge is not ready (nil)
	result, err := handler.HandleTool(ctx, ToolArchiveChat, map[string]interface{}{
		"jid": "test@s.whatsapp.net",
	})
	require.NoError(t, err)
	// Should return an error result because bridge is not ready
	assert.True(t, result.IsError)
}

func TestHandler_HandleConnectionHistory(t *testing.T) {
	handler, storeDB := setupTestHandler(t)
	ctx := context.Background()

	// Log some transitions
	_ = storeDB.State.LogTransition(ctx, state.StateDisconnected, state.StateConnecting, "connect")
	_ = storeDB.State.LogTransition(ctx, state.StateConnecting, state.StateReady, "authenticated")

	result, err := handler.HandleTool(ctx, ToolGetConnectionHistory, map[string]interface{}{"limit": 10})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Parse result
	require.Len(t, result.Content, 1)
	var history []store.Transition
	err = json.Unmarshal([]byte(result.Content[0].Text), &history)
	require.NoError(t, err)
	assert.Len(t, history, 2)
}

func TestHandler_HandleUnknownTool(t *testing.T) {
	handler, _ := setupTestHandler(t)
	ctx := context.Background()

	// Unknown tool returns an error result (not a Go error)
	result, err := handler.HandleTool(ctx, "unknown_tool", map[string]interface{}{})
	require.NoError(t, err) // No Go error
	require.NotNil(t, result)
	assert.True(t, result.IsError) // But the result is marked as error
}

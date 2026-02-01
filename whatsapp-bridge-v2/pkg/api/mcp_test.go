package api

import (
	"context"
	"sync"
	"testing"

	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/config"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/health"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/state"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FakeWhatsAppClient for testing
type FakeWhatsAppClient struct {
	mu           sync.Mutex
	connected    bool
	loggedIn     bool
	sentMessages []struct{ JID, Content string }
}

func (f *FakeWhatsAppClient) Connect() error { f.connected = true; return nil }
func (f *FakeWhatsAppClient) Disconnect()    { f.connected = false }
func (f *FakeWhatsAppClient) IsConnected() bool { return f.connected }
func (f *FakeWhatsAppClient) IsLoggedIn() bool  { return f.loggedIn }
func (f *FakeWhatsAppClient) SendMessage(ctx context.Context, jid, text string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sentMessages = append(f.sentMessages, struct{ JID, Content string }{jid, text})
	return "msg-123", nil
}
func (f *FakeWhatsAppClient) SendMedia(ctx context.Context, jid string, data []byte, mimeType, filename string) (string, error) {
	return "media-123", nil
}
func (f *FakeWhatsAppClient) GetQRChannel() (<-chan string, error) { return make(chan string), nil }
func (f *FakeWhatsAppClient) AddEventHandler(handler func(interface{})) {}

func setupTestMCPServer(t *testing.T) *MCPServer {
	storeDB, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { storeDB.Close() })

	cfg := config.DefaultConfig()
	sm := state.NewMachine()
	hm := health.NewMonitor(cfg, sm)

	server := NewMCPServer(storeDB, sm, hm)
	return server
}

func TestNewMCPServer(t *testing.T) {
	server := setupTestMCPServer(t)
	assert.NotNil(t, server)
}

func TestMCPServer_GetTools(t *testing.T) {
	server := setupTestMCPServer(t)
	tools := server.GetTools()

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

func TestMCPServer_HandleGetBridgeStatus(t *testing.T) {
	server := setupTestMCPServer(t)
	ctx := context.Background()

	result, err := server.HandleTool(ctx, ToolGetBridgeStatus, map[string]interface{}{})
	require.NoError(t, err)

	status, ok := result.(health.Status)
	require.True(t, ok)
	assert.Equal(t, string(state.StateDisconnected), status.State)
}

func TestMCPServer_HandleListChats(t *testing.T) {
	server := setupTestMCPServer(t)
	ctx := context.Background()

	// Add some chats
	err := server.store.Chats.Upsert(ctx, &store.Chat{JID: "1@s.whatsapp.net", Name: "Chat 1"})
	require.NoError(t, err)
	err = server.store.Chats.Upsert(ctx, &store.Chat{JID: "2@s.whatsapp.net", Name: "Chat 2"})
	require.NoError(t, err)

	result, err := server.HandleTool(ctx, ToolListChats, map[string]interface{}{"limit": 10})
	require.NoError(t, err)

	chats, ok := result.([]store.Chat)
	require.True(t, ok)
	assert.Len(t, chats, 2)
}

func TestMCPServer_HandleGetChat(t *testing.T) {
	server := setupTestMCPServer(t)
	ctx := context.Background()

	// Add a chat
	err := server.store.Chats.Upsert(ctx, &store.Chat{JID: "test@s.whatsapp.net", Name: "Test Chat"})
	require.NoError(t, err)

	result, err := server.HandleTool(ctx, ToolGetChat, map[string]interface{}{"jid": "test@s.whatsapp.net"})
	require.NoError(t, err)

	chat, ok := result.(*store.Chat)
	require.True(t, ok)
	assert.Equal(t, "Test Chat", chat.Name)
}

func TestMCPServer_HandleGetChat_NotFound(t *testing.T) {
	server := setupTestMCPServer(t)
	ctx := context.Background()

	_, err := server.HandleTool(ctx, ToolGetChat, map[string]interface{}{"jid": "nonexistent@s.whatsapp.net"})
	assert.Error(t, err)

	mcpErr, ok := err.(*MCPError)
	require.True(t, ok)
	assert.Equal(t, ErrNotFound, mcpErr.Code)
}

func TestMCPServer_HandleSearchContacts(t *testing.T) {
	server := setupTestMCPServer(t)
	ctx := context.Background()

	// Add contacts
	err := server.store.Contacts.Upsert(ctx, &store.Contact{JID: "1@s.whatsapp.net", Name: "John Doe"})
	require.NoError(t, err)
	err = server.store.Contacts.Upsert(ctx, &store.Contact{JID: "2@s.whatsapp.net", Name: "Jane Doe"})
	require.NoError(t, err)

	result, err := server.HandleTool(ctx, ToolSearchContacts, map[string]interface{}{"query": "Doe"})
	require.NoError(t, err)

	contacts, ok := result.([]store.Contact)
	require.True(t, ok)
	assert.Len(t, contacts, 2)
}

func TestMCPServer_HandleArchiveChat(t *testing.T) {
	server := setupTestMCPServer(t)
	ctx := context.Background()

	// Add a chat
	err := server.store.Chats.Upsert(ctx, &store.Chat{JID: "test@s.whatsapp.net", Name: "Test"})
	require.NoError(t, err)

	// Archive
	_, err = server.HandleTool(ctx, ToolArchiveChat, map[string]interface{}{
		"jid":     "test@s.whatsapp.net",
		"archive": true,
	})
	require.NoError(t, err)

	// Verify
	chat, err := server.store.Chats.GetByJID(ctx, "test@s.whatsapp.net")
	require.NoError(t, err)
	assert.True(t, chat.Archived)
}

func TestMCPServer_HandleConnectionHistory(t *testing.T) {
	server := setupTestMCPServer(t)
	ctx := context.Background()

	// Log some transitions
	_ = server.store.State.LogTransition(ctx, state.StateDisconnected, state.StateConnecting, "connect")
	_ = server.store.State.LogTransition(ctx, state.StateConnecting, state.StateReady, "authenticated")

	result, err := server.HandleTool(ctx, ToolGetConnectionHistory, map[string]interface{}{"limit": 10})
	require.NoError(t, err)

	history, ok := result.([]store.Transition)
	require.True(t, ok)
	assert.Len(t, history, 2)
}

func TestMCPServer_HandleUnknownTool(t *testing.T) {
	server := setupTestMCPServer(t)
	ctx := context.Background()

	_, err := server.HandleTool(ctx, "unknown_tool", map[string]interface{}{})
	assert.Error(t, err)
}

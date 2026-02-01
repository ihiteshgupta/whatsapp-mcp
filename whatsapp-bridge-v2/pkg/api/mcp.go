package api

import (
	"context"
	"fmt"

	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/health"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/state"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/store"
)

// MCPServer handles MCP tool requests.
type MCPServer struct {
	store        *store.SQLiteStore
	stateMachine *state.Machine
	health       *health.Monitor
}

// NewMCPServer creates a new MCP server.
func NewMCPServer(storeDB *store.SQLiteStore, sm *state.Machine, hm *health.Monitor) *MCPServer {
	return &MCPServer{
		store:        storeDB,
		stateMachine: sm,
		health:       hm,
	}
}

// GetTools returns all available tool definitions.
func (m *MCPServer) GetTools() []ToolDefinition {
	return GetToolDefinitions()
}

// HandleTool handles a tool invocation.
func (m *MCPServer) HandleTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	case ToolGetBridgeStatus:
		return m.handleGetBridgeStatus(ctx, args)
	case ToolListChats:
		return m.handleListChats(ctx, args)
	case ToolGetChat:
		return m.handleGetChat(ctx, args)
	case ToolSearchContacts:
		return m.handleSearchContacts(ctx, args)
	case ToolArchiveChat:
		return m.handleArchiveChat(ctx, args)
	case ToolBlockContact:
		return m.handleBlockContact(ctx, args)
	case ToolGetConnectionHistory:
		return m.handleGetConnectionHistory(ctx, args)
	default:
		return nil, NewInvalidInputError(fmt.Sprintf("Unknown tool: %s", toolName))
	}
}

func (m *MCPServer) handleGetBridgeStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return m.health.GetStatus(), nil
}

func (m *MCPServer) handleListChats(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	chats, err := m.store.Chats.GetAll(ctx)
	if err != nil {
		return nil, NewInternalError(err)
	}
	return chats, nil
}

func (m *MCPServer) handleGetChat(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	jid, ok := args["jid"].(string)
	if !ok || jid == "" {
		return nil, NewInvalidInputError("jid is required")
	}

	chat, err := m.store.Chats.GetByJID(ctx, jid)
	if err == store.ErrNotFound {
		return nil, NewNotFoundError("chat")
	}
	if err != nil {
		return nil, NewInternalError(err)
	}
	return chat, nil
}

func (m *MCPServer) handleSearchContacts(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, NewInvalidInputError("query is required")
	}

	contacts, err := m.store.Contacts.Search(ctx, query)
	if err != nil {
		return nil, NewInternalError(err)
	}
	return contacts, nil
}

func (m *MCPServer) handleArchiveChat(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	jid, ok := args["jid"].(string)
	if !ok || jid == "" {
		return nil, NewInvalidInputError("jid is required")
	}

	archive, ok := args["archive"].(bool)
	if !ok {
		return nil, NewInvalidInputError("archive is required")
	}

	if err := m.store.Chats.Archive(ctx, jid, archive); err != nil {
		return nil, NewInternalError(err)
	}

	return map[string]interface{}{"success": true}, nil
}

func (m *MCPServer) handleBlockContact(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	jid, ok := args["jid"].(string)
	if !ok || jid == "" {
		return nil, NewInvalidInputError("jid is required")
	}

	block, ok := args["block"].(bool)
	if !ok {
		return nil, NewInvalidInputError("block is required")
	}

	if err := m.store.Contacts.Block(ctx, jid, block); err != nil {
		return nil, NewInternalError(err)
	}

	return map[string]interface{}{"success": true}, nil
}

func (m *MCPServer) handleGetConnectionHistory(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	if l, ok := args["limit"].(int); ok {
		limit = l
	}

	history, err := m.store.State.GetTransitionHistory(ctx, limit)
	if err != nil {
		return nil, NewInternalError(err)
	}
	return history, nil
}

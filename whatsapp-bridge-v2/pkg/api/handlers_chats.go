package api

import (
	"context"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/store"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Chat tool handlers

func (h *Handler) handleListChats(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	limit := getInt(args, "limit", 50)

	chats, err := h.store.Chats.List(ctx, limit)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(chats)
}

func (h *Handler) handleGetChat(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	chat, err := h.store.Chats.GetByJID(ctx, jid)
	if err == store.ErrNotFound {
		return h.errorResult(NewNotFoundError("chat"))
	}
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(chat)
}

func (h *Handler) handleListMessages(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	chatJID := getString(args, "chat_jid")
	if chatJID == "" {
		return h.errorResult(NewInvalidInputError("chat_jid is required"))
	}

	limit := getInt(args, "limit", 50)
	before := getString(args, "before")

	messages, err := h.store.Messages.List(ctx, chatJID, limit, before)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(messages)
}

func (h *Handler) handleArchiveChat(ctx context.Context, args map[string]interface{}, archive bool) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	if err := h.bridge.ArchiveChat(ctx, jid, archive); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	action := "archived"
	if !archive {
		action = "unarchived"
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Chat " + action,
	})
}

func (h *Handler) handlePinChat(ctx context.Context, args map[string]interface{}, pin bool) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	if err := h.bridge.PinChat(ctx, jid, pin); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	action := "pinned"
	if !pin {
		action = "unpinned"
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Chat " + action,
	})
}

func (h *Handler) handleMuteChat(ctx context.Context, args map[string]interface{}, mute bool) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	duration := getString(args, "duration")

	if err := h.bridge.MuteChat(ctx, jid, mute, duration); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	action := "muted"
	if !mute {
		action = "unmuted"
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Chat " + action,
	})
}

func (h *Handler) handleMarkChatRead(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	if err := h.bridge.MarkChatRead(ctx, jid); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Chat marked as read",
	})
}

func (h *Handler) handleDeleteChat(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	if err := h.bridge.DeleteChat(ctx, jid); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Chat deleted",
	})
}

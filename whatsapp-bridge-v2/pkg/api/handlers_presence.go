package api

import (
	"context"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Presence tool handlers

func (h *Handler) handleSubscribePresence(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	if err := h.bridge.SubscribePresence(ctx, jid); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Subscribed to presence",
	})
}

func (h *Handler) handleSendTyping(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	if err := h.bridge.SendTyping(ctx, jid); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Typing indicator sent",
	})
}

func (h *Handler) handleSendRecording(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	if err := h.bridge.SendRecording(ctx, jid); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Recording indicator sent",
	})
}

func (h *Handler) handleSetOnline(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if err := h.bridge.SetOnline(ctx); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Set to online",
	})
}

func (h *Handler) handleSetOffline(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if err := h.bridge.SetOffline(ctx); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Set to offline",
	})
}

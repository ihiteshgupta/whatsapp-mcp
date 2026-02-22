package api

import (
	"context"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Bridge tool handlers

func (h *Handler) handleGetBridgeStatus(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	status := h.health.GetStatus()
	return h.successResult(status)
}

func (h *Handler) handleGetConnectionHistory(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	limit := getInt(args, "limit", 20)

	history, err := h.store.State.GetTransitionHistory(ctx, limit)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(history)
}

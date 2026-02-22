package api

import (
	"context"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Status tool handlers

func (h *Handler) handlePostTextStatus(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	text := getString(args, "text")
	if text == "" {
		return h.errorResult(NewInvalidInputError("text is required"))
	}

	backgroundColor := getString(args, "background_color")

	if err := h.bridge.PostTextStatus(ctx, text, backgroundColor); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Status posted",
	})
}

func (h *Handler) handlePostImageStatus(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	imagePath := getString(args, "image_path")
	if imagePath == "" {
		return h.errorResult(NewInvalidInputError("image_path is required"))
	}

	caption := getString(args, "caption")

	if err := h.bridge.PostImageStatus(ctx, imagePath, caption); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Status posted",
	})
}

func (h *Handler) handleGetStatusUpdates(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	contactJID := getString(args, "contact_jid")

	var statuses interface{}
	var err error

	if contactJID != "" {
		statuses, err = h.store.Status.GetByContact(ctx, contactJID)
	} else {
		statuses, err = h.store.Status.GetAll(ctx)
	}

	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(statuses)
}

func (h *Handler) handleDeleteStatus(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	statusID := getString(args, "status_id")
	if statusID == "" {
		return h.errorResult(NewInvalidInputError("status_id is required"))
	}

	if err := h.bridge.DeleteStatus(ctx, statusID); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Status deleted",
	})
}

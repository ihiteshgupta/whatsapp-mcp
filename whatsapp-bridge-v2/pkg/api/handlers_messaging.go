package api

import (
	"context"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Messaging tool handlers

func (h *Handler) handleSendMessage(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	recipient := getString(args, "recipient")
	if recipient == "" {
		return h.errorResult(NewInvalidInputError("recipient is required"))
	}

	message := getString(args, "message")
	if message == "" {
		return h.errorResult(NewInvalidInputError("message is required"))
	}

	msgID, err := h.bridge.SendMessage(ctx, recipient, message)
	if err != nil {
		return h.errorResult(NewMessageFailedError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":    true,
		"message_id": msgID,
	})
}

func (h *Handler) handleReplyToMessage(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	chatJID := getString(args, "chat_jid")
	if chatJID == "" {
		return h.errorResult(NewInvalidInputError("chat_jid is required"))
	}

	messageID := getString(args, "message_id")
	if messageID == "" {
		return h.errorResult(NewInvalidInputError("message_id is required"))
	}

	message := getString(args, "message")
	if message == "" {
		return h.errorResult(NewInvalidInputError("message is required"))
	}

	msgID, err := h.bridge.ReplyToMessage(ctx, chatJID, messageID, message)
	if err != nil {
		return h.errorResult(NewMessageFailedError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":    true,
		"message_id": msgID,
	})
}

func (h *Handler) handleForwardMessage(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	sourceChatJID := getString(args, "source_chat_jid")
	if sourceChatJID == "" {
		return h.errorResult(NewInvalidInputError("source_chat_jid is required"))
	}

	messageID := getString(args, "message_id")
	if messageID == "" {
		return h.errorResult(NewInvalidInputError("message_id is required"))
	}

	targetJID := getString(args, "target_jid")
	if targetJID == "" {
		return h.errorResult(NewInvalidInputError("target_jid is required"))
	}

	msgID, err := h.bridge.ForwardMessage(ctx, sourceChatJID, messageID, targetJID)
	if err != nil {
		return h.errorResult(NewMessageFailedError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":    true,
		"message_id": msgID,
	})
}

func (h *Handler) handleEditMessage(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	chatJID := getString(args, "chat_jid")
	if chatJID == "" {
		return h.errorResult(NewInvalidInputError("chat_jid is required"))
	}

	messageID := getString(args, "message_id")
	if messageID == "" {
		return h.errorResult(NewInvalidInputError("message_id is required"))
	}

	newContent := getString(args, "new_content")
	if newContent == "" {
		return h.errorResult(NewInvalidInputError("new_content is required"))
	}

	if err := h.bridge.EditMessage(ctx, chatJID, messageID, newContent); err != nil {
		return h.errorResult(NewMessageFailedError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Message edited",
	})
}

func (h *Handler) handleDeleteMessage(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	chatJID := getString(args, "chat_jid")
	if chatJID == "" {
		return h.errorResult(NewInvalidInputError("chat_jid is required"))
	}

	messageID := getString(args, "message_id")
	if messageID == "" {
		return h.errorResult(NewInvalidInputError("message_id is required"))
	}

	forEveryone := getBool(args, "for_everyone", false)

	if err := h.bridge.DeleteMessage(ctx, chatJID, messageID, forEveryone); err != nil {
		return h.errorResult(NewMessageFailedError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Message deleted",
	})
}

func (h *Handler) handleReactToMessage(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	chatJID := getString(args, "chat_jid")
	if chatJID == "" {
		return h.errorResult(NewInvalidInputError("chat_jid is required"))
	}

	messageID := getString(args, "message_id")
	if messageID == "" {
		return h.errorResult(NewInvalidInputError("message_id is required"))
	}

	emoji := getString(args, "emoji")
	if emoji == "" {
		return h.errorResult(NewInvalidInputError("emoji is required"))
	}

	if err := h.bridge.ReactToMessage(ctx, chatJID, messageID, emoji); err != nil {
		return h.errorResult(NewMessageFailedError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Reaction added",
	})
}

func (h *Handler) handleStarMessage(ctx context.Context, args map[string]interface{}, star bool) (*mcp.CallToolResult, error) {
	chatJID := getString(args, "chat_jid")
	if chatJID == "" {
		return h.errorResult(NewInvalidInputError("chat_jid is required"))
	}

	messageID := getString(args, "message_id")
	if messageID == "" {
		return h.errorResult(NewInvalidInputError("message_id is required"))
	}

	// Star/unstar via store (local operation)
	if err := h.store.Messages.SetStarred(ctx, chatJID, messageID, star); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	action := "starred"
	if !star {
		action = "unstarred"
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Message " + action,
	})
}

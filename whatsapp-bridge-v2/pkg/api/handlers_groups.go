package api

import (
	"context"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Group tool handlers

func (h *Handler) handleCreateGroup(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	name := getString(args, "name")
	if name == "" {
		return h.errorResult(NewInvalidInputError("name is required"))
	}

	participants := getStringArray(args, "participants")
	if len(participants) == 0 {
		return h.errorResult(NewInvalidInputError("participants is required"))
	}

	groupJID, err := h.bridge.CreateGroup(ctx, name, participants)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":   true,
		"group_jid": groupJID,
	})
}

func (h *Handler) handleGetGroupInfo(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	info, err := h.bridge.GetGroupInfo(ctx, jid)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(info)
}

func (h *Handler) handleLeaveGroup(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	if err := h.bridge.LeaveGroup(ctx, jid); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Left group",
	})
}

func (h *Handler) handleAddGroupMembers(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	groupJID := getString(args, "group_jid")
	if groupJID == "" {
		return h.errorResult(NewInvalidInputError("group_jid is required"))
	}

	participants := getStringArray(args, "participants")
	if len(participants) == 0 {
		return h.errorResult(NewInvalidInputError("participants is required"))
	}

	if err := h.bridge.AddGroupMembers(ctx, groupJID, participants); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Members added",
	})
}

func (h *Handler) handleRemoveGroupMembers(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	groupJID := getString(args, "group_jid")
	if groupJID == "" {
		return h.errorResult(NewInvalidInputError("group_jid is required"))
	}

	participants := getStringArray(args, "participants")
	if len(participants) == 0 {
		return h.errorResult(NewInvalidInputError("participants is required"))
	}

	if err := h.bridge.RemoveGroupMembers(ctx, groupJID, participants); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Members removed",
	})
}

func (h *Handler) handlePromoteAdmin(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	groupJID := getString(args, "group_jid")
	if groupJID == "" {
		return h.errorResult(NewInvalidInputError("group_jid is required"))
	}

	participants := getStringArray(args, "participants")
	if len(participants) == 0 {
		return h.errorResult(NewInvalidInputError("participants is required"))
	}

	if err := h.bridge.PromoteAdmin(ctx, groupJID, participants); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Members promoted to admin",
	})
}

func (h *Handler) handleDemoteAdmin(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	groupJID := getString(args, "group_jid")
	if groupJID == "" {
		return h.errorResult(NewInvalidInputError("group_jid is required"))
	}

	participants := getStringArray(args, "participants")
	if len(participants) == 0 {
		return h.errorResult(NewInvalidInputError("participants is required"))
	}

	if err := h.bridge.DemoteAdmin(ctx, groupJID, participants); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Admins demoted",
	})
}

func (h *Handler) handleSetGroupName(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	groupJID := getString(args, "group_jid")
	if groupJID == "" {
		return h.errorResult(NewInvalidInputError("group_jid is required"))
	}

	name := getString(args, "name")
	if name == "" {
		return h.errorResult(NewInvalidInputError("name is required"))
	}

	if err := h.bridge.SetGroupName(ctx, groupJID, name); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Group name updated",
	})
}

func (h *Handler) handleSetGroupTopic(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	groupJID := getString(args, "group_jid")
	if groupJID == "" {
		return h.errorResult(NewInvalidInputError("group_jid is required"))
	}

	topic := getString(args, "topic")
	if topic == "" {
		return h.errorResult(NewInvalidInputError("topic is required"))
	}

	if err := h.bridge.SetGroupTopic(ctx, groupJID, topic); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Group topic updated",
	})
}

func (h *Handler) handleSetGroupPhoto(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	groupJID := getString(args, "group_jid")
	if groupJID == "" {
		return h.errorResult(NewInvalidInputError("group_jid is required"))
	}

	imagePath := getString(args, "image_path")
	if imagePath == "" {
		return h.errorResult(NewInvalidInputError("image_path is required"))
	}

	if err := h.bridge.SetGroupPhoto(ctx, groupJID, imagePath); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Group photo updated",
	})
}

func (h *Handler) handleGetInviteLink(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	groupJID := getString(args, "group_jid")
	if groupJID == "" {
		return h.errorResult(NewInvalidInputError("group_jid is required"))
	}

	link, err := h.bridge.GetInviteLink(ctx, groupJID)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"invite_link": link,
	})
}

func (h *Handler) handleRevokeInviteLink(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	groupJID := getString(args, "group_jid")
	if groupJID == "" {
		return h.errorResult(NewInvalidInputError("group_jid is required"))
	}

	newLink, err := h.bridge.RevokeInviteLink(ctx, groupJID)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":         true,
		"new_invite_link": newLink,
	})
}

func (h *Handler) handleJoinViaInvite(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	inviteLink := getString(args, "invite_link")
	if inviteLink == "" {
		return h.errorResult(NewInvalidInputError("invite_link is required"))
	}

	groupJID, err := h.bridge.JoinViaInvite(ctx, inviteLink)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":   true,
		"group_jid": groupJID,
	})
}

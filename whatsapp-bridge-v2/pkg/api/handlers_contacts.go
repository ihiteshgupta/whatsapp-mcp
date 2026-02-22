package api

import (
	"context"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/store"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Contact tool handlers

func (h *Handler) handleSearchContacts(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	query := getString(args, "query")
	if query == "" {
		return h.errorResult(NewInvalidInputError("query is required"))
	}

	limit := getInt(args, "limit", 20)

	contacts, err := h.store.Contacts.Search(ctx, query, limit)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(contacts)
}

func (h *Handler) handleGetContact(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	contact, err := h.store.Contacts.GetByJID(ctx, jid)
	if err == store.ErrNotFound {
		return h.errorResult(NewNotFoundError("contact"))
	}
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(contact)
}

func (h *Handler) handleBlockContact(ctx context.Context, args map[string]interface{}, block bool) (*mcp.CallToolResult, error) {
	jid := getString(args, "jid")
	if jid == "" {
		return h.errorResult(NewInvalidInputError("jid is required"))
	}

	if err := h.bridge.BlockContact(ctx, jid, block); err != nil {
		return h.errorResult(NewInternalError(err))
	}

	action := "blocked"
	if !block {
		action = "unblocked"
	}

	return h.successResult(map[string]interface{}{
		"success": true,
		"message": "Contact " + action,
	})
}

func (h *Handler) handleGetBlockedContacts(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	contacts, err := h.store.Contacts.GetBlocked(ctx)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(contacts)
}

func (h *Handler) handleCheckPhoneRegistered(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	phone := getString(args, "phone")
	if phone == "" {
		return h.errorResult(NewInvalidInputError("phone is required"))
	}

	registered, err := h.bridge.CheckPhoneRegistered(ctx, phone)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"phone":      phone,
		"registered": registered,
	})
}

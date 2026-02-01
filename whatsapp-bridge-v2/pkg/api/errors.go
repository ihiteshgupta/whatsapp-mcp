// Package api provides the MCP server for Claude Code integration.
package api

import (
	"encoding/json"
	"fmt"
)

// Error codes
const (
	ErrNotReady       = "NOT_READY"
	ErrInvalidJID     = "INVALID_JID"
	ErrMessageFailed  = "MESSAGE_FAILED"
	ErrMediaFailed    = "MEDIA_FAILED"
	ErrNotFound       = "NOT_FOUND"
	ErrRateLimited    = "RATE_LIMITED"
	ErrSessionExpired = "SESSION_EXPIRED"
	ErrInvalidInput   = "INVALID_INPUT"
	ErrInternal       = "INTERNAL_ERROR"
)

// MCPError represents a structured error for MCP responses.
type MCPError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Retry   bool   `json:"retry"`
}

// Error implements the error interface.
func (e *MCPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// JSON returns the error as a JSON string.
func (e *MCPError) JSON() string {
	data, _ := json.Marshal(e)
	return string(data)
}

// NewNotReadyError creates an error for when the bridge is not ready.
func NewNotReadyError(state string) *MCPError {
	return &MCPError{
		Code:    ErrNotReady,
		Message: fmt.Sprintf("Bridge not ready, current state: %s", state),
		Retry:   true,
	}
}

// NewInvalidJIDError creates an error for invalid JID.
func NewInvalidJIDError(jid string) *MCPError {
	return &MCPError{
		Code:    ErrInvalidJID,
		Message: fmt.Sprintf("Invalid JID format: %s", jid),
		Retry:   false,
	}
}

// NewMessageFailedError creates an error for failed message sending.
func NewMessageFailedError(err error) *MCPError {
	return &MCPError{
		Code:    ErrMessageFailed,
		Message: fmt.Sprintf("Failed to send message: %s", err.Error()),
		Retry:   true,
	}
}

// NewNotFoundError creates an error for not found resources.
func NewNotFoundError(resource string) *MCPError {
	return &MCPError{
		Code:    ErrNotFound,
		Message: fmt.Sprintf("Resource not found: %s", resource),
		Retry:   false,
	}
}

// NewInvalidInputError creates an error for invalid input.
func NewInvalidInputError(message string) *MCPError {
	return &MCPError{
		Code:    ErrInvalidInput,
		Message: message,
		Retry:   false,
	}
}

// NewInternalError creates an error for internal errors.
func NewInternalError(err error) *MCPError {
	return &MCPError{
		Code:    ErrInternal,
		Message: fmt.Sprintf("Internal error: %s", err.Error()),
		Retry:   false,
	}
}

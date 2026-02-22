package api

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Media tool handlers

func (h *Handler) handleSendImage(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	recipient := getString(args, "recipient")
	if recipient == "" {
		return h.errorResult(NewInvalidInputError("recipient is required"))
	}

	imagePath := getString(args, "image_path")
	if imagePath == "" {
		return h.errorResult(NewInvalidInputError("image_path is required"))
	}

	caption := getString(args, "caption")

	msgID, err := h.bridge.SendImage(ctx, recipient, imagePath, caption)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":    true,
		"message_id": msgID,
	})
}

func (h *Handler) handleSendVideo(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	recipient := getString(args, "recipient")
	if recipient == "" {
		return h.errorResult(NewInvalidInputError("recipient is required"))
	}

	videoPath := getString(args, "video_path")
	if videoPath == "" {
		return h.errorResult(NewInvalidInputError("video_path is required"))
	}

	caption := getString(args, "caption")

	msgID, err := h.bridge.SendVideo(ctx, recipient, videoPath, caption)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":    true,
		"message_id": msgID,
	})
}

func (h *Handler) handleSendAudio(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	recipient := getString(args, "recipient")
	if recipient == "" {
		return h.errorResult(NewInvalidInputError("recipient is required"))
	}

	audioPath := getString(args, "audio_path")
	if audioPath == "" {
		return h.errorResult(NewInvalidInputError("audio_path is required"))
	}

	asVoice := getBool(args, "as_voice", false)

	msgID, err := h.bridge.SendAudio(ctx, recipient, audioPath, asVoice)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":    true,
		"message_id": msgID,
	})
}

func (h *Handler) handleSendDocument(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	recipient := getString(args, "recipient")
	if recipient == "" {
		return h.errorResult(NewInvalidInputError("recipient is required"))
	}

	filePath := getString(args, "file_path")
	if filePath == "" {
		return h.errorResult(NewInvalidInputError("file_path is required"))
	}

	filename := getString(args, "filename")

	msgID, err := h.bridge.SendDocument(ctx, recipient, filePath, filename)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":    true,
		"message_id": msgID,
	})
}

func (h *Handler) handleSendLocation(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	recipient := getString(args, "recipient")
	if recipient == "" {
		return h.errorResult(NewInvalidInputError("recipient is required"))
	}

	latitude := getFloat(args, "latitude")
	longitude := getFloat(args, "longitude")

	if latitude < -90 || latitude > 90 {
		return h.errorResult(NewInvalidInputError("latitude must be between -90 and 90"))
	}

	if longitude < -180 || longitude > 180 {
		return h.errorResult(NewInvalidInputError("longitude must be between -180 and 180"))
	}

	name := getString(args, "name")
	address := getString(args, "address")

	msgID, err := h.bridge.SendLocation(ctx, recipient, latitude, longitude, name, address)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":    true,
		"message_id": msgID,
	})
}

func (h *Handler) handleSendContactCard(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	recipient := getString(args, "recipient")
	if recipient == "" {
		return h.errorResult(NewInvalidInputError("recipient is required"))
	}

	contactJID := getString(args, "contact_jid")
	if contactJID == "" {
		return h.errorResult(NewInvalidInputError("contact_jid is required"))
	}

	msgID, err := h.bridge.SendContactCard(ctx, recipient, contactJID)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":    true,
		"message_id": msgID,
	})
}

func (h *Handler) handleDownloadMedia(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	chatJID := getString(args, "chat_jid")
	if chatJID == "" {
		return h.errorResult(NewInvalidInputError("chat_jid is required"))
	}

	messageID := getString(args, "message_id")
	if messageID == "" {
		return h.errorResult(NewInvalidInputError("message_id is required"))
	}

	savePath := getString(args, "save_path")
	if err := validateSavePath(savePath); err != nil {
		return h.errorResult(NewInvalidInputError(err.Error()))
	}

	filePath, err := h.bridge.DownloadMedia(ctx, chatJID, messageID, savePath)
	if err != nil {
		return h.errorResult(NewInternalError(err))
	}

	return h.successResult(map[string]interface{}{
		"success":   true,
		"file_path": filePath,
	})
}

func validateSavePath(path string) error {
	cleanPath := filepath.Clean(path)

	if cleanPath == "." {
		return errors.New("save_path is required")
	}

	if strings.Contains(cleanPath, "..") {
		return errors.New("save_path must not contain \"..\"")
	}

	forbiddenPrefixes := []string{
		"/etc",
		"/proc",
		"/sys",
		"/bin",
		"/sbin",
		"/usr",
		"/boot",
		"/dev",
		"/lib",
		"/lib64",
		"/root",
		"~/.ssh",
	}
	for _, prefix := range forbiddenPrefixes {
		if cleanPath == prefix || strings.HasPrefix(cleanPath, prefix+string(filepath.Separator)) {
			return errors.New("save_path is not allowed")
		}
	}

	return nil
}

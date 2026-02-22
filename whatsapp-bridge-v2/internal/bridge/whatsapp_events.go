package bridge

import (
	"context"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/store"
)

// registerWhatsAppEventHandler registers the bridge as an event handler on the WhatsApp client.
// This must be called after Connect() so that incoming messages, history syncs, and contact
// updates are persisted to messages.db and available to MCP tools.
func (b *Bridge) registerWhatsAppEventHandler() {
	b.client.AddEventHandler(b.handleWhatsAppEvent)
}

// handleWhatsAppEvent processes raw whatsmeow events and persists relevant data to the store.
func (b *Bridge) handleWhatsAppEvent(rawEvt interface{}) {
	ctx := context.Background()
	switch evt := rawEvt.(type) {
	case *events.Message:
		b.persistMessage(ctx, evt)
	case *events.HistorySync:
		b.persistHistorySync(ctx, evt)
	}
}

// persistMessage stores a new incoming/outgoing message and updates the chat record.
func (b *Bridge) persistMessage(ctx context.Context, evt *events.Message) {
	chatJID := evt.Info.Chat.String()
	content := extractMessageText(evt.Message)
	sender := evt.Info.Sender.String()
	if evt.Info.IsFromMe {
		sender = "me"
	}

	// Upsert the chat so it appears in list_chats
	chat := &store.Chat{
		JID:             chatJID,
		IsGroup:         evt.Info.IsGroup,
		LastMessageTime: evt.Info.Timestamp,
	}
	if err := b.store.Chats.Upsert(ctx, chat); err != nil {
		b.log.Error("failed to upsert chat on message", "error", err, "jid", chatJID)
	}
	if err := b.store.Chats.UpdateLastMessage(ctx, chatJID, evt.Info.Timestamp); err != nil {
		b.log.Debug("failed to update last message time", "error", err, "jid", chatJID)
	}

	// Store the message
	msg := &store.Message{
		ID:        evt.Info.ID,
		ChatJID:   chatJID,
		Sender:    sender,
		Content:   content,
		Timestamp: evt.Info.Timestamp,
		IsFromMe:  evt.Info.IsFromMe,
	}
	if err := b.store.Messages.Store(ctx, msg); err != nil {
		b.log.Debug("failed to store message", "error", err, "id", evt.Info.ID)
	}
}

// persistHistorySync processes a WhatsApp history sync batch and stores chats + messages.
func (b *Bridge) persistHistorySync(ctx context.Context, evt *events.HistorySync) {
	convs := evt.Data.GetConversations()
	b.log.Info("processing history sync", "type", evt.Data.GetSyncType().String(), "conversations", len(convs))

	for _, conv := range convs {
		jid := conv.GetID()
		if jid == "" {
			continue
		}

		name := conv.GetName()
		isGroup := strings.HasSuffix(jid, "@g.us") || strings.HasSuffix(jid, "@broadcast")

		// Determine last message time from the conversation timestamp or messages
		var lastMsgTime time.Time
		if ts := conv.GetConversationTimestamp(); ts > 0 {
			lastMsgTime = time.Unix(int64(ts), 0)
		}

		unread := int(conv.GetUnreadCount())
		archived := conv.GetArchived()
		pinned := conv.GetPinned() > 0
		muteEnd := conv.GetMuteEndTime()
		muted := muteEnd > 0

		var mutedUntil *time.Time
		if muted {
			t := time.Unix(int64(muteEnd), 0)
			mutedUntil = &t
		}

		chat := &store.Chat{
			JID:             jid,
			Name:            name,
			IsGroup:         isGroup,
			LastMessageTime: lastMsgTime,
			UnreadCount:     unread,
			Archived:        archived,
			Pinned:          pinned,
			Muted:           muted,
			MutedUntil:      mutedUntil,
		}
		if err := b.store.Chats.Upsert(ctx, chat); err != nil {
			b.log.Error("failed to upsert chat from history", "error", err, "jid", jid)
			continue
		}

		// Store messages from history
		for _, histMsg := range conv.GetMessages() {
			webMsg := histMsg.GetMessage()
			if webMsg == nil {
				continue
			}
			key := webMsg.GetKey()
			if key == nil {
				continue
			}
			msgID := key.GetID()
			if msgID == "" {
				continue
			}

			ts := time.Unix(int64(webMsg.GetMessageTimestamp()), 0)
			fromMe := key.GetFromMe()
			participant := key.GetParticipant()
			sender := participant
			if fromMe {
				sender = "me"
			}

			content := extractMessageText(webMsg.GetMessage())

			msg := &store.Message{
				ID:        msgID,
				ChatJID:   jid,
				Sender:    sender,
				Content:   content,
				Timestamp: ts,
				IsFromMe:  fromMe,
			}
			if err := b.store.Messages.Store(ctx, msg); err != nil {
				// Duplicate key errors are expected; log at debug only
				b.log.Debug("failed to store history message", "error", err, "id", msgID)
			}
		}
	}
}

// extractMessageText pulls the plain-text content out of a WhatsApp message.
func extractMessageText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Conversation != nil {
		return *msg.Conversation
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}
	if img := msg.GetImageMessage(); img != nil {
		return img.GetCaption()
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return vid.GetCaption()
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return doc.GetTitle()
	}
	if audio := msg.GetAudioMessage(); audio != nil {
		return "[audio]"
	}
	if msg.GetStickerMessage() != nil {
		return "[sticker]"
	}
	if loc := msg.GetLocationMessage(); loc != nil {
		return "[location]"
	}
	if contact := msg.GetContactMessage(); contact != nil {
		return "[contact: " + contact.GetDisplayName() + "]"
	}
	if msg.GetReactionMessage() != nil {
		return "[reaction]"
	}
	return ""
}

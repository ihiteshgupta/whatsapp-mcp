package main

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"

	"bytes"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// Message represents a chat message for our client
type Message struct {
	Time      time.Time
	Sender    string
	Content   string
	IsFromMe  bool
	MediaType string
	Filename  string
}

// Database handler for storing message history
type MessageStore struct {
	db *sql.DB
}

// Initialize message store
func NewMessageStore() (*MessageStore, error) {
	// Create directory for database if it doesn't exist
	if err := os.MkdirAll("store", 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %v", err)
	}

	// Open SQLite database for messages
	db, err := sql.Open("sqlite3", "file:store/messages.db?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open message database: %v", err)
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS chats (
			jid TEXT PRIMARY KEY,
			name TEXT,
			last_message_time TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT,
			chat_jid TEXT,
			sender TEXT,
			content TEXT,
			timestamp TIMESTAMP,
			is_from_me BOOLEAN,
			media_type TEXT,
			filename TEXT,
			url TEXT,
			media_key BLOB,
			file_sha256 BLOB,
			file_enc_sha256 BLOB,
			file_length INTEGER,
			PRIMARY KEY (id, chat_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return &MessageStore{db: db}, nil
}

// Close the database connection
func (store *MessageStore) Close() error {
	return store.db.Close()
}

// Store a chat in the database
func (store *MessageStore) StoreChat(jid, name string, lastMessageTime time.Time) error {
	_, err := store.db.Exec(
		"INSERT OR REPLACE INTO chats (jid, name, last_message_time) VALUES (?, ?, ?)",
		jid, name, lastMessageTime,
	)
	return err
}

// Store a message in the database
func (store *MessageStore) StoreMessage(id, chatJID, sender, content string, timestamp time.Time, isFromMe bool,
	mediaType, filename, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) error {
	// Only store if there's actual content or media
	if content == "" && mediaType == "" {
		return nil
	}

	_, err := store.db.Exec(
		`INSERT OR REPLACE INTO messages 
		(id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, chatJID, sender, content, timestamp, isFromMe, mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength,
	)
	return err
}

// Get messages from a chat
func (store *MessageStore) GetMessages(chatJID string, limit int) ([]Message, error) {
	rows, err := store.db.Query(
		"SELECT sender, content, timestamp, is_from_me, media_type, filename FROM messages WHERE chat_jid = ? ORDER BY timestamp DESC LIMIT ?",
		chatJID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var timestamp time.Time
		err := rows.Scan(&msg.Sender, &msg.Content, &timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename)
		if err != nil {
			return nil, err
		}
		msg.Time = timestamp
		messages = append(messages, msg)
	}

	return messages, nil
}

// Get all chats
func (store *MessageStore) GetChats() (map[string]time.Time, error) {
	rows, err := store.db.Query("SELECT jid, last_message_time FROM chats ORDER BY last_message_time DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make(map[string]time.Time)
	for rows.Next() {
		var jid string
		var lastMessageTime time.Time
		err := rows.Scan(&jid, &lastMessageTime)
		if err != nil {
			return nil, err
		}
		chats[jid] = lastMessageTime
	}

	return chats, nil
}

// Extract text content from a message
func extractTextContent(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}

	// Try to get text content
	if text := msg.GetConversation(); text != "" {
		return text
	} else if extendedText := msg.GetExtendedTextMessage(); extendedText != nil {
		return extendedText.GetText()
	}

	// For now, we're ignoring non-text messages
	return ""
}

// SendMessageResponse represents the response for the send message API
type SendMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SendMessageRequest represents the request body for the send message API
type SendMessageRequest struct {
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
	MediaPath string `json:"media_path,omitempty"`
}

// ============== NEW REQUEST/RESPONSE TYPES ==============

// GenericResponse for simple success/failure responses
type GenericResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ChatRequest for chat-related operations
type ChatRequest struct {
	ChatJID string `json:"chat_jid"`
}

// MuteChatRequest for muting chats with duration
type MuteChatRequest struct {
	ChatJID  string `json:"chat_jid"`
	Duration int64  `json:"duration"` // Duration in seconds, 0 for indefinite, -1 to unmute
}

// ReactRequest for message reactions
type ReactRequest struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	Sender    string `json:"sender"`
	Emoji     string `json:"emoji"` // Empty string to remove reaction
}

// ReplyRequest for replying to messages
type ReplyRequest struct {
	ChatJID         string `json:"chat_jid"`
	MessageID       string `json:"message_id"`
	QuotedSender    string `json:"quoted_sender"`
	Message         string `json:"message"`
	MediaPath       string `json:"media_path,omitempty"`
}

// ForwardRequest for forwarding messages
type ForwardRequest struct {
	FromChatJID string `json:"from_chat_jid"`
	ToChatJID   string `json:"to_chat_jid"`
	MessageID   string `json:"message_id"`
}

// DeleteMessageRequest for deleting messages
type DeleteMessageRequest struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	Sender    string `json:"sender"`
	ForAll    bool   `json:"for_all"` // true to delete for everyone
}

// StarMessageRequest for starring messages
type StarMessageRequest struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	Star      bool   `json:"star"` // true to star, false to unstar
}

// EditMessageRequest for editing sent messages
type EditMessageRequest struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	NewText   string `json:"new_text"`
}

// CreateGroupRequest for creating groups
type CreateGroupRequest struct {
	Name         string   `json:"name"`
	Participants []string `json:"participants"` // List of JIDs
}

// CreateGroupResponse contains the new group info
type CreateGroupResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	GroupJID string `json:"group_jid,omitempty"`
}

// GroupMembersRequest for adding/removing group members
type GroupMembersRequest struct {
	GroupJID     string   `json:"group_jid"`
	Participants []string `json:"participants"` // List of JIDs
}

// GroupAdminRequest for promoting/demoting admins
type GroupAdminRequest struct {
	GroupJID    string   `json:"group_jid"`
	Participants []string `json:"participants"` // List of JIDs
}

// GroupInfoRequest for getting/updating group info
type GroupInfoRequest struct {
	GroupJID string `json:"group_jid"`
}

// GroupInfoResponse contains group details
type GroupInfoResponse struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	JID          string   `json:"jid,omitempty"`
	Name         string   `json:"name,omitempty"`
	Topic        string   `json:"topic,omitempty"`
	Created      string   `json:"created,omitempty"`
	Creator      string   `json:"creator,omitempty"`
	Participants []string `json:"participants,omitempty"`
	Admins       []string `json:"admins,omitempty"`
}

// GroupNameRequest for updating group name
type GroupNameRequest struct {
	GroupJID string `json:"group_jid"`
	Name     string `json:"name"`
}

// GroupTopicRequest for updating group description
type GroupTopicRequest struct {
	GroupJID string `json:"group_jid"`
	Topic    string `json:"topic"`
}

// GroupPhotoRequest for updating group photo
type GroupPhotoRequest struct {
	GroupJID  string `json:"group_jid"`
	PhotoPath string `json:"photo_path"`
}

// GroupInviteResponse contains invite link
type GroupInviteResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	InviteLink string `json:"invite_link,omitempty"`
	InviteCode string `json:"invite_code,omitempty"`
}

// JoinGroupRequest for joining via invite link
type JoinGroupRequest struct {
	InviteLink string `json:"invite_link"`
}

// ProfileResponse contains profile info
type ProfileResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	JID         string `json:"jid,omitempty"`
	Name        string `json:"name,omitempty"`
	About       string `json:"about,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

// UpdateProfileRequest for updating profile
type UpdateProfileRequest struct {
	Name  string `json:"name,omitempty"`
	About string `json:"about,omitempty"`
}

// ProfilePhotoRequest for updating profile photo
type ProfilePhotoRequest struct {
	PhotoPath string `json:"photo_path"`
}

// PrivacySettingsResponse contains privacy settings
type PrivacySettingsResponse struct {
	Success          bool   `json:"success"`
	Message          string `json:"message"`
	LastSeen         string `json:"last_seen,omitempty"`
	ProfilePhoto     string `json:"profile_photo,omitempty"`
	About            string `json:"about,omitempty"`
	ReadReceipts     string `json:"read_receipts,omitempty"`
	Groups           string `json:"groups,omitempty"`
	Status           string `json:"status,omitempty"`
}

// UpdatePrivacyRequest for updating privacy settings
type UpdatePrivacyRequest struct {
	Setting string `json:"setting"` // last_seen, profile_photo, about, read_receipts, groups, status
	Value   string `json:"value"`   // all, contacts, contact_blacklist, none
}

// BlockContactRequest for blocking/unblocking contacts
type BlockContactRequest struct {
	JID string `json:"jid"`
}

// BlockedContactsResponse contains list of blocked contacts
type BlockedContactsResponse struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message"`
	Contacts []string `json:"contacts,omitempty"`
}

// PresenceRequest for presence operations
type PresenceRequest struct {
	JID string `json:"jid"`
}

// TypingRequest for typing indicators
type TypingRequest struct {
	ChatJID string `json:"chat_jid"`
	Typing  bool   `json:"typing"` // true for typing, false for stopped
}

// RecordingRequest for recording indicators
type RecordingRequest struct {
	ChatJID   string `json:"chat_jid"`
	Recording bool   `json:"recording"`
}

// StatusTextRequest for posting text status
type StatusTextRequest struct {
	Text            string `json:"text"`
	BackgroundColor string `json:"background_color,omitempty"` // Hex color
	Font            int    `json:"font,omitempty"`             // Font style
}

// StatusMediaRequest for posting media status
type StatusMediaRequest struct {
	MediaPath string `json:"media_path"`
	Caption   string `json:"caption,omitempty"`
}

// DeleteStatusRequest for deleting status
type DeleteStatusRequest struct {
	StatusID string `json:"status_id"`
}

// LocationRequest for sending location
type LocationRequest struct {
	ChatJID     string  `json:"chat_jid"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Name        string  `json:"name,omitempty"`
	Address     string  `json:"address,omitempty"`
}

// ContactCardRequest for sending contact cards
type ContactCardRequest struct {
	ChatJID     string `json:"chat_jid"`
	ContactJID  string `json:"contact_jid"`
	DisplayName string `json:"display_name"`
	PhoneNumber string `json:"phone_number"`
}

// CheckPhoneRequest for checking if phone is on WhatsApp
type CheckPhoneRequest struct {
	PhoneNumbers []string `json:"phone_numbers"`
}

// CheckPhoneResponse contains registration status
type CheckPhoneResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Results map[string]bool   `json:"results,omitempty"` // phone -> is_registered
	JIDs    map[string]string `json:"jids,omitempty"`    // phone -> jid
}

// ContactInfoRequest for getting contact info
type ContactInfoRequest struct {
	JID string `json:"jid"`
}

// ContactInfoResponse contains contact details
type ContactInfoResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	JID         string `json:"jid,omitempty"`
	Name        string `json:"name,omitempty"`
	FullName    string `json:"full_name,omitempty"`
	PushName    string `json:"push_name,omitempty"`
	BusinessName string `json:"business_name,omitempty"`
}

// MarkReadRequest for marking messages as read
type MarkReadRequest struct {
	ChatJID    string   `json:"chat_jid"`
	MessageIDs []string `json:"message_ids"`
	Sender     string   `json:"sender"`
}

// Function to send a WhatsApp message
func sendWhatsAppMessage(client *whatsmeow.Client, recipient string, message string, mediaPath string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	// Create JID for recipient
	var recipientJID types.JID
	var err error

	// Check if recipient is a JID
	isJID := strings.Contains(recipient, "@")

	if isJID {
		// Parse the JID string
		recipientJID, err = types.ParseJID(recipient)
		if err != nil {
			return false, fmt.Sprintf("Error parsing JID: %v", err)
		}
	} else {
		// Create JID from phone number
		recipientJID = types.JID{
			User:   recipient,
			Server: "s.whatsapp.net", // For personal chats
		}
	}

	msg := &waProto.Message{}

	// Check if we have media to send
	if mediaPath != "" {
		// Read media file
		mediaData, err := os.ReadFile(mediaPath)
		if err != nil {
			return false, fmt.Sprintf("Error reading media file: %v", err)
		}

		// Determine media type and mime type based on file extension
		fileExt := strings.ToLower(mediaPath[strings.LastIndex(mediaPath, ".")+1:])
		var mediaType whatsmeow.MediaType
		var mimeType string

		// Handle different media types
		switch fileExt {
		// Image types
		case "jpg", "jpeg":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/jpeg"
		case "png":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/png"
		case "gif":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/gif"
		case "webp":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/webp"

		// Audio types
		case "ogg":
			mediaType = whatsmeow.MediaAudio
			mimeType = "audio/ogg; codecs=opus"

		// Video types
		case "mp4":
			mediaType = whatsmeow.MediaVideo
			mimeType = "video/mp4"
		case "avi":
			mediaType = whatsmeow.MediaVideo
			mimeType = "video/avi"
		case "mov":
			mediaType = whatsmeow.MediaVideo
			mimeType = "video/quicktime"

		// Document types (for any other file type)
		default:
			mediaType = whatsmeow.MediaDocument
			mimeType = "application/octet-stream"
		}

		// Upload media to WhatsApp servers
		resp, err := client.Upload(context.Background(), mediaData, mediaType)
		if err != nil {
			return false, fmt.Sprintf("Error uploading media: %v", err)
		}

		fmt.Println("Media uploaded", resp)

		// Create the appropriate message type based on media type
		switch mediaType {
		case whatsmeow.MediaImage:
			msg.ImageMessage = &waProto.ImageMessage{
				Caption:       proto.String(message),
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			}
		case whatsmeow.MediaAudio:
			// Handle ogg audio files
			var seconds uint32 = 30 // Default fallback
			var waveform []byte = nil

			// Try to analyze the ogg file
			if strings.Contains(mimeType, "ogg") {
				analyzedSeconds, analyzedWaveform, err := analyzeOggOpus(mediaData)
				if err == nil {
					seconds = analyzedSeconds
					waveform = analyzedWaveform
				} else {
					return false, fmt.Sprintf("Failed to analyze Ogg Opus file: %v", err)
				}
			} else {
				fmt.Printf("Not an Ogg Opus file: %s\n", mimeType)
			}

			msg.AudioMessage = &waProto.AudioMessage{
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
				Seconds:       proto.Uint32(seconds),
				PTT:           proto.Bool(true),
				Waveform:      waveform,
			}
		case whatsmeow.MediaVideo:
			msg.VideoMessage = &waProto.VideoMessage{
				Caption:       proto.String(message),
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			}
		case whatsmeow.MediaDocument:
			msg.DocumentMessage = &waProto.DocumentMessage{
				Title:         proto.String(mediaPath[strings.LastIndex(mediaPath, "/")+1:]),
				Caption:       proto.String(message),
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			}
		}
	} else {
		msg.Conversation = proto.String(message)
	}

	// Send message
	_, err = client.SendMessage(context.Background(), recipientJID, msg)

	if err != nil {
		return false, fmt.Sprintf("Error sending message: %v", err)
	}

	return true, fmt.Sprintf("Message sent to %s", recipient)
}

// Extract media info from a message
func extractMediaInfo(msg *waProto.Message) (mediaType string, filename string, url string, mediaKey []byte, fileSHA256 []byte, fileEncSHA256 []byte, fileLength uint64) {
	if msg == nil {
		return "", "", "", nil, nil, nil, 0
	}

	// Check for image message
	if img := msg.GetImageMessage(); img != nil {
		// Use SHA256 hash prefix for unique filename instead of timestamp
		hashPrefix := fmt.Sprintf("%x", img.GetFileSHA256())[:12]
		return "image", "image_" + hashPrefix + ".jpg",
			img.GetURL(), img.GetMediaKey(), img.GetFileSHA256(), img.GetFileEncSHA256(), img.GetFileLength()
	}

	// Check for video message
	if vid := msg.GetVideoMessage(); vid != nil {
		// Use SHA256 hash prefix for unique filename instead of timestamp
		hashPrefix := fmt.Sprintf("%x", vid.GetFileSHA256())[:12]
		return "video", "video_" + hashPrefix + ".mp4",
			vid.GetURL(), vid.GetMediaKey(), vid.GetFileSHA256(), vid.GetFileEncSHA256(), vid.GetFileLength()
	}

	// Check for audio message
	if aud := msg.GetAudioMessage(); aud != nil {
		// Use SHA256 hash prefix for unique filename instead of timestamp
		hashPrefix := fmt.Sprintf("%x", aud.GetFileSHA256())[:12]
		return "audio", "audio_" + hashPrefix + ".ogg",
			aud.GetURL(), aud.GetMediaKey(), aud.GetFileSHA256(), aud.GetFileEncSHA256(), aud.GetFileLength()
	}

	// Check for document message
	if doc := msg.GetDocumentMessage(); doc != nil {
		filename := doc.GetFileName()
		if filename == "" {
			filename = "document_" + time.Now().Format("20060102_150405")
		}
		return "document", filename,
			doc.GetURL(), doc.GetMediaKey(), doc.GetFileSHA256(), doc.GetFileEncSHA256(), doc.GetFileLength()
	}

	return "", "", "", nil, nil, nil, 0
}

// Handle regular incoming messages with media support
func handleMessage(client *whatsmeow.Client, messageStore *MessageStore, msg *events.Message, logger waLog.Logger) {
	// Save message to database
	chatJID := msg.Info.Chat.String()
	sender := msg.Info.Sender.User

	// Get appropriate chat name (pass nil for conversation since we don't have one for regular messages)
	name := GetChatName(client, messageStore, msg.Info.Chat, chatJID, nil, sender, logger)

	// Update chat in database with the message timestamp (keeps last message time updated)
	err := messageStore.StoreChat(chatJID, name, msg.Info.Timestamp)
	if err != nil {
		logger.Warnf("Failed to store chat: %v", err)
	}

	// Extract text content
	content := extractTextContent(msg.Message)

	// Extract media info
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg.Message)

	// Skip if there's no content and no media
	if content == "" && mediaType == "" {
		return
	}

	// Store message in database
	err = messageStore.StoreMessage(
		msg.Info.ID,
		chatJID,
		sender,
		content,
		msg.Info.Timestamp,
		msg.Info.IsFromMe,
		mediaType,
		filename,
		url,
		mediaKey,
		fileSHA256,
		fileEncSHA256,
		fileLength,
	)

	if err != nil {
		logger.Warnf("Failed to store message: %v", err)
	} else {
		// Log message reception
		timestamp := msg.Info.Timestamp.Format("2006-01-02 15:04:05")
		direction := "←"
		if msg.Info.IsFromMe {
			direction = "→"
		}

		// Log based on message type
		if mediaType != "" {
			fmt.Printf("[%s] %s %s: [%s: %s] %s\n", timestamp, direction, sender, mediaType, filename, content)
		} else if content != "" {
			fmt.Printf("[%s] %s %s: %s\n", timestamp, direction, sender, content)
		}
	}
}

// DownloadMediaRequest represents the request body for the download media API
type DownloadMediaRequest struct {
	MessageID string `json:"message_id"`
	ChatJID   string `json:"chat_jid"`
}

// DownloadMediaResponse represents the response for the download media API
type DownloadMediaResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
}

// Store additional media info in the database
func (store *MessageStore) StoreMediaInfo(id, chatJID, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) error {
	_, err := store.db.Exec(
		"UPDATE messages SET url = ?, media_key = ?, file_sha256 = ?, file_enc_sha256 = ?, file_length = ? WHERE id = ? AND chat_jid = ?",
		url, mediaKey, fileSHA256, fileEncSHA256, fileLength, id, chatJID,
	)
	return err
}

// Get media info from the database
func (store *MessageStore) GetMediaInfo(id, chatJID string) (string, string, string, []byte, []byte, []byte, uint64, error) {
	var mediaType, filename, url string
	var mediaKey, fileSHA256, fileEncSHA256 []byte
	var fileLength uint64

	err := store.db.QueryRow(
		"SELECT media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length FROM messages WHERE id = ? AND chat_jid = ?",
		id, chatJID,
	).Scan(&mediaType, &filename, &url, &mediaKey, &fileSHA256, &fileEncSHA256, &fileLength)

	return mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength, err
}

// MediaDownloader implements the whatsmeow.DownloadableMessage interface
type MediaDownloader struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileLength    uint64
	FileSHA256    []byte
	FileEncSHA256 []byte
	MediaType     whatsmeow.MediaType
}

// GetDirectPath implements the DownloadableMessage interface
func (d *MediaDownloader) GetDirectPath() string {
	return d.DirectPath
}

// GetURL implements the DownloadableMessage interface
func (d *MediaDownloader) GetURL() string {
	return d.URL
}

// GetMediaKey implements the DownloadableMessage interface
func (d *MediaDownloader) GetMediaKey() []byte {
	return d.MediaKey
}

// GetFileLength implements the DownloadableMessage interface
func (d *MediaDownloader) GetFileLength() uint64 {
	return d.FileLength
}

// GetFileSHA256 implements the DownloadableMessage interface
func (d *MediaDownloader) GetFileSHA256() []byte {
	return d.FileSHA256
}

// GetFileEncSHA256 implements the DownloadableMessage interface
func (d *MediaDownloader) GetFileEncSHA256() []byte {
	return d.FileEncSHA256
}

// GetMediaType implements the DownloadableMessage interface
func (d *MediaDownloader) GetMediaType() whatsmeow.MediaType {
	return d.MediaType
}

// Function to download media from a message
func downloadMedia(client *whatsmeow.Client, messageStore *MessageStore, messageID, chatJID string) (bool, string, string, string, error) {
	// Query the database for the message
	var mediaType, filename, url string
	var mediaKey, fileSHA256, fileEncSHA256 []byte
	var fileLength uint64
	var err error

	// First, check if we already have this file
	chatDir := fmt.Sprintf("store/%s", strings.ReplaceAll(chatJID, ":", "_"))
	localPath := ""

	// Get media info from the database
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength, err = messageStore.GetMediaInfo(messageID, chatJID)

	if err != nil {
		// Try to get basic info if extended info isn't available
		err = messageStore.db.QueryRow(
			"SELECT media_type, filename FROM messages WHERE id = ? AND chat_jid = ?",
			messageID, chatJID,
		).Scan(&mediaType, &filename)

		if err != nil {
			return false, "", "", "", fmt.Errorf("failed to find message: %v", err)
		}
	}

	// Check if this is a media message
	if mediaType == "" {
		return false, "", "", "", fmt.Errorf("not a media message")
	}

	// Create directory for the chat if it doesn't exist
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return false, "", "", "", fmt.Errorf("failed to create chat directory: %v", err)
	}

	// Generate a unique filename using message ID to avoid overwrites
	ext := ".bin"
	switch mediaType {
	case "image":
		ext = ".jpg"
	case "video":
		ext = ".mp4"
	case "audio":
		ext = ".ogg"
	case "document":
		// Keep original extension for documents
		if filename != "" {
			ext = filepath.Ext(filename)
			if ext == "" {
				ext = ".bin"
			}
		}
	}
	uniqueFilename := fmt.Sprintf("%s_%s%s", mediaType, messageID, ext)

	// Generate a local path for the file using unique filename
	localPath = fmt.Sprintf("%s/%s", chatDir, uniqueFilename)
	filename = uniqueFilename

	// Get absolute path
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return false, "", "", "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Check if file already exists
	if _, err := os.Stat(localPath); err == nil {
		// File exists, return it
		return true, mediaType, filename, absPath, nil
	}

	// If we don't have all the media info we need, we can't download
	if url == "" || len(mediaKey) == 0 || len(fileSHA256) == 0 || len(fileEncSHA256) == 0 || fileLength == 0 {
		return false, "", "", "", fmt.Errorf("incomplete media information for download")
	}

	fmt.Printf("Attempting to download media for message %s in chat %s...\n", messageID, chatJID)

	// Extract direct path from URL
	directPath := extractDirectPathFromURL(url)

	// Create a downloader that implements DownloadableMessage
	var waMediaType whatsmeow.MediaType
	switch mediaType {
	case "image":
		waMediaType = whatsmeow.MediaImage
	case "video":
		waMediaType = whatsmeow.MediaVideo
	case "audio":
		waMediaType = whatsmeow.MediaAudio
	case "document":
		waMediaType = whatsmeow.MediaDocument
	default:
		return false, "", "", "", fmt.Errorf("unsupported media type: %s", mediaType)
	}

	downloader := &MediaDownloader{
		URL:           url,
		DirectPath:    directPath,
		MediaKey:      mediaKey,
		FileLength:    fileLength,
		FileSHA256:    fileSHA256,
		FileEncSHA256: fileEncSHA256,
		MediaType:     waMediaType,
	}

	// Download the media using whatsmeow client
	mediaData, err := client.Download(context.Background(), downloader)
	if err != nil {
		return false, "", "", "", fmt.Errorf("failed to download media: %v", err)
	}

	// Save the downloaded media to file
	if err := os.WriteFile(localPath, mediaData, 0644); err != nil {
		return false, "", "", "", fmt.Errorf("failed to save media file: %v", err)
	}

	fmt.Printf("Successfully downloaded %s media to %s (%d bytes)\n", mediaType, absPath, len(mediaData))
	return true, mediaType, filename, absPath, nil
}

// Extract direct path from a WhatsApp media URL
func extractDirectPathFromURL(url string) string {
	// The direct path is typically in the URL, we need to extract it
	// Example URL: https://mmg.whatsapp.net/v/t62.7118-24/13812002_698058036224062_3424455886509161511_n.enc?ccb=11-4&oh=...

	// Find the path part after the domain
	parts := strings.SplitN(url, ".net/", 2)
	if len(parts) < 2 {
		return url // Return original URL if parsing fails
	}

	pathPart := parts[1]

	// Remove query parameters
	pathPart = strings.SplitN(pathPart, "?", 2)[0]

	// Create proper direct path format
	return "/" + pathPart
}

// ============== HELPER FUNCTIONS ==============

// parseJID parses a JID string, handling phone numbers and full JIDs
func parseJID(recipient string) (types.JID, error) {
	if strings.Contains(recipient, "@") {
		return types.ParseJID(recipient)
	}
	return types.JID{
		User:   recipient,
		Server: "s.whatsapp.net",
	}, nil
}

// ============== MESSAGE FEATURE HANDLERS ==============

// handleReactToMessage sends a reaction to a message
func handleReactToMessage(client *whatsmeow.Client, chatJID, messageID, sender, emoji string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	senderJID, err := parseJID(sender)
	if err != nil {
		return false, fmt.Sprintf("Invalid sender JID: %v", err)
	}

	// Create reaction message
	reactionMsg := &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{
				RemoteJID:   proto.String(chatJID),
				FromMe:      proto.Bool(senderJID.User == client.Store.ID.User),
				ID:          proto.String(messageID),
				Participant: proto.String(senderJID.String()),
			},
			Text:              proto.String(emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}

	_, err = client.SendMessage(context.Background(), chat, reactionMsg)
	if err != nil {
		return false, fmt.Sprintf("Failed to send reaction: %v", err)
	}

	if emoji == "" {
		return true, "Reaction removed"
	}
	return true, fmt.Sprintf("Reacted with %s", emoji)
}

// handleReplyToMessage sends a reply to a specific message
func handleReplyToMessage(client *whatsmeow.Client, chatJID, messageID, quotedSender, message, mediaPath string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	quotedSenderJID, err := parseJID(quotedSender)
	if err != nil {
		return false, fmt.Sprintf("Invalid quoted sender JID: %v", err)
	}

	// Create context info for the reply
	contextInfo := &waProto.ContextInfo{
		StanzaID:      proto.String(messageID),
		Participant:   proto.String(quotedSenderJID.String()),
		QuotedMessage: &waProto.Message{Conversation: proto.String("")}, // Placeholder
	}

	var msg *waProto.Message
	if mediaPath != "" {
		// Handle media reply - similar to sendWhatsAppMessage but with context
		mediaData, err := os.ReadFile(mediaPath)
		if err != nil {
			return false, fmt.Sprintf("Error reading media file: %v", err)
		}

		fileExt := strings.ToLower(filepath.Ext(mediaPath))
		var mediaType whatsmeow.MediaType
		var mimeType string

		switch fileExt {
		case ".jpg", ".jpeg":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/jpeg"
		case ".png":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/png"
		case ".mp4":
			mediaType = whatsmeow.MediaVideo
			mimeType = "video/mp4"
		default:
			mediaType = whatsmeow.MediaDocument
			mimeType = "application/octet-stream"
		}

		resp, err := client.Upload(context.Background(), mediaData, mediaType)
		if err != nil {
			return false, fmt.Sprintf("Error uploading media: %v", err)
		}

		switch mediaType {
		case whatsmeow.MediaImage:
			msg = &waProto.Message{
				ImageMessage: &waProto.ImageMessage{
					Caption:       proto.String(message),
					Mimetype:      proto.String(mimeType),
					URL:           &resp.URL,
					DirectPath:    &resp.DirectPath,
					MediaKey:      resp.MediaKey,
					FileEncSHA256: resp.FileEncSHA256,
					FileSHA256:    resp.FileSHA256,
					FileLength:    &resp.FileLength,
					ContextInfo:   contextInfo,
				},
			}
		default:
			msg = &waProto.Message{
				DocumentMessage: &waProto.DocumentMessage{
					Caption:       proto.String(message),
					Mimetype:      proto.String(mimeType),
					URL:           &resp.URL,
					DirectPath:    &resp.DirectPath,
					MediaKey:      resp.MediaKey,
					FileEncSHA256: resp.FileEncSHA256,
					FileSHA256:    resp.FileSHA256,
					FileLength:    &resp.FileLength,
					ContextInfo:   contextInfo,
				},
			}
		}
	} else {
		msg = &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        proto.String(message),
				ContextInfo: contextInfo,
			},
		}
	}

	_, err = client.SendMessage(context.Background(), chat, msg)
	if err != nil {
		return false, fmt.Sprintf("Failed to send reply: %v", err)
	}

	return true, "Reply sent"
}

// handleDeleteMessage deletes a message
func handleDeleteMessage(client *whatsmeow.Client, chatJID, messageID, sender string, forAll bool) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	senderJID, err := parseJID(sender)
	if err != nil {
		return false, fmt.Sprintf("Invalid sender JID: %v", err)
	}

	isFromMe := senderJID.User == client.Store.ID.User

	if forAll {
		// Delete for everyone - using revoke
		msg := &waProto.Message{
			ProtocolMessage: &waProto.ProtocolMessage{
				Key: &waProto.MessageKey{
					RemoteJID: proto.String(chatJID),
					FromMe:    proto.Bool(isFromMe),
					ID:        proto.String(messageID),
				},
				Type: waProto.ProtocolMessage_REVOKE.Enum(),
			},
		}

		_, err = client.SendMessage(context.Background(), chat, msg)
		if err != nil {
			return false, fmt.Sprintf("Failed to delete message for everyone: %v", err)
		}
		return true, "Message deleted for everyone"
	}

	// Delete for me only - not directly supported, would need app state
	return false, "Delete for me only is not yet implemented"
}

// handleEditMessage edits a sent message
func handleEditMessage(client *whatsmeow.Client, chatJID, messageID, newText string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	msg := &waProto.Message{
		ProtocolMessage: &waProto.ProtocolMessage{
			Key: &waProto.MessageKey{
				RemoteJID: proto.String(chatJID),
				FromMe:    proto.Bool(true),
				ID:        proto.String(messageID),
			},
			Type: waProto.ProtocolMessage_MESSAGE_EDIT.Enum(),
			EditedMessage: &waProto.Message{
				Conversation: proto.String(newText),
			},
		},
	}

	_, err = client.SendMessage(context.Background(), chat, msg)
	if err != nil {
		return false, fmt.Sprintf("Failed to edit message: %v", err)
	}

	return true, "Message edited"
}

// ============== CHAT MANAGEMENT HANDLERS ==============

// handleMarkRead marks messages as read
func handleMarkRead(client *whatsmeow.Client, chatJID string, messageIDs []string, sender string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	senderJID, err := parseJID(sender)
	if err != nil {
		return false, fmt.Sprintf("Invalid sender JID: %v", err)
	}

	ids := make([]types.MessageID, len(messageIDs))
	for i, id := range messageIDs {
		ids[i] = types.MessageID(id)
	}

	err = client.MarkRead(context.Background(), ids, time.Now(), chat, senderJID)
	if err != nil {
		return false, fmt.Sprintf("Failed to mark as read: %v", err)
	}

	return true, "Marked as read"
}

// syncAndSendAppState syncs the app state before sending a patch to avoid conflicts
func syncAndSendAppState(client *whatsmeow.Client, patch appstate.PatchInfo) error {
	ctx := context.Background()

	// First, try to send without full sync
	err := client.SendAppState(ctx, patch)
	if err == nil {
		return nil
	}

	// Check if this is a missing keys error
	if strings.Contains(err.Error(), "no app state keys") {
		return fmt.Errorf("app state keys not synced yet - open WhatsApp on your phone to sync keys, then retry")
	}

	// If we get a conflict error, do a full sync and retry
	if strings.Contains(err.Error(), "conflict") || strings.Contains(err.Error(), "LTHash") {
		// Full sync the app state type
		syncErr := client.FetchAppState(ctx, patch.Type, true, false)
		if syncErr != nil {
			// Check if sync failed due to missing keys
			if strings.Contains(syncErr.Error(), "no app state keys") {
				return fmt.Errorf("app state keys not synced yet - open WhatsApp on your phone to sync keys, then retry")
			}
			return fmt.Errorf("sync failed: %v (original error: %v)", syncErr, err)
		}

		// Retry sending after sync
		retryErr := client.SendAppState(ctx, patch)
		if retryErr != nil && strings.Contains(retryErr.Error(), "no app state keys") {
			return fmt.Errorf("app state keys not synced yet - open WhatsApp on your phone to sync keys, then retry")
		}
		return retryErr
	}

	return err
}

// handlePinChat pins or unpins a chat using appstate
func handlePinChat(client *whatsmeow.Client, chatJID string, pin bool) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	patch := appstate.BuildPin(chat, pin)
	err = syncAndSendAppState(client, patch)
	if err != nil {
		return false, fmt.Sprintf("Failed to %s chat: %v", map[bool]string{true: "pin", false: "unpin"}[pin], err)
	}

	if pin {
		return true, "Chat pinned"
	}
	return true, "Chat unpinned"
}

// handleArchiveChat archives or unarchives a chat using appstate
func handleArchiveChat(client *whatsmeow.Client, chatJID string, archive bool) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	// Archive with current timestamp, no specific message key
	patch := appstate.BuildArchive(chat, archive, time.Now(), nil)
	err = syncAndSendAppState(client, patch)
	if err != nil {
		return false, fmt.Sprintf("Failed to %s chat: %v", map[bool]string{true: "archive", false: "unarchive"}[archive], err)
	}

	if archive {
		return true, "Chat archived"
	}
	return true, "Chat unarchived"
}

// handleMuteChat mutes or unmutes a chat using appstate
func handleMuteChat(client *whatsmeow.Client, chatJID string, mute bool, durationSeconds int64) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	var duration time.Duration
	if durationSeconds > 0 {
		duration = time.Duration(durationSeconds) * time.Second
	} // 0 means forever

	patch := appstate.BuildMute(chat, mute, duration)
	err = syncAndSendAppState(client, patch)
	if err != nil {
		return false, fmt.Sprintf("Failed to %s chat: %v", map[bool]string{true: "mute", false: "unmute"}[mute], err)
	}

	if mute {
		if durationSeconds > 0 {
			return true, fmt.Sprintf("Chat muted for %d seconds", durationSeconds)
		}
		return true, "Chat muted indefinitely"
	}
	return true, "Chat unmuted"
}

// handleStarMessage stars or unstars a message using appstate
func handleStarMessage(client *whatsmeow.Client, chatJID, messageID, sender string, star bool) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	senderJID, err := parseJID(sender)
	if err != nil {
		return false, fmt.Sprintf("Invalid sender JID: %v", err)
	}

	fromMe := senderJID.User == client.Store.ID.User
	patch := appstate.BuildStar(chat, senderJID, types.MessageID(messageID), fromMe, star)
	err = syncAndSendAppState(client, patch)
	if err != nil {
		return false, fmt.Sprintf("Failed to %s message: %v", map[bool]string{true: "star", false: "unstar"}[star], err)
	}

	if star {
		return true, "Message starred"
	}
	return true, "Message unstarred"
}

// handleForwardMessage forwards a message to another chat
func handleForwardMessage(client *whatsmeow.Client, fromChatJID, toChatJID, messageID string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	toChat, err := types.ParseJID(toChatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid destination chat JID: %v", err)
	}

	// Create a forwarded message
	// Note: We need the original message content to forward it
	// For now, we'll create a forwarded text message placeholder
	// In a full implementation, we'd need to retrieve the original message content

	forwardedMsg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String("[Forwarded message]"),
			ContextInfo: &waProto.ContextInfo{
				IsForwarded:    proto.Bool(true),
				ForwardingScore: proto.Uint32(1),
			},
		},
	}

	_, err = client.SendMessage(context.Background(), toChat, forwardedMsg)
	if err != nil {
		return false, fmt.Sprintf("Failed to forward message: %v", err)
	}

	return true, "Message forwarded"
}

// handleLabelChat labels or unlabels a chat
func handleLabelChat(client *whatsmeow.Client, chatJID, labelID string, labeled bool) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	patch := appstate.BuildLabelChat(chat, labelID, labeled)
	err = syncAndSendAppState(client, patch)
	if err != nil {
		return false, fmt.Sprintf("Failed to %s label: %v", map[bool]string{true: "add", false: "remove"}[labeled], err)
	}

	if labeled {
		return true, fmt.Sprintf("Label %s added to chat", labelID)
	}
	return true, fmt.Sprintf("Label %s removed from chat", labelID)
}

// handleDeleteChat deletes a chat using appstate
func handleDeleteChat(client *whatsmeow.Client, chatJID string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	patch := appstate.BuildDeleteChat(chat, time.Now(), nil, false)
	err = syncAndSendAppState(client, patch)
	if err != nil {
		return false, fmt.Sprintf("Failed to delete chat: %v", err)
	}

	return true, "Chat deleted"
}

// ============== GROUP MANAGEMENT HANDLERS ==============

// handleCreateGroup creates a new group
func handleCreateGroup(client *whatsmeow.Client, name string, participants []string) (bool, string, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp", ""
	}

	participantJIDs := make([]types.JID, len(participants))
	for i, p := range participants {
		jid, err := parseJID(p)
		if err != nil {
			return false, fmt.Sprintf("Invalid participant JID %s: %v", p, err), ""
		}
		participantJIDs[i] = jid
	}

	req := whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: participantJIDs,
	}

	groupInfo, err := client.CreateGroup(context.Background(), req)
	if err != nil {
		return false, fmt.Sprintf("Failed to create group: %v", err), ""
	}

	return true, "Group created", groupInfo.JID.String()
}

// handleGetGroupInfo gets information about a group
func handleGetGroupInfo(client *whatsmeow.Client, groupJID string) (*GroupInfoResponse, error) {
	if !client.IsConnected() {
		return nil, fmt.Errorf("not connected to WhatsApp")
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return nil, fmt.Errorf("invalid group JID: %v", err)
	}

	info, err := client.GetGroupInfo(context.Background(), jid)
	if err != nil {
		return nil, fmt.Errorf("failed to get group info: %v", err)
	}

	participants := make([]string, len(info.Participants))
	admins := []string{}
	for i, p := range info.Participants {
		participants[i] = p.JID.String()
		if p.IsAdmin || p.IsSuperAdmin {
			admins = append(admins, p.JID.String())
		}
	}

	return &GroupInfoResponse{
		Success:      true,
		Message:      "Group info retrieved",
		JID:          info.JID.String(),
		Name:         info.Name,
		Topic:        info.Topic,
		Created:      info.GroupCreated.Format(time.RFC3339),
		Creator:      info.OwnerJID.String(),
		Participants: participants,
		Admins:       admins,
	}, nil
}

// handleAddGroupMembers adds members to a group
func handleAddGroupMembers(client *whatsmeow.Client, groupJID string, participants []string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid group JID: %v", err)
	}

	participantJIDs := make([]types.JID, len(participants))
	for i, p := range participants {
		pJID, err := parseJID(p)
		if err != nil {
			return false, fmt.Sprintf("Invalid participant JID %s: %v", p, err)
		}
		participantJIDs[i] = pJID
	}

	_, err = client.UpdateGroupParticipants(context.Background(), jid, participantJIDs, whatsmeow.ParticipantChangeAdd)
	if err != nil {
		return false, fmt.Sprintf("Failed to add members: %v", err)
	}

	return true, "Members added"
}

// handleRemoveGroupMembers removes members from a group
func handleRemoveGroupMembers(client *whatsmeow.Client, groupJID string, participants []string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid group JID: %v", err)
	}

	participantJIDs := make([]types.JID, len(participants))
	for i, p := range participants {
		pJID, err := parseJID(p)
		if err != nil {
			return false, fmt.Sprintf("Invalid participant JID %s: %v", p, err)
		}
		participantJIDs[i] = pJID
	}

	_, err = client.UpdateGroupParticipants(context.Background(), jid, participantJIDs, whatsmeow.ParticipantChangeRemove)
	if err != nil {
		return false, fmt.Sprintf("Failed to remove members: %v", err)
	}

	return true, "Members removed"
}

// handlePromoteGroupAdmin promotes members to admin
func handlePromoteGroupAdmin(client *whatsmeow.Client, groupJID string, participants []string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid group JID: %v", err)
	}

	participantJIDs := make([]types.JID, len(participants))
	for i, p := range participants {
		pJID, err := parseJID(p)
		if err != nil {
			return false, fmt.Sprintf("Invalid participant JID %s: %v", p, err)
		}
		participantJIDs[i] = pJID
	}

	_, err = client.UpdateGroupParticipants(context.Background(), jid, participantJIDs, whatsmeow.ParticipantChangePromote)
	if err != nil {
		return false, fmt.Sprintf("Failed to promote to admin: %v", err)
	}

	return true, "Promoted to admin"
}

// handleDemoteGroupAdmin demotes admins to regular members
func handleDemoteGroupAdmin(client *whatsmeow.Client, groupJID string, participants []string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid group JID: %v", err)
	}

	participantJIDs := make([]types.JID, len(participants))
	for i, p := range participants {
		pJID, err := parseJID(p)
		if err != nil {
			return false, fmt.Sprintf("Invalid participant JID %s: %v", p, err)
		}
		participantJIDs[i] = pJID
	}

	_, err = client.UpdateGroupParticipants(context.Background(), jid, participantJIDs, whatsmeow.ParticipantChangeDemote)
	if err != nil {
		return false, fmt.Sprintf("Failed to demote admin: %v", err)
	}

	return true, "Demoted from admin"
}

// handleSetGroupName sets the group name
func handleSetGroupName(client *whatsmeow.Client, groupJID, name string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid group JID: %v", err)
	}

	err = client.SetGroupName(context.Background(), jid, name)
	if err != nil {
		return false, fmt.Sprintf("Failed to set group name: %v", err)
	}

	return true, "Group name updated"
}

// handleSetGroupTopic sets the group description/topic
func handleSetGroupTopic(client *whatsmeow.Client, groupJID, topic string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid group JID: %v", err)
	}

	err = client.SetGroupTopic(context.Background(), jid, "", "", topic)
	if err != nil {
		return false, fmt.Sprintf("Failed to set group topic: %v", err)
	}

	return true, "Group topic updated"
}

// handleSetGroupPhoto sets the group photo
func handleSetGroupPhoto(client *whatsmeow.Client, groupJID, photoPath string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid group JID: %v", err)
	}

	photoData, err := os.ReadFile(photoPath)
	if err != nil {
		return false, fmt.Sprintf("Failed to read photo: %v", err)
	}

	_, err = client.SetGroupPhoto(context.Background(), jid, photoData)
	if err != nil {
		return false, fmt.Sprintf("Failed to set group photo: %v", err)
	}

	return true, "Group photo updated"
}

// handleGetGroupInviteLink gets the group invite link
func handleGetGroupInviteLink(client *whatsmeow.Client, groupJID string, revoke bool) (bool, string, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp", ""
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid group JID: %v", err), ""
	}

	var link string
	if revoke {
		link, err = client.GetGroupInviteLink(context.Background(), jid, true)
	} else {
		link, err = client.GetGroupInviteLink(context.Background(), jid, false)
	}

	if err != nil {
		return false, fmt.Sprintf("Failed to get invite link: %v", err), ""
	}

	return true, "Invite link retrieved", link
}

// handleJoinGroupWithLink joins a group using an invite link
func handleJoinGroupWithLink(client *whatsmeow.Client, inviteLink string) (bool, string, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp", ""
	}

	// Extract invite code from link
	inviteCode := inviteLink
	if strings.Contains(inviteLink, "chat.whatsapp.com/") {
		parts := strings.Split(inviteLink, "chat.whatsapp.com/")
		if len(parts) > 1 {
			inviteCode = parts[1]
		}
	}

	groupJID, err := client.JoinGroupWithLink(context.Background(), inviteCode)
	if err != nil {
		return false, fmt.Sprintf("Failed to join group: %v", err), ""
	}

	return true, "Joined group", groupJID.String()
}

// handleLeaveGroup leaves a group
func handleLeaveGroup(client *whatsmeow.Client, groupJID string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid group JID: %v", err)
	}

	err = client.LeaveGroup(context.Background(), jid)
	if err != nil {
		return false, fmt.Sprintf("Failed to leave group: %v", err)
	}

	return true, "Left group"
}

// ============== PROFILE & PRIVACY HANDLERS ==============

// handleGetProfile gets the current user's profile
func handleGetProfile(client *whatsmeow.Client) (*ProfileResponse, error) {
	if !client.IsConnected() {
		return nil, fmt.Errorf("not connected to WhatsApp")
	}

	return &ProfileResponse{
		Success:     true,
		Message:     "Profile retrieved",
		JID:         client.Store.ID.String(),
		PhoneNumber: client.Store.ID.User,
	}, nil
}

// handleSetProfilePhoto sets the profile photo
func handleSetProfilePhoto(client *whatsmeow.Client, photoPath string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	photoData, err := os.ReadFile(photoPath)
	if err != nil {
		return false, fmt.Sprintf("Failed to read photo: %v", err)
	}

	_, err = client.SetGroupPhoto(context.Background(), types.EmptyJID, photoData)
	if err != nil {
		return false, fmt.Sprintf("Failed to set profile photo: %v", err)
	}

	return true, "Profile photo updated"
}

// handleGetPrivacySettings gets privacy settings
func handleGetPrivacySettings(client *whatsmeow.Client) (*PrivacySettingsResponse, error) {
	if !client.IsConnected() {
		return nil, fmt.Errorf("not connected to WhatsApp")
	}

	settings := client.GetPrivacySettings(context.Background())

	return &PrivacySettingsResponse{
		Success:      true,
		Message:      "Privacy settings retrieved",
		LastSeen:     string(settings.LastSeen),
		ProfilePhoto: string(settings.Profile),
		About:        string(settings.Status),
		ReadReceipts: string(settings.ReadReceipts),
		Groups:       string(settings.GroupAdd),
	}, nil
}

// handleBlockContact blocks a contact
func handleBlockContact(client *whatsmeow.Client, jidStr string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := parseJID(jidStr)
	if err != nil {
		return false, fmt.Sprintf("Invalid JID: %v", err)
	}

	_, err = client.UpdateBlocklist(context.Background(), jid, events.BlocklistChangeActionBlock)
	if err != nil {
		return false, fmt.Sprintf("Failed to block contact: %v", err)
	}

	return true, "Contact blocked"
}

// handleUnblockContact unblocks a contact
func handleUnblockContact(client *whatsmeow.Client, jidStr string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := parseJID(jidStr)
	if err != nil {
		return false, fmt.Sprintf("Invalid JID: %v", err)
	}

	_, err = client.UpdateBlocklist(context.Background(), jid, events.BlocklistChangeActionUnblock)
	if err != nil {
		return false, fmt.Sprintf("Failed to unblock contact: %v", err)
	}

	return true, "Contact unblocked"
}

// handleGetBlockedContacts gets the list of blocked contacts
func handleGetBlockedContacts(client *whatsmeow.Client) (*BlockedContactsResponse, error) {
	if !client.IsConnected() {
		return nil, fmt.Errorf("not connected to WhatsApp")
	}

	blocklist, err := client.GetBlocklist(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get blocklist: %v", err)
	}

	contacts := make([]string, len(blocklist.JIDs))
	for i, jid := range blocklist.JIDs {
		contacts[i] = jid.String()
	}

	return &BlockedContactsResponse{
		Success:  true,
		Message:  "Blocklist retrieved",
		Contacts: contacts,
	}, nil
}

// ============== PRESENCE HANDLERS ==============

// handleSubscribePresence subscribes to presence updates for a contact
func handleSubscribePresence(client *whatsmeow.Client, jidStr string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	jid, err := parseJID(jidStr)
	if err != nil {
		return false, fmt.Sprintf("Invalid JID: %v", err)
	}

	err = client.SubscribePresence(context.Background(), jid)
	if err != nil {
		return false, fmt.Sprintf("Failed to subscribe to presence: %v", err)
	}

	return true, "Subscribed to presence"
}

// handleSendTyping sends typing indicator
func handleSendTyping(client *whatsmeow.Client, chatJID string, typing bool) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	var presence types.ChatPresence
	if typing {
		presence = types.ChatPresenceComposing
	} else {
		presence = types.ChatPresencePaused
	}

	err = client.SendChatPresence(context.Background(), chat, presence, types.ChatPresenceMediaText)
	if err != nil {
		return false, fmt.Sprintf("Failed to send typing indicator: %v", err)
	}

	return true, "Typing indicator sent"
}

// handleSendRecording sends recording indicator
func handleSendRecording(client *whatsmeow.Client, chatJID string, recording bool) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	var presence types.ChatPresence
	if recording {
		presence = types.ChatPresenceComposing
	} else {
		presence = types.ChatPresencePaused
	}

	err = client.SendChatPresence(context.Background(), chat, presence, types.ChatPresenceMediaAudio)
	if err != nil {
		return false, fmt.Sprintf("Failed to send recording indicator: %v", err)
	}

	return true, "Recording indicator sent"
}

// handleSetPresence sets online/offline presence
func handleSetPresence(client *whatsmeow.Client, available bool) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	var presence types.Presence
	if available {
		presence = types.PresenceAvailable
	} else {
		presence = types.PresenceUnavailable
	}

	err := client.SendPresence(context.Background(), presence)
	if err != nil {
		return false, fmt.Sprintf("Failed to set presence: %v", err)
	}

	return true, "Presence updated"
}

// ============== STATUS/STORIES HANDLERS ==============

// handlePostTextStatus posts a text status
func handlePostTextStatus(client *whatsmeow.Client, text string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	statusJID := types.JID{
		User:   "status",
		Server: "broadcast",
	}

	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
		},
	}

	_, err := client.SendMessage(context.Background(), statusJID, msg)
	if err != nil {
		return false, fmt.Sprintf("Failed to post status: %v", err)
	}

	return true, "Status posted"
}

// handlePostImageStatus posts an image status
func handlePostImageStatus(client *whatsmeow.Client, imagePath, caption string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return false, fmt.Sprintf("Failed to read image: %v", err)
	}

	resp, err := client.Upload(context.Background(), imageData, whatsmeow.MediaImage)
	if err != nil {
		return false, fmt.Sprintf("Failed to upload image: %v", err)
	}

	statusJID := types.JID{
		User:   "status",
		Server: "broadcast",
	}

	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String("image/jpeg"),
			URL:           &resp.URL,
			DirectPath:    &resp.DirectPath,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
		},
	}

	_, err = client.SendMessage(context.Background(), statusJID, msg)
	if err != nil {
		return false, fmt.Sprintf("Failed to post status: %v", err)
	}

	return true, "Image status posted"
}

// ============== UTILITY HANDLERS ==============

// handleSendLocation sends a location message
func handleSendLocation(client *whatsmeow.Client, chatJID string, lat, lng float64, name, address string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	msg := &waProto.Message{
		LocationMessage: &waProto.LocationMessage{
			DegreesLatitude:  proto.Float64(lat),
			DegreesLongitude: proto.Float64(lng),
			Name:             proto.String(name),
			Address:          proto.String(address),
		},
	}

	_, err = client.SendMessage(context.Background(), chat, msg)
	if err != nil {
		return false, fmt.Sprintf("Failed to send location: %v", err)
	}

	return true, "Location sent"
}

// handleSendContact sends a contact card
func handleSendContact(client *whatsmeow.Client, chatJID, displayName, phoneNumber string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return false, fmt.Sprintf("Invalid chat JID: %v", err)
	}

	// Create vCard format
	vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nFN:%s\nTEL;type=CELL;type=VOICE;waid=%s:+%s\nEND:VCARD",
		displayName, phoneNumber, phoneNumber)

	msg := &waProto.Message{
		ContactMessage: &waProto.ContactMessage{
			DisplayName: proto.String(displayName),
			Vcard:       proto.String(vcard),
		},
	}

	_, err = client.SendMessage(context.Background(), chat, msg)
	if err != nil {
		return false, fmt.Sprintf("Failed to send contact: %v", err)
	}

	return true, "Contact sent"
}

// handleCheckPhoneNumbers checks if phone numbers are registered on WhatsApp
func handleCheckPhoneNumbers(client *whatsmeow.Client, phoneNumbers []string) (*CheckPhoneResponse, error) {
	if !client.IsConnected() {
		return nil, fmt.Errorf("not connected to WhatsApp")
	}

	results, err := client.IsOnWhatsApp(context.Background(), phoneNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to check phone numbers: %v", err)
	}

	registered := make(map[string]bool)
	jids := make(map[string]string)

	for _, result := range results {
		registered[result.Query] = result.IsIn
		if result.IsIn {
			jids[result.Query] = result.JID.String()
		}
	}

	return &CheckPhoneResponse{
		Success: true,
		Message: "Phone numbers checked",
		Results: registered,
		JIDs:    jids,
	}, nil
}

// handleGetContactInfo gets information about a contact
func handleGetContactInfo(client *whatsmeow.Client, jidStr string) (*ContactInfoResponse, error) {
	if !client.IsConnected() {
		return nil, fmt.Errorf("not connected to WhatsApp")
	}

	jid, err := parseJID(jidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid JID: %v", err)
	}

	contact, err := client.Store.Contacts.GetContact(context.Background(), jid)
	if err != nil {
		return nil, fmt.Errorf("failed to get contact: %v", err)
	}

	return &ContactInfoResponse{
		Success:      true,
		Message:      "Contact info retrieved",
		JID:          jid.String(),
		FullName:     contact.FullName,
		PushName:     contact.PushName,
		BusinessName: contact.BusinessName,
	}, nil
}

// Start a REST API server to expose the WhatsApp client functionality
func startRESTServer(client *whatsmeow.Client, messageStore *MessageStore, port int) {
	// Handler for sending messages
	http.HandleFunc("/api/send", func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body
		var req SendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.Recipient == "" {
			http.Error(w, "Recipient is required", http.StatusBadRequest)
			return
		}

		if req.Message == "" && req.MediaPath == "" {
			http.Error(w, "Message or media path is required", http.StatusBadRequest)
			return
		}

		fmt.Println("Received request to send message", req.Message, req.MediaPath)

		// Send the message
		success, message := sendWhatsAppMessage(client, req.Recipient, req.Message, req.MediaPath)
		fmt.Println("Message sent", success, message)
		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Set appropriate status code
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}

		// Send response
		json.NewEncoder(w).Encode(SendMessageResponse{
			Success: success,
			Message: message,
		})
	})

	// Handler for downloading media
	http.HandleFunc("/api/download", func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body
		var req DownloadMediaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.MessageID == "" || req.ChatJID == "" {
			http.Error(w, "Message ID and Chat JID are required", http.StatusBadRequest)
			return
		}

		// Download the media
		success, mediaType, filename, path, err := downloadMedia(client, messageStore, req.MessageID, req.ChatJID)

		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Handle download result
		if !success || err != nil {
			errMsg := "Unknown error"
			if err != nil {
				errMsg = err.Error()
			}

			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(DownloadMediaResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to download media: %s", errMsg),
			})
			return
		}

		// Send successful response
		json.NewEncoder(w).Encode(DownloadMediaResponse{
			Success:  true,
			Message:  fmt.Sprintf("Successfully downloaded %s media", mediaType),
			Filename: filename,
			Path:     path,
		})
	})

	// ============== MESSAGE FEATURE ROUTES ==============

	// React to message
	http.HandleFunc("/api/message/react", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ReactRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleReactToMessage(client, req.ChatJID, req.MessageID, req.Sender, req.Emoji)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Reply to message
	http.HandleFunc("/api/message/reply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ReplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleReplyToMessage(client, req.ChatJID, req.MessageID, req.QuotedSender, req.Message, req.MediaPath)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Delete message
	http.HandleFunc("/api/message/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req DeleteMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleDeleteMessage(client, req.ChatJID, req.MessageID, req.Sender, req.ForAll)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Edit message
	http.HandleFunc("/api/message/edit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req EditMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleEditMessage(client, req.ChatJID, req.MessageID, req.NewText)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// ============== CHAT MANAGEMENT ROUTES ==============

	// Mark messages as read
	http.HandleFunc("/api/chat/read", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req MarkReadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleMarkRead(client, req.ChatJID, req.MessageIDs, req.Sender)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// ============== GROUP MANAGEMENT ROUTES ==============

	// Create group
	http.HandleFunc("/api/group/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CreateGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message, groupJID := handleCreateGroup(client, req.Name, req.Participants)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(CreateGroupResponse{Success: success, Message: message, GroupJID: groupJID})
	})

	// Get group info
	http.HandleFunc("/api/group/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupInfoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		resp, err := handleGetGroupInfo(client, req.GroupJID)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(GroupInfoResponse{Success: false, Message: err.Error()})
			return
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Add group members
	http.HandleFunc("/api/group/members/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupMembersRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleAddGroupMembers(client, req.GroupJID, req.Participants)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Remove group members
	http.HandleFunc("/api/group/members/remove", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupMembersRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleRemoveGroupMembers(client, req.GroupJID, req.Participants)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Promote to admin
	http.HandleFunc("/api/group/admin/promote", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupAdminRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handlePromoteGroupAdmin(client, req.GroupJID, req.Participants)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Demote admin
	http.HandleFunc("/api/group/admin/demote", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupAdminRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleDemoteGroupAdmin(client, req.GroupJID, req.Participants)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Set group name
	http.HandleFunc("/api/group/name", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupNameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleSetGroupName(client, req.GroupJID, req.Name)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Set group topic/description
	http.HandleFunc("/api/group/topic", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupTopicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleSetGroupTopic(client, req.GroupJID, req.Topic)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Set group photo
	http.HandleFunc("/api/group/photo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupPhotoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleSetGroupPhoto(client, req.GroupJID, req.PhotoPath)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Get group invite link
	http.HandleFunc("/api/group/invite", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupInfoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message, link := handleGetGroupInviteLink(client, req.GroupJID, false)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GroupInviteResponse{Success: success, Message: message, InviteLink: link})
	})

	// Revoke group invite link
	http.HandleFunc("/api/group/invite/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupInfoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message, link := handleGetGroupInviteLink(client, req.GroupJID, true)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GroupInviteResponse{Success: success, Message: message, InviteLink: link})
	})

	// Join group via invite
	http.HandleFunc("/api/group/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req JoinGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message, groupJID := handleJoinGroupWithLink(client, req.InviteLink)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(CreateGroupResponse{Success: success, Message: message, GroupJID: groupJID})
	})

	// Leave group
	http.HandleFunc("/api/group/leave", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GroupInfoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleLeaveGroup(client, req.GroupJID)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// ============== PROFILE & PRIVACY ROUTES ==============

	// Get profile
	http.HandleFunc("/api/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp, err := handleGetProfile(client)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ProfileResponse{Success: false, Message: err.Error()})
			return
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Set profile photo
	http.HandleFunc("/api/profile/photo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ProfilePhotoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleSetProfilePhoto(client, req.PhotoPath)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Get privacy settings
	http.HandleFunc("/api/privacy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp, err := handleGetPrivacySettings(client)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(PrivacySettingsResponse{Success: false, Message: err.Error()})
			return
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Block contact
	http.HandleFunc("/api/contact/block", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req BlockContactRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleBlockContact(client, req.JID)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Unblock contact
	http.HandleFunc("/api/contact/unblock", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req BlockContactRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleUnblockContact(client, req.JID)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Get blocked contacts
	http.HandleFunc("/api/contact/blocked", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp, err := handleGetBlockedContacts(client)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(BlockedContactsResponse{Success: false, Message: err.Error()})
			return
		}
		json.NewEncoder(w).Encode(resp)
	})

	// ============== PRESENCE ROUTES ==============

	// Subscribe to presence
	http.HandleFunc("/api/presence/subscribe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req PresenceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleSubscribePresence(client, req.JID)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Send typing indicator
	http.HandleFunc("/api/presence/typing", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req TypingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleSendTyping(client, req.ChatJID, req.Typing)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Send recording indicator
	http.HandleFunc("/api/presence/recording", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RecordingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleSendRecording(client, req.ChatJID, req.Recording)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Set online presence
	http.HandleFunc("/api/presence/online", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		success, message := handleSetPresence(client, true)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Set offline presence
	http.HandleFunc("/api/presence/offline", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		success, message := handleSetPresence(client, false)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// ============== STATUS/STORIES ROUTES ==============

	// Post text status
	http.HandleFunc("/api/status/text", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req StatusTextRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handlePostTextStatus(client, req.Text)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Post image status
	http.HandleFunc("/api/status/image", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req StatusMediaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handlePostImageStatus(client, req.MediaPath, req.Caption)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// ============== UTILITY ROUTES ==============

	// Send location
	http.HandleFunc("/api/message/location", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req LocationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleSendLocation(client, req.ChatJID, req.Latitude, req.Longitude, req.Name, req.Address)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Send contact card
	http.HandleFunc("/api/message/contact", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ContactCardRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleSendContact(client, req.ChatJID, req.DisplayName, req.PhoneNumber)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Check phone numbers on WhatsApp
	http.HandleFunc("/api/contact/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CheckPhoneRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		resp, err := handleCheckPhoneNumbers(client, req.PhoneNumbers)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(CheckPhoneResponse{Success: false, Message: err.Error()})
			return
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Get contact info
	http.HandleFunc("/api/contact/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ContactInfoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		resp, err := handleGetContactInfo(client, req.JID)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ContactInfoResponse{Success: false, Message: err.Error()})
			return
		}
		json.NewEncoder(w).Encode(resp)
	})

	// ============== CHAT STATE ROUTES (PIN, ARCHIVE, MUTE, STAR) ==============

	// Pin chat
	http.HandleFunc("/api/chat/pin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handlePinChat(client, req.ChatJID, true)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Unpin chat
	http.HandleFunc("/api/chat/unpin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handlePinChat(client, req.ChatJID, false)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Archive chat
	http.HandleFunc("/api/chat/archive", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleArchiveChat(client, req.ChatJID, true)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Unarchive chat
	http.HandleFunc("/api/chat/unarchive", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleArchiveChat(client, req.ChatJID, false)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Mute chat
	http.HandleFunc("/api/chat/mute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req MuteChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleMuteChat(client, req.ChatJID, true, req.Duration)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Unmute chat
	http.HandleFunc("/api/chat/unmute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleMuteChat(client, req.ChatJID, false, 0)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Delete chat
	http.HandleFunc("/api/chat/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleDeleteChat(client, req.ChatJID)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Star message
	http.HandleFunc("/api/message/star", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req StarMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Use chat JID as sender for now - proper implementation would need the actual sender
		success, message := handleStarMessage(client, req.ChatJID, req.MessageID, req.ChatJID, req.Star)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Forward message
	http.HandleFunc("/api/message/forward", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ForwardRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleForwardMessage(client, req.FromChatJID, req.ToChatJID, req.MessageID)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Label chat
	http.HandleFunc("/api/chat/label", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ChatJID string `json:"chat_jid"`
			LabelID string `json:"label_id"`
			Labeled bool   `json:"labeled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		success, message := handleLabelChat(client, req.ChatJID, req.LabelID, req.Labeled)
		w.Header().Set("Content-Type", "application/json")
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(GenericResponse{Success: success, Message: message})
	})

	// Sync app state (triggers key requests for missing keys)
	http.HandleFunc("/api/sync/appstate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := context.Background()
		var errors []string

		// Try to sync all app state types - this will request missing keys
		for _, name := range []appstate.WAPatchName{
			appstate.WAPatchCriticalBlock,
			appstate.WAPatchCriticalUnblockLow,
			appstate.WAPatchRegularHigh,
			appstate.WAPatchRegular,
			appstate.WAPatchRegularLow,
		} {
			err := client.FetchAppState(ctx, name, true, false)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", name, err))
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if len(errors) > 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Some app states failed to sync - keys may be requested from phone",
				"errors":  errors,
			})
		} else {
			json.NewEncoder(w).Encode(GenericResponse{Success: true, Message: "All app states synced successfully"})
		}
	})

	// Start the server
	serverAddr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting REST API server on %s...\n", serverAddr)

	// Run server in a goroutine so it doesn't block
	go func() {
		if err := http.ListenAndServe(serverAddr, nil); err != nil {
			fmt.Printf("REST API server error: %v\n", err)
		}
	}()
}

func main() {
	// Set up logger
	logger := waLog.Stdout("Client", "INFO", true)
	logger.Infof("Starting WhatsApp client...")

	// Set device name that appears in WhatsApp's Linked Devices
	store.SetOSInfo("MCP Server", [3]uint32{1, 0, 0})

	// Create database connection for storing session data
	dbLog := waLog.Stdout("Database", "INFO", true)

	// Create directory for database if it doesn't exist
	if err := os.MkdirAll("store", 0755); err != nil {
		logger.Errorf("Failed to create store directory: %v", err)
		return
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", "file:store/whatsapp.db?_foreign_keys=on", dbLog)
	if err != nil {
		logger.Errorf("Failed to connect to database: %v", err)
		return
	}

	// Get device store - This contains session information
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			// No device exists, create one
			deviceStore = container.NewDevice()
			logger.Infof("Created new device")
		} else {
			logger.Errorf("Failed to get device: %v", err)
			return
		}
	}

	// Create client instance
	client := whatsmeow.NewClient(deviceStore, logger)
	if client == nil {
		logger.Errorf("Failed to create WhatsApp client")
		return
	}

	// Initialize message store
	messageStore, err := NewMessageStore()
	if err != nil {
		logger.Errorf("Failed to initialize message store: %v", err)
		return
	}
	defer messageStore.Close()

	// Setup event handling for messages and history sync
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			// Process regular messages
			handleMessage(client, messageStore, v, logger)

		case *events.HistorySync:
			// Process history sync events
			handleHistorySync(client, messageStore, v, logger)

		case *events.Connected:
			logger.Infof("Connected to WhatsApp")

		case *events.LoggedOut:
			logger.Warnf("Device logged out, please scan QR code to log in again")
		}
	})

	// Create channel to track connection success
	connected := make(chan bool, 1)

	// Connect to WhatsApp
	if client.Store.ID == nil {
		// No ID stored, this is a new client, need to pair with phone
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			logger.Errorf("Failed to connect: %v", err)
			return
		}

		// Print QR code for pairing with phone
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("\nScan this QR code with your WhatsApp app:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else if evt.Event == "success" {
				connected <- true
				break
			}
		}

		// Wait for connection
		select {
		case <-connected:
			fmt.Println("\nSuccessfully connected and authenticated!")
		case <-time.After(3 * time.Minute):
			logger.Errorf("Timeout waiting for QR code scan")
			return
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			logger.Errorf("Failed to connect: %v", err)
			return
		}
		connected <- true
	}

	// Wait a moment for connection to stabilize
	time.Sleep(2 * time.Second)

	if !client.IsConnected() {
		logger.Errorf("Failed to establish stable connection")
		return
	}

	fmt.Println("\n✓ Connected to WhatsApp! Type 'help' for commands.")

	// Start REST API server
	startRESTServer(client, messageStore, 8080)

	// Create a channel to keep the main goroutine alive
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("REST server is running. Press Ctrl+C to disconnect and exit.")

	// Wait for termination signal
	<-exitChan

	fmt.Println("Disconnecting...")
	// Disconnect client
	client.Disconnect()
}

// GetChatName determines the appropriate name for a chat based on JID and other info
func GetChatName(client *whatsmeow.Client, messageStore *MessageStore, jid types.JID, chatJID string, conversation interface{}, sender string, logger waLog.Logger) string {
	// First, check if chat already exists in database with a name
	var existingName string
	err := messageStore.db.QueryRow("SELECT name FROM chats WHERE jid = ?", chatJID).Scan(&existingName)
	if err == nil && existingName != "" {
		// Chat exists with a name, use that
		logger.Infof("Using existing chat name for %s: %s", chatJID, existingName)
		return existingName
	}

	// Need to determine chat name
	var name string

	if jid.Server == "g.us" {
		// This is a group chat
		logger.Infof("Getting name for group: %s", chatJID)

		// Use conversation data if provided (from history sync)
		if conversation != nil {
			// Extract name from conversation if available
			// This uses type assertions to handle different possible types
			var displayName, convName *string
			// Try to extract the fields we care about regardless of the exact type
			v := reflect.ValueOf(conversation)
			if v.Kind() == reflect.Ptr && !v.IsNil() {
				v = v.Elem()

				// Try to find DisplayName field
				if displayNameField := v.FieldByName("DisplayName"); displayNameField.IsValid() && displayNameField.Kind() == reflect.Ptr && !displayNameField.IsNil() {
					dn := displayNameField.Elem().String()
					displayName = &dn
				}

				// Try to find Name field
				if nameField := v.FieldByName("Name"); nameField.IsValid() && nameField.Kind() == reflect.Ptr && !nameField.IsNil() {
					n := nameField.Elem().String()
					convName = &n
				}
			}

			// Use the name we found
			if displayName != nil && *displayName != "" {
				name = *displayName
			} else if convName != nil && *convName != "" {
				name = *convName
			}
		}

		// If we didn't get a name, try group info
		if name == "" {
			groupInfo, err := client.GetGroupInfo(context.Background(), jid)
			if err == nil && groupInfo.Name != "" {
				name = groupInfo.Name
			} else {
				// Fallback name for groups
				name = fmt.Sprintf("Group %s", jid.User)
			}
		}

		logger.Infof("Using group name: %s", name)
	} else {
		// This is an individual contact
		logger.Infof("Getting name for contact: %s", chatJID)

		// Just use contact info (full name)
		contact, err := client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.FullName != "" {
			name = contact.FullName
		} else if sender != "" {
			// Fallback to sender
			name = sender
		} else {
			// Last fallback to JID
			name = jid.User
		}

		logger.Infof("Using contact name: %s", name)
	}

	return name
}

// Handle history sync events
func handleHistorySync(client *whatsmeow.Client, messageStore *MessageStore, historySync *events.HistorySync, logger waLog.Logger) {
	fmt.Printf("Received history sync event with %d conversations\n", len(historySync.Data.Conversations))

	syncedCount := 0
	for _, conversation := range historySync.Data.Conversations {
		// Parse JID from the conversation
		if conversation.ID == nil {
			continue
		}

		chatJID := *conversation.ID

		// Try to parse the JID
		jid, err := types.ParseJID(chatJID)
		if err != nil {
			logger.Warnf("Failed to parse JID %s: %v", chatJID, err)
			continue
		}

		// Get appropriate chat name by passing the history sync conversation directly
		name := GetChatName(client, messageStore, jid, chatJID, conversation, "", logger)

		// Process messages
		messages := conversation.Messages
		if len(messages) > 0 {
			// Update chat with latest message timestamp
			latestMsg := messages[0]
			if latestMsg == nil || latestMsg.Message == nil {
				continue
			}

			// Get timestamp from message info
			timestamp := time.Time{}
			if ts := latestMsg.Message.GetMessageTimestamp(); ts != 0 {
				timestamp = time.Unix(int64(ts), 0)
			} else {
				continue
			}

			messageStore.StoreChat(chatJID, name, timestamp)

			// Store messages
			for _, msg := range messages {
				if msg == nil || msg.Message == nil {
					continue
				}

				// Extract text content
				var content string
				if msg.Message.Message != nil {
					if conv := msg.Message.Message.GetConversation(); conv != "" {
						content = conv
					} else if ext := msg.Message.Message.GetExtendedTextMessage(); ext != nil {
						content = ext.GetText()
					}
				}

				// Extract media info
				var mediaType, filename, url string
				var mediaKey, fileSHA256, fileEncSHA256 []byte
				var fileLength uint64

				if msg.Message.Message != nil {
					mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength = extractMediaInfo(msg.Message.Message)
				}

				// Log the message content for debugging
				logger.Infof("Message content: %v, Media Type: %v", content, mediaType)

				// Skip messages with no content and no media
				if content == "" && mediaType == "" {
					continue
				}

				// Determine sender
				var sender string
				isFromMe := false
				if msg.Message.Key != nil {
					if msg.Message.Key.FromMe != nil {
						isFromMe = *msg.Message.Key.FromMe
					}
					if !isFromMe && msg.Message.Key.Participant != nil && *msg.Message.Key.Participant != "" {
						sender = *msg.Message.Key.Participant
					} else if isFromMe {
						sender = client.Store.ID.User
					} else {
						sender = jid.User
					}
				} else {
					sender = jid.User
				}

				// Store message
				msgID := ""
				if msg.Message.Key != nil && msg.Message.Key.ID != nil {
					msgID = *msg.Message.Key.ID
				}

				// Get message timestamp
				timestamp := time.Time{}
				if ts := msg.Message.GetMessageTimestamp(); ts != 0 {
					timestamp = time.Unix(int64(ts), 0)
				} else {
					continue
				}

				err = messageStore.StoreMessage(
					msgID,
					chatJID,
					sender,
					content,
					timestamp,
					isFromMe,
					mediaType,
					filename,
					url,
					mediaKey,
					fileSHA256,
					fileEncSHA256,
					fileLength,
				)
				if err != nil {
					logger.Warnf("Failed to store history message: %v", err)
				} else {
					syncedCount++
					// Log successful message storage
					if mediaType != "" {
						logger.Infof("Stored message: [%s] %s -> %s: [%s: %s] %s",
							timestamp.Format("2006-01-02 15:04:05"), sender, chatJID, mediaType, filename, content)
					} else {
						logger.Infof("Stored message: [%s] %s -> %s: %s",
							timestamp.Format("2006-01-02 15:04:05"), sender, chatJID, content)
					}
				}
			}
		}
	}

	fmt.Printf("History sync complete. Stored %d messages.\n", syncedCount)
}

// Request history sync from the server
func requestHistorySync(client *whatsmeow.Client) {
	if client == nil {
		fmt.Println("Client is not initialized. Cannot request history sync.")
		return
	}

	if !client.IsConnected() {
		fmt.Println("Client is not connected. Please ensure you are connected to WhatsApp first.")
		return
	}

	if client.Store.ID == nil {
		fmt.Println("Client is not logged in. Please scan the QR code first.")
		return
	}

	// Build and send a history sync request
	historyMsg := client.BuildHistorySyncRequest(nil, 100)
	if historyMsg == nil {
		fmt.Println("Failed to build history sync request.")
		return
	}

	_, err := client.SendMessage(context.Background(), types.JID{
		Server: "s.whatsapp.net",
		User:   "status",
	}, historyMsg)

	if err != nil {
		fmt.Printf("Failed to request history sync: %v\n", err)
	} else {
		fmt.Println("History sync requested. Waiting for server response...")
	}
}

// analyzeOggOpus tries to extract duration and generate a simple waveform from an Ogg Opus file
func analyzeOggOpus(data []byte) (duration uint32, waveform []byte, err error) {
	// Try to detect if this is a valid Ogg file by checking for the "OggS" signature
	// at the beginning of the file
	if len(data) < 4 || string(data[0:4]) != "OggS" {
		return 0, nil, fmt.Errorf("not a valid Ogg file (missing OggS signature)")
	}

	// Parse Ogg pages to find the last page with a valid granule position
	var lastGranule uint64
	var sampleRate uint32 = 48000 // Default Opus sample rate
	var preSkip uint16 = 0
	var foundOpusHead bool

	// Scan through the file looking for Ogg pages
	for i := 0; i < len(data); {
		// Check if we have enough data to read Ogg page header
		if i+27 >= len(data) {
			break
		}

		// Verify Ogg page signature
		if string(data[i:i+4]) != "OggS" {
			// Skip until next potential page
			i++
			continue
		}

		// Extract header fields
		granulePos := binary.LittleEndian.Uint64(data[i+6 : i+14])
		pageSeqNum := binary.LittleEndian.Uint32(data[i+18 : i+22])
		numSegments := int(data[i+26])

		// Extract segment table
		if i+27+numSegments >= len(data) {
			break
		}
		segmentTable := data[i+27 : i+27+numSegments]

		// Calculate page size
		pageSize := 27 + numSegments
		for _, segLen := range segmentTable {
			pageSize += int(segLen)
		}

		// Check if we're looking at an OpusHead packet (should be in first few pages)
		if !foundOpusHead && pageSeqNum <= 1 {
			// Look for "OpusHead" marker in this page
			pageData := data[i : i+pageSize]
			headPos := bytes.Index(pageData, []byte("OpusHead"))
			if headPos >= 0 && headPos+12 < len(pageData) {
				// Found OpusHead, extract sample rate and pre-skip
				// OpusHead format: Magic(8) + Version(1) + Channels(1) + PreSkip(2) + SampleRate(4) + ...
				headPos += 8 // Skip "OpusHead" marker
				// PreSkip is 2 bytes at offset 10
				if headPos+12 <= len(pageData) {
					preSkip = binary.LittleEndian.Uint16(pageData[headPos+10 : headPos+12])
					sampleRate = binary.LittleEndian.Uint32(pageData[headPos+12 : headPos+16])
					foundOpusHead = true
					fmt.Printf("Found OpusHead: sampleRate=%d, preSkip=%d\n", sampleRate, preSkip)
				}
			}
		}

		// Keep track of last valid granule position
		if granulePos != 0 {
			lastGranule = granulePos
		}

		// Move to next page
		i += pageSize
	}

	if !foundOpusHead {
		fmt.Println("Warning: OpusHead not found, using default values")
	}

	// Calculate duration based on granule position
	if lastGranule > 0 {
		// Formula for duration: (lastGranule - preSkip) / sampleRate
		durationSeconds := float64(lastGranule-uint64(preSkip)) / float64(sampleRate)
		duration = uint32(math.Ceil(durationSeconds))
		fmt.Printf("Calculated Opus duration from granule: %f seconds (lastGranule=%d)\n",
			durationSeconds, lastGranule)
	} else {
		// Fallback to rough estimation if granule position not found
		fmt.Println("Warning: No valid granule position found, using estimation")
		durationEstimate := float64(len(data)) / 2000.0 // Very rough approximation
		duration = uint32(durationEstimate)
	}

	// Make sure we have a reasonable duration (at least 1 second, at most 300 seconds)
	if duration < 1 {
		duration = 1
	} else if duration > 300 {
		duration = 300
	}

	// Generate waveform
	waveform = placeholderWaveform(duration)

	fmt.Printf("Ogg Opus analysis: size=%d bytes, calculated duration=%d sec, waveform=%d bytes\n",
		len(data), duration, len(waveform))

	return duration, waveform, nil
}

// min returns the smaller of x or y
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// placeholderWaveform generates a synthetic waveform for WhatsApp voice messages
// that appears natural with some variability based on the duration
func placeholderWaveform(duration uint32) []byte {
	// WhatsApp expects a 64-byte waveform for voice messages
	const waveformLength = 64
	waveform := make([]byte, waveformLength)

	// Seed the random number generator for consistent results with the same duration
	rand.Seed(int64(duration))

	// Create a more natural looking waveform with some patterns and variability
	// rather than completely random values

	// Base amplitude and frequency - longer messages get faster frequency
	baseAmplitude := 35.0
	frequencyFactor := float64(min(int(duration), 120)) / 30.0

	for i := range waveform {
		// Position in the waveform (normalized 0-1)
		pos := float64(i) / float64(waveformLength)

		// Create a wave pattern with some randomness
		// Use multiple sine waves of different frequencies for more natural look
		val := baseAmplitude * math.Sin(pos*math.Pi*frequencyFactor*8)
		val += (baseAmplitude / 2) * math.Sin(pos*math.Pi*frequencyFactor*16)

		// Add some randomness to make it look more natural
		val += (rand.Float64() - 0.5) * 15

		// Add some fade-in and fade-out effects
		fadeInOut := math.Sin(pos * math.Pi)
		val = val * (0.7 + 0.3*fadeInOut)

		// Center around 50 (typical voice baseline)
		val = val + 50

		// Ensure values stay within WhatsApp's expected range (0-100)
		if val < 0 {
			val = 0
		} else if val > 100 {
			val = 100
		}

		waveform[i] = byte(val)
	}

	return waveform
}

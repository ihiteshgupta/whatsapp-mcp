package bridge

import (
	"context"
)

// WhatsAppClient defines the interface for WhatsApp operations.
// This allows for easy mocking in tests.
type WhatsAppClient interface {
	Connect(ctx context.Context) error
	Disconnect()
	IsConnected() bool
	IsLoggedIn() bool

	// Messaging
	SendMessage(ctx context.Context, jid string, text string) (string, error)
	ReplyToMessage(ctx context.Context, chatJID, messageID, text string) (string, error)
	ForwardMessage(ctx context.Context, sourceChatJID, messageID, targetJID string) (string, error)
	EditMessage(ctx context.Context, chatJID, messageID, newContent string) error
	DeleteMessage(ctx context.Context, chatJID, messageID string, forEveryone bool) error
	ReactToMessage(ctx context.Context, chatJID, messageID, emoji string) error

	// Media
	SendImage(ctx context.Context, jid, imagePath, caption string) (string, error)
	SendVideo(ctx context.Context, jid, videoPath, caption string) (string, error)
	SendAudio(ctx context.Context, jid, audioPath string, asVoice bool) (string, error)
	SendDocument(ctx context.Context, jid, filePath, filename string) (string, error)
	SendLocation(ctx context.Context, jid string, lat, lon float64, name, address string) (string, error)
	SendContactCard(ctx context.Context, jid, contactJID string) (string, error)
	DownloadMedia(ctx context.Context, chatJID, messageID, savePath string) (string, error)

	// Chats
	ArchiveChat(ctx context.Context, jid string, archive bool) error
	PinChat(ctx context.Context, jid string, pin bool) error
	MuteChat(ctx context.Context, jid string, mute bool, duration string) error
	MarkChatRead(ctx context.Context, jid string) error
	DeleteChat(ctx context.Context, jid string) error

	// Contacts
	BlockContact(ctx context.Context, jid string, block bool) error
	CheckPhoneRegistered(ctx context.Context, phone string) (bool, error)

	// Groups
	CreateGroup(ctx context.Context, name string, participants []string) (string, error)
	GetGroupInfo(ctx context.Context, jid string) (interface{}, error)
	LeaveGroup(ctx context.Context, jid string) error
	AddGroupMembers(ctx context.Context, groupJID string, participants []string) error
	RemoveGroupMembers(ctx context.Context, groupJID string, participants []string) error
	PromoteAdmin(ctx context.Context, groupJID string, participants []string) error
	DemoteAdmin(ctx context.Context, groupJID string, participants []string) error
	SetGroupName(ctx context.Context, groupJID, name string) error
	SetGroupTopic(ctx context.Context, groupJID, topic string) error
	SetGroupPhoto(ctx context.Context, groupJID, imagePath string) error
	GetInviteLink(ctx context.Context, groupJID string) (string, error)
	RevokeInviteLink(ctx context.Context, groupJID string) (string, error)
	JoinViaInvite(ctx context.Context, inviteLink string) (string, error)

	// Presence
	SubscribePresence(ctx context.Context, jid string) error
	SendTyping(ctx context.Context, jid string) error
	SendRecording(ctx context.Context, jid string) error
	SetOnline(ctx context.Context) error
	SetOffline(ctx context.Context) error

	// Status
	PostTextStatus(ctx context.Context, text, backgroundColor string) error
	PostImageStatus(ctx context.Context, imagePath, caption string) error
	DeleteStatus(ctx context.Context, statusID string) error

	GetQRChannel() <-chan string

	// Event handling
	AddEventHandler(handler func(interface{}))
}

// SendMessageResult contains the result of sending a message.
type SendMessageResult struct {
	ID        string
	Timestamp int64
}

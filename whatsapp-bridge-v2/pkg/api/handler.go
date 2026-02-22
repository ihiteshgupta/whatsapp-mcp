package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/health"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/store"
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Bridge defines the interface for WhatsApp bridge operations.
type Bridge interface {
	// State
	CurrentState() state.State
	IsReady() bool

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
}

// Handler implements the MCP ToolHandler interface.
type Handler struct {
	store   *store.SQLiteStore
	health  *health.Monitor
	bridge  Bridge
	stateM  *state.Machine
}

// NewHandler creates a new tool handler.
func NewHandler(storeDB *store.SQLiteStore, health *health.Monitor, bridge Bridge, stateM *state.Machine) *Handler {
	return &Handler{
		store:  storeDB,
		health: health,
		bridge: bridge,
		stateM: stateM,
	}
}

// GetTools returns all available tool definitions.
func (h *Handler) GetTools() []mcp.Tool {
	return GetAllTools()
}

// HandleTool handles a tool invocation and returns the result.
func (h *Handler) HandleTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Check bridge state for tools that require ready state
	if requiresReady(name) && (h.bridge == nil || !h.bridge.IsReady()) {
		currentState := "disconnected"
		if h.bridge != nil {
			currentState = string(h.bridge.CurrentState())
		}
		return h.errorResult(NewNotReadyError(currentState))
	}

	switch name {
	// Bridge
	case ToolGetBridgeStatus:
		return h.handleGetBridgeStatus(ctx, args)
	case ToolGetConnectionHistory:
		return h.handleGetConnectionHistory(ctx, args)

	// Chats
	case ToolListChats:
		return h.handleListChats(ctx, args)
	case ToolGetChat:
		return h.handleGetChat(ctx, args)
	case ToolListMessages:
		return h.handleListMessages(ctx, args)
	case ToolArchiveChat, ToolUnarchiveChat:
		return h.handleArchiveChat(ctx, args, name == ToolArchiveChat)
	case ToolPinChat, ToolUnpinChat:
		return h.handlePinChat(ctx, args, name == ToolPinChat)
	case ToolMuteChat, ToolUnmuteChat:
		return h.handleMuteChat(ctx, args, name == ToolMuteChat)
	case ToolMarkChatRead:
		return h.handleMarkChatRead(ctx, args)
	case ToolDeleteChat:
		return h.handleDeleteChat(ctx, args)

	// Contacts
	case ToolSearchContacts:
		return h.handleSearchContacts(ctx, args)
	case ToolGetContact:
		return h.handleGetContact(ctx, args)
	case ToolBlockContact, ToolUnblockContact:
		return h.handleBlockContact(ctx, args, name == ToolBlockContact)
	case ToolGetBlockedContacts:
		return h.handleGetBlockedContacts(ctx, args)
	case ToolCheckPhoneRegistered:
		return h.handleCheckPhoneRegistered(ctx, args)

	// Messaging
	case ToolSendMessage:
		return h.handleSendMessage(ctx, args)
	case ToolReplyToMessage:
		return h.handleReplyToMessage(ctx, args)
	case ToolForwardMessage:
		return h.handleForwardMessage(ctx, args)
	case ToolEditMessage:
		return h.handleEditMessage(ctx, args)
	case ToolDeleteMessage:
		return h.handleDeleteMessage(ctx, args)
	case ToolReactToMessage:
		return h.handleReactToMessage(ctx, args)
	case ToolStarMessage, ToolUnstarMessage:
		return h.handleStarMessage(ctx, args, name == ToolStarMessage)

	// Groups
	case ToolCreateGroup:
		return h.handleCreateGroup(ctx, args)
	case ToolGetGroupInfo:
		return h.handleGetGroupInfo(ctx, args)
	case ToolLeaveGroup:
		return h.handleLeaveGroup(ctx, args)
	case ToolAddGroupMembers:
		return h.handleAddGroupMembers(ctx, args)
	case ToolRemoveGroupMembers:
		return h.handleRemoveGroupMembers(ctx, args)
	case ToolPromoteAdmin:
		return h.handlePromoteAdmin(ctx, args)
	case ToolDemoteAdmin:
		return h.handleDemoteAdmin(ctx, args)
	case ToolSetGroupName:
		return h.handleSetGroupName(ctx, args)
	case ToolSetGroupTopic:
		return h.handleSetGroupTopic(ctx, args)
	case ToolSetGroupPhoto:
		return h.handleSetGroupPhoto(ctx, args)
	case ToolGetInviteLink:
		return h.handleGetInviteLink(ctx, args)
	case ToolRevokeInviteLink:
		return h.handleRevokeInviteLink(ctx, args)
	case ToolJoinViaInvite:
		return h.handleJoinViaInvite(ctx, args)

	// Media
	case ToolSendImage:
		return h.handleSendImage(ctx, args)
	case ToolSendVideo:
		return h.handleSendVideo(ctx, args)
	case ToolSendAudio:
		return h.handleSendAudio(ctx, args)
	case ToolSendDocument:
		return h.handleSendDocument(ctx, args)
	case ToolSendLocation:
		return h.handleSendLocation(ctx, args)
	case ToolSendContactCard:
		return h.handleSendContactCard(ctx, args)
	case ToolDownloadMedia:
		return h.handleDownloadMedia(ctx, args)

	// Presence
	case ToolSubscribePresence:
		return h.handleSubscribePresence(ctx, args)
	case ToolSendTyping:
		return h.handleSendTyping(ctx, args)
	case ToolSendRecording:
		return h.handleSendRecording(ctx, args)
	case ToolSetOnline:
		return h.handleSetOnline(ctx, args)
	case ToolSetOffline:
		return h.handleSetOffline(ctx, args)

	// Status
	case ToolPostTextStatus:
		return h.handlePostTextStatus(ctx, args)
	case ToolPostImageStatus:
		return h.handlePostImageStatus(ctx, args)
	case ToolGetStatusUpdates:
		return h.handleGetStatusUpdates(ctx, args)
	case ToolDeleteStatus:
		return h.handleDeleteStatus(ctx, args)

	default:
		return h.errorResult(NewInvalidInputError(fmt.Sprintf("Unknown tool: %s", name)))
	}
}

// requiresReady returns true if the tool requires the bridge to be in Ready state.
func requiresReady(name string) bool {
	// These tools can work without ready state
	switch name {
	case ToolGetBridgeStatus, ToolGetConnectionHistory, ToolListChats, ToolGetChat,
		ToolListMessages, ToolSearchContacts, ToolGetContact, ToolGetBlockedContacts:
		return false
	default:
		return true
	}
}

// Helper methods

func (h *Handler) successResult(data interface{}) (*mcp.CallToolResult, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, NewInternalError(err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.ContentBlock{mcp.TextContent(string(jsonData))},
	}, nil
}

func (h *Handler) errorResult(err *MCPError) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.ContentBlock{mcp.TextContent(err.JSON())},
		IsError: true,
	}, nil
}

func getString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func getInt(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	if v, ok := args[key].(int); ok {
		return v
	}
	return defaultVal
}

func getBool(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return defaultVal
}

func getFloat(args map[string]interface{}, key string) float64 {
	if v, ok := args[key].(float64); ok {
		return v
	}
	return 0
}

func getStringArray(args map[string]interface{}, key string) []string {
	if v, ok := args[key].([]interface{}); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

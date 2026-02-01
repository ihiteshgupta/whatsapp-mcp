package api

// Tool names
const (
	// Messaging
	ToolSendMessage    = "send_message"
	ToolReplyMessage   = "reply_message"
	ToolForwardMessage = "forward_message"
	ToolEditMessage    = "edit_message"
	ToolDeleteMessage  = "delete_message"
	ToolReactToMessage = "react_to_message"

	// Chats
	ToolListChats   = "list_chats"
	ToolGetChat     = "get_chat"
	ToolArchiveChat = "archive_chat"
	ToolPinChat     = "pin_chat"
	ToolMuteChat    = "mute_chat"
	ToolMarkRead    = "mark_read"

	// Contacts
	ToolSearchContacts = "search_contacts"
	ToolGetContact     = "get_contact"
	ToolBlockContact   = "block_contact"
	ToolGetBlocked     = "get_blocked"

	// Groups
	ToolCreateGroup    = "create_group"
	ToolGetGroupInfo   = "get_group_info"
	ToolAddMembers     = "add_members"
	ToolRemoveMembers  = "remove_members"
	ToolSetGroupName   = "set_group_name"
	ToolSetGroupTopic  = "set_group_topic"
	ToolLeaveGroup     = "leave_group"
	ToolGetInviteLink  = "get_invite_link"

	// Media
	ToolSendFile     = "send_file"
	ToolSendAudio    = "send_audio"
	ToolDownloadMedia = "download_media"
	ToolSendLocation = "send_location"

	// Status
	ToolGetBridgeStatus      = "get_bridge_status"
	ToolGetConnectionHistory = "get_connection_history"
)

// ToolDefinition describes an MCP tool.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// GetToolDefinitions returns all available tool definitions.
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        ToolSendMessage,
			Description: "Send a text message to a WhatsApp contact or group",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"recipient": map[string]interface{}{
						"type":        "string",
						"description": "Phone number or JID of the recipient",
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Text message to send",
					},
				},
				"required": []string{"recipient", "message"},
			},
		},
		{
			Name:        ToolListChats,
			Description: "List all available WhatsApp chats with metadata",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of chats to return",
						"default":     50,
					},
				},
			},
		},
		{
			Name:        ToolGetChat,
			Description: "Get details of a specific chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": map[string]interface{}{
						"type":        "string",
						"description": "JID of the chat",
					},
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolSearchContacts,
			Description: "Search contacts by name or phone number",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        ToolGetBridgeStatus,
			Description: "Get the current health status of the WhatsApp bridge",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        ToolGetConnectionHistory,
			Description: "Get the state transition history of the bridge",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of transitions to return",
						"default":     20,
					},
				},
			},
		},
		{
			Name:        ToolArchiveChat,
			Description: "Archive or unarchive a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": map[string]interface{}{
						"type":        "string",
						"description": "JID of the chat",
					},
					"archive": map[string]interface{}{
						"type":        "boolean",
						"description": "True to archive, false to unarchive",
					},
				},
				"required": []string{"jid", "archive"},
			},
		},
		{
			Name:        ToolBlockContact,
			Description: "Block or unblock a contact",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": map[string]interface{}{
						"type":        "string",
						"description": "JID of the contact",
					},
					"block": map[string]interface{}{
						"type":        "boolean",
						"description": "True to block, false to unblock",
					},
				},
				"required": []string{"jid", "block"},
			},
		},
	}
}

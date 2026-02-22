package api

import (
	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/pkg/mcp"
)

// Tool name constants
const (
	// Messaging (8)
	ToolSendMessage    = "send_message"
	ToolReplyToMessage = "reply_to_message"
	ToolForwardMessage = "forward_message"
	ToolEditMessage    = "edit_message"
	ToolDeleteMessage  = "delete_message"
	ToolReactToMessage = "react_to_message"
	ToolStarMessage    = "star_message"
	ToolUnstarMessage  = "unstar_message"

	// Chats (10)
	ToolListChats     = "list_chats"
	ToolGetChat       = "get_chat"
	ToolListMessages  = "list_messages"
	ToolArchiveChat   = "archive_chat"
	ToolUnarchiveChat = "unarchive_chat"
	ToolPinChat       = "pin_chat"
	ToolUnpinChat     = "unpin_chat"
	ToolMuteChat      = "mute_chat"
	ToolUnmuteChat    = "unmute_chat"
	ToolMarkChatRead  = "mark_chat_read"
	ToolDeleteChat    = "delete_chat"

	// Contacts (6)
	ToolSearchContacts       = "search_contacts"
	ToolGetContact           = "get_contact"
	ToolBlockContact         = "block_contact"
	ToolUnblockContact       = "unblock_contact"
	ToolGetBlockedContacts   = "get_blocked_contacts"
	ToolCheckPhoneRegistered = "check_phone_registered"

	// Groups (13)
	ToolCreateGroup        = "create_group"
	ToolGetGroupInfo       = "get_group_info"
	ToolLeaveGroup         = "leave_group"
	ToolAddGroupMembers    = "add_group_members"
	ToolRemoveGroupMembers = "remove_group_members"
	ToolPromoteAdmin       = "promote_admin"
	ToolDemoteAdmin        = "demote_admin"
	ToolSetGroupName       = "set_group_name"
	ToolSetGroupTopic      = "set_group_topic"
	ToolSetGroupPhoto      = "set_group_photo"
	ToolGetInviteLink      = "get_invite_link"
	ToolRevokeInviteLink   = "revoke_invite_link"
	ToolJoinViaInvite      = "join_via_invite"

	// Media (7)
	ToolSendImage       = "send_image"
	ToolSendVideo       = "send_video"
	ToolSendAudio       = "send_audio"
	ToolSendDocument    = "send_document"
	ToolSendLocation    = "send_location"
	ToolSendContactCard = "send_contact_card"
	ToolDownloadMedia   = "download_media"

	// Presence (5)
	ToolSubscribePresence = "subscribe_presence"
	ToolSendTyping        = "send_typing"
	ToolSendRecording     = "send_recording"
	ToolSetOnline         = "set_online"
	ToolSetOffline        = "set_offline"

	// Status (4)
	ToolPostTextStatus   = "post_text_status"
	ToolPostImageStatus  = "post_image_status"
	ToolGetStatusUpdates = "get_status_updates"
	ToolDeleteStatus     = "delete_status"

	// Bridge (2)
	ToolGetBridgeStatus      = "get_bridge_status"
	ToolGetConnectionHistory = "get_connection_history"
)

// GetAllTools returns all 55 tool definitions.
func GetAllTools() []mcp.Tool {
	return []mcp.Tool{
		// ============ MESSAGING (8) ============
		{
			Name:        ToolSendMessage,
			Description: "Send a text message to a WhatsApp contact or group",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"recipient": prop("string", "Phone number (e.g., +1234567890) or JID of the recipient"),
					"message":   prop("string", "Text message to send"),
				},
				"required": []string{"recipient", "message"},
			},
		},
		{
			Name:        ToolReplyToMessage,
			Description: "Reply to a specific message in a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"chat_jid":   prop("string", "JID of the chat"),
					"message_id": prop("string", "ID of the message to reply to"),
					"message":    prop("string", "Reply message text"),
				},
				"required": []string{"chat_jid", "message_id", "message"},
			},
		},
		{
			Name:        ToolForwardMessage,
			Description: "Forward a message to another chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source_chat_jid": prop("string", "JID of the source chat"),
					"message_id":      prop("string", "ID of the message to forward"),
					"target_jid":      prop("string", "JID of the target chat"),
				},
				"required": []string{"source_chat_jid", "message_id", "target_jid"},
			},
		},
		{
			Name:        ToolEditMessage,
			Description: "Edit a previously sent message",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"chat_jid":    prop("string", "JID of the chat"),
					"message_id":  prop("string", "ID of the message to edit"),
					"new_content": prop("string", "New message content"),
				},
				"required": []string{"chat_jid", "message_id", "new_content"},
			},
		},
		{
			Name:        ToolDeleteMessage,
			Description: "Delete a message (for me or for everyone)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"chat_jid":     prop("string", "JID of the chat"),
					"message_id":   prop("string", "ID of the message to delete"),
					"for_everyone": propBool("Delete for everyone (true) or just for me (false)"),
				},
				"required": []string{"chat_jid", "message_id"},
			},
		},
		{
			Name:        ToolReactToMessage,
			Description: "Add an emoji reaction to a message",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"chat_jid":   prop("string", "JID of the chat"),
					"message_id": prop("string", "ID of the message"),
					"emoji":      prop("string", "Emoji reaction (e.g., '👍', '❤️', '😂')"),
				},
				"required": []string{"chat_jid", "message_id", "emoji"},
			},
		},
		{
			Name:        ToolStarMessage,
			Description: "Star a message for later reference",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"chat_jid":   prop("string", "JID of the chat"),
					"message_id": prop("string", "ID of the message to star"),
				},
				"required": []string{"chat_jid", "message_id"},
			},
		},
		{
			Name:        ToolUnstarMessage,
			Description: "Remove star from a message",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"chat_jid":   prop("string", "JID of the chat"),
					"message_id": prop("string", "ID of the message to unstar"),
				},
				"required": []string{"chat_jid", "message_id"},
			},
		},

		// ============ CHATS (10) ============
		{
			Name:        ToolListChats,
			Description: "List all WhatsApp chats with metadata",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit":         propInt("Maximum number of chats to return (default: 50)"),
					"include_muted": propBool("Include muted chats (default: true)"),
				},
			},
		},
		{
			Name:        ToolGetChat,
			Description: "Get details of a specific chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolListMessages,
			Description: "Get messages from a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"chat_jid": prop("string", "JID of the chat"),
					"limit":    propInt("Maximum number of messages to return (default: 50)"),
					"before":   prop("string", "Message ID to fetch messages before (for pagination)"),
				},
				"required": []string{"chat_jid"},
			},
		},
		{
			Name:        ToolArchiveChat,
			Description: "Archive a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat to archive"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolUnarchiveChat,
			Description: "Unarchive a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat to unarchive"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolPinChat,
			Description: "Pin a chat to the top",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat to pin"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolUnpinChat,
			Description: "Unpin a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat to unpin"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolMuteChat,
			Description: "Mute notifications for a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid":      prop("string", "JID of the chat to mute"),
					"duration": prop("string", "Duration to mute (e.g., '8h', '1w', 'forever')"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolUnmuteChat,
			Description: "Unmute notifications for a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat to unmute"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolMarkChatRead,
			Description: "Mark all messages in a chat as read",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolDeleteChat,
			Description: "Delete a chat locally",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat to delete"),
				},
				"required": []string{"jid"},
			},
		},

		// ============ CONTACTS (6) ============
		{
			Name:        ToolSearchContacts,
			Description: "Search contacts by name or phone number",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": prop("string", "Search query (name or phone number)"),
					"limit": propInt("Maximum number of results (default: 20)"),
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        ToolGetContact,
			Description: "Get details of a specific contact",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the contact"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolBlockContact,
			Description: "Block a contact",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the contact to block"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolUnblockContact,
			Description: "Unblock a contact",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the contact to unblock"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolGetBlockedContacts,
			Description: "Get list of blocked contacts",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        ToolCheckPhoneRegistered,
			Description: "Check if a phone number is registered on WhatsApp",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"phone": prop("string", "Phone number to check (e.g., +1234567890)"),
				},
				"required": []string{"phone"},
			},
		},

		// ============ GROUPS (13) ============
		{
			Name:        ToolCreateGroup,
			Description: "Create a new WhatsApp group",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":         prop("string", "Group name"),
					"participants": propArray("string", "List of participant JIDs to add"),
				},
				"required": []string{"name", "participants"},
			},
		},
		{
			Name:        ToolGetGroupInfo,
			Description: "Get information about a group",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the group"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolLeaveGroup,
			Description: "Leave a group",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the group to leave"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolAddGroupMembers,
			Description: "Add members to a group",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"group_jid":    prop("string", "JID of the group"),
					"participants": propArray("string", "List of participant JIDs to add"),
				},
				"required": []string{"group_jid", "participants"},
			},
		},
		{
			Name:        ToolRemoveGroupMembers,
			Description: "Remove members from a group",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"group_jid":    prop("string", "JID of the group"),
					"participants": propArray("string", "List of participant JIDs to remove"),
				},
				"required": []string{"group_jid", "participants"},
			},
		},
		{
			Name:        ToolPromoteAdmin,
			Description: "Promote members to group admin",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"group_jid":    prop("string", "JID of the group"),
					"participants": propArray("string", "List of participant JIDs to promote"),
				},
				"required": []string{"group_jid", "participants"},
			},
		},
		{
			Name:        ToolDemoteAdmin,
			Description: "Demote admins to regular members",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"group_jid":    prop("string", "JID of the group"),
					"participants": propArray("string", "List of admin JIDs to demote"),
				},
				"required": []string{"group_jid", "participants"},
			},
		},
		{
			Name:        ToolSetGroupName,
			Description: "Change group name",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"group_jid": prop("string", "JID of the group"),
					"name":      prop("string", "New group name"),
				},
				"required": []string{"group_jid", "name"},
			},
		},
		{
			Name:        ToolSetGroupTopic,
			Description: "Change group topic/description",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"group_jid": prop("string", "JID of the group"),
					"topic":     prop("string", "New group topic"),
				},
				"required": []string{"group_jid", "topic"},
			},
		},
		{
			Name:        ToolSetGroupPhoto,
			Description: "Change group profile photo",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"group_jid":  prop("string", "JID of the group"),
					"image_path": prop("string", "Path to the image file"),
				},
				"required": []string{"group_jid", "image_path"},
			},
		},
		{
			Name:        ToolGetInviteLink,
			Description: "Get group invite link",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"group_jid": prop("string", "JID of the group"),
				},
				"required": []string{"group_jid"},
			},
		},
		{
			Name:        ToolRevokeInviteLink,
			Description: "Revoke current group invite link",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"group_jid": prop("string", "JID of the group"),
				},
				"required": []string{"group_jid"},
			},
		},
		{
			Name:        ToolJoinViaInvite,
			Description: "Join a group via invite link",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"invite_link": prop("string", "Group invite link"),
				},
				"required": []string{"invite_link"},
			},
		},

		// ============ MEDIA (7) ============
		{
			Name:        ToolSendImage,
			Description: "Send an image to a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"recipient":  prop("string", "Phone number or JID of the recipient"),
					"image_path": prop("string", "Path to the image file"),
					"caption":    prop("string", "Optional caption for the image"),
				},
				"required": []string{"recipient", "image_path"},
			},
		},
		{
			Name:        ToolSendVideo,
			Description: "Send a video to a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"recipient":  prop("string", "Phone number or JID of the recipient"),
					"video_path": prop("string", "Path to the video file"),
					"caption":    prop("string", "Optional caption for the video"),
				},
				"required": []string{"recipient", "video_path"},
			},
		},
		{
			Name:        ToolSendAudio,
			Description: "Send an audio file or voice message",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"recipient":  prop("string", "Phone number or JID of the recipient"),
					"audio_path": prop("string", "Path to the audio file"),
					"as_voice":   propBool("Send as voice message (true) or audio file (false)"),
				},
				"required": []string{"recipient", "audio_path"},
			},
		},
		{
			Name:        ToolSendDocument,
			Description: "Send a document to a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"recipient": prop("string", "Phone number or JID of the recipient"),
					"file_path": prop("string", "Path to the document file"),
					"filename":  prop("string", "Optional filename to display"),
				},
				"required": []string{"recipient", "file_path"},
			},
		},
		{
			Name:        ToolSendLocation,
			Description: "Send a location to a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"recipient": prop("string", "Phone number or JID of the recipient"),
					"latitude":  propNumber("Latitude coordinate"),
					"longitude": propNumber("Longitude coordinate"),
					"name":      prop("string", "Optional location name"),
					"address":   prop("string", "Optional address"),
				},
				"required": []string{"recipient", "latitude", "longitude"},
			},
		},
		{
			Name:        ToolSendContactCard,
			Description: "Send a contact card to a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"recipient":   prop("string", "Phone number or JID of the recipient"),
					"contact_jid": prop("string", "JID of the contact to share"),
				},
				"required": []string{"recipient", "contact_jid"},
			},
		},
		{
			Name:        ToolDownloadMedia,
			Description: "Download media from a message",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"chat_jid":   prop("string", "JID of the chat"),
					"message_id": prop("string", "ID of the message containing media"),
					"save_path":  prop("string", "Path to save the downloaded file"),
				},
				"required": []string{"chat_jid", "message_id"},
			},
		},

		// ============ PRESENCE (5) ============
		{
			Name:        ToolSubscribePresence,
			Description: "Subscribe to presence updates for a contact",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the contact to subscribe to"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolSendTyping,
			Description: "Send typing indicator to a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolSendRecording,
			Description: "Send recording indicator to a chat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"jid": prop("string", "JID of the chat"),
				},
				"required": []string{"jid"},
			},
		},
		{
			Name:        ToolSetOnline,
			Description: "Set presence to online",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        ToolSetOffline,
			Description: "Set presence to offline",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},

		// ============ STATUS (4) ============
		{
			Name:        ToolPostTextStatus,
			Description: "Post a text status update",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text":             prop("string", "Status text"),
					"background_color": prop("string", "Background color hex code (e.g., #FF5733)"),
				},
				"required": []string{"text"},
			},
		},
		{
			Name:        ToolPostImageStatus,
			Description: "Post an image status update",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"image_path": prop("string", "Path to the image file"),
					"caption":    prop("string", "Optional caption"),
				},
				"required": []string{"image_path"},
			},
		},
		{
			Name:        ToolGetStatusUpdates,
			Description: "Get status updates from contacts",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"contact_jid": prop("string", "Optional: JID to get status from specific contact"),
				},
			},
		},
		{
			Name:        ToolDeleteStatus,
			Description: "Delete a status update",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status_id": prop("string", "ID of the status to delete"),
				},
				"required": []string{"status_id"},
			},
		},

		// ============ BRIDGE (2) ============
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
					"limit": propInt("Maximum number of transitions to return (default: 20)"),
				},
			},
		},
	}
}

// Helper functions for schema creation
func prop(typeName, description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        typeName,
		"description": description,
	}
}

func propInt(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "integer",
		"description": description,
	}
}

func propNumber(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "number",
		"description": description,
	}
}

func propBool(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "boolean",
		"description": description,
	}
}

func propArray(itemType, description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": description,
		"items": map[string]interface{}{
			"type": itemType,
		},
	}
}

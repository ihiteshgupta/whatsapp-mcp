# WhatsApp MCP Server

MCP server for WhatsApp using whatsmeow Go library for direct WhatsApp Web API access.

## Versions

### V2 (Recommended) - Single Binary

Production-grade implementation with 55 tools, FSM state management, native MCP stdio transport.

**Location:** `./whatsapp-bridge-v2/`

**Features:**
- Single Go binary (no Python dependency)
- 55 MCP tools (messaging, media, groups, presence, status)
- FSM-based connection lifecycle
- Self-healing with exponential backoff
- Full SQLite storage (messages, chats, contacts, groups, status)

**Setup:**
```bash
cd whatsapp-bridge-v2
go build -o whatsapp-bridge ./cmd/bridge
```

**Claude Code config (`~/.claude.json`):**
```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "/Users/hitesh.gupta/personal-projects/whatsapp-mcp/whatsapp-bridge-v2/whatsapp-bridge",
      "args": []
    }
  }
}
```

### V1 (Legacy) - Two-Component

Original implementation with Go bridge + Python MCP server.

**Location:** `./whatsapp-bridge/` + `./whatsapp-mcp-server/`

## V2 Tools (55 total)

### Messaging (8)
send_message, reply_to_message, forward_message, edit_message, delete_message, react_to_message, star_message, unstar_message

### Chats (10)
list_chats, get_chat, list_messages, archive_chat, unarchive_chat, pin_chat, unpin_chat, mute_chat, unmute_chat, mark_chat_read, delete_chat

### Contacts (6)
search_contacts, get_contact, block_contact, unblock_contact, get_blocked_contacts, check_phone_registered

### Groups (13)
create_group, get_group_info, leave_group, add_group_members, remove_group_members, promote_admin, demote_admin, set_group_name, set_group_topic, set_group_photo, get_invite_link, revoke_invite_link, join_via_invite

### Media (7)
send_image, send_video, send_audio, send_document, send_location, send_contact_card, download_media

### Presence (5)
subscribe_presence, send_typing, send_recording, set_online, set_offline

### Status (4)
post_text_status, post_image_status, get_status_updates, delete_status

### Bridge (2)
get_bridge_status, get_connection_history

## Data Storage (V2)

- Session DB: `whatsapp-bridge-v2/store/whatsapp.db` (whatsmeow session)
- App DB: `whatsapp-bridge-v2/store/messages.db` (messages, chats, contacts, groups)

## First-Time Setup

1. Start Claude Code with WhatsApp MCP configured
2. QR code appears in terminal and saved to `store/qrcode.png`
3. Scan with WhatsApp mobile app
4. Wait for history sync to complete
5. Session persists for ~20 days

## Troubleshooting

- **QR Code not showing**: Check stderr output, or open `store/qrcode.png`
- **Session expired**: Delete `store/whatsapp.db` and re-authenticate
- **Out of sync**: Delete both `.db` files and restart

## Development

```bash
cd whatsapp-bridge-v2
go test ./...       # Run tests
go build ./...      # Build
```

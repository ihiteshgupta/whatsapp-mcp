# WhatsApp MCP

MCP server for WhatsApp using the whatsmeow Go library. Single binary — no Python, no separate bridge process.

## Setup

```bash
brew install ihiteshgupta/tap/whatsapp-mcp
```

**Claude Code config (`~/.claude.json`):**
```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "/opt/homebrew/bin/whatsapp-mcp",
      "args": []
    }
  }
}
```

### Build from source (dev)
```bash
cd whatsapp-bridge-v2
go build -o whatsapp-mcp ./cmd/whatsapp-mcp
```

## Data Storage

All data stored in `~/.whatsapp-mcp/` (created automatically, no config file needed):
- `~/.whatsapp-mcp/whatsapp.db` — WhatsApp session (whatsmeow)
- `~/.whatsapp-mcp/messages.db` — Messages, chats, contacts, groups
- `~/.whatsapp-mcp/qrcode.png` — QR code on first launch

## First-Time Auth

1. Start Claude Code with MCP configured above
2. QR code printed to terminal and saved to `~/.whatsapp-mcp/qrcode.png`
3. Scan with WhatsApp → Settings → Linked Devices → Link a Device
4. Wait for history sync
5. Session persists ~20 days

## Tools (55 total)

### Messaging (8)
send_message, reply_to_message, forward_message, edit_message, delete_message, react_to_message, star_message, unstar_message

### Chats (11)
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

## Troubleshooting

- **QR Code not showing**: Open `~/.whatsapp-mcp/qrcode.png`
- **Session expired**: Delete `~/.whatsapp-mcp/whatsapp.db` and restart
- **Out of sync**: Delete `~/.whatsapp-mcp/*.db` and restart

## Development

```bash
cd whatsapp-bridge-v2
go test ./...
go build ./...
```

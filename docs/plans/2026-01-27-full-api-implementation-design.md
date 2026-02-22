# WhatsApp MCP Full API Implementation Design

## Overview

Implement all missing WhatsApp API features in the whatsapp-mcp project, covering chat management, message features, group management, status/stories, profile/privacy, presence, and utilities.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Python MCP Server                         │
│                  (whatsapp-mcp-server/)                      │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  MCP Tools (FastMCP)                                │    │
│  │  - Chat Management (pin, archive, mute, etc.)       │    │
│  │  - Message Features (react, reply, forward, etc.)   │    │
│  │  - Group Management (create, members, etc.)         │    │
│  │  - Status/Stories (view, post, etc.)                │    │
│  │  - Profile/Privacy (update, block, etc.)            │    │
│  │  - Presence (online, typing, etc.)                  │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │ HTTP                             │
└───────────────────────────┼─────────────────────────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                     Go Bridge                                │
│                  (whatsapp-bridge/)                          │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  REST API Endpoints                                 │    │
│  │  /api/chat/*      - Chat management                 │    │
│  │  /api/message/*   - Message operations              │    │
│  │  /api/group/*     - Group management                │    │
│  │  /api/status/*    - Status/stories                  │    │
│  │  /api/profile/*   - Profile management              │    │
│  │  /api/presence/*  - Presence features               │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  whatsmeow Client                                   │    │
│  │  - Direct WhatsApp protocol communication           │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## Feature Categories

### 1. Chat Management
| Feature | Go Endpoint | MCP Tool | whatsmeow Method |
|---------|-------------|----------|------------------|
| Pin chat | POST /api/chat/pin | pin_chat | client.SendAppState() |
| Unpin chat | POST /api/chat/unpin | unpin_chat | client.SendAppState() |
| Archive chat | POST /api/chat/archive | archive_chat | client.SendAppState() |
| Unarchive chat | POST /api/chat/unarchive | unarchive_chat | client.SendAppState() |
| Mute chat | POST /api/chat/mute | mute_chat | client.SendAppState() |
| Unmute chat | POST /api/chat/unmute | unmute_chat | client.SendAppState() |
| Mark read | POST /api/chat/read | mark_chat_read | client.MarkRead() |
| Mark unread | POST /api/chat/unread | mark_chat_unread | client.SendAppState() |
| Delete chat | POST /api/chat/delete | delete_chat | client.SendAppState() |

### 2. Message Features
| Feature | Go Endpoint | MCP Tool | whatsmeow Method |
|---------|-------------|----------|------------------|
| React to message | POST /api/message/react | react_to_message | client.SendMessage() with ReactionMessage |
| Reply to message | POST /api/message/reply | reply_to_message | client.SendMessage() with ContextInfo |
| Forward message | POST /api/message/forward | forward_message | client.SendMessage() with forwarded flag |
| Delete message | POST /api/message/delete | delete_message | client.SendMessage() with ProtocolMessage |
| Star message | POST /api/message/star | star_message | client.SendAppState() |
| Unstar message | POST /api/message/unstar | unstar_message | client.SendAppState() |
| Edit message | POST /api/message/edit | edit_message | client.SendMessage() with EditedMessage |

### 3. Group Management
| Feature | Go Endpoint | MCP Tool | whatsmeow Method |
|---------|-------------|----------|------------------|
| Create group | POST /api/group/create | create_group | client.CreateGroup() |
| Get group info | GET /api/group/info | get_group_info | client.GetGroupInfo() |
| Add members | POST /api/group/members/add | add_group_members | client.UpdateGroupParticipants() |
| Remove members | POST /api/group/members/remove | remove_group_members | client.UpdateGroupParticipants() |
| Promote to admin | POST /api/group/admin/promote | promote_group_admin | client.UpdateGroupParticipants() |
| Demote admin | POST /api/group/admin/demote | demote_group_admin | client.UpdateGroupParticipants() |
| Update group name | POST /api/group/name | update_group_name | client.SetGroupName() |
| Update group topic | POST /api/group/topic | update_group_topic | client.SetGroupTopic() |
| Update group photo | POST /api/group/photo | update_group_photo | client.SetGroupPhoto() |
| Get invite link | GET /api/group/invite | get_group_invite_link | client.GetGroupInviteLink() |
| Revoke invite link | POST /api/group/invite/revoke | revoke_group_invite | client.RevokeGroupInviteLink() |
| Join via invite | POST /api/group/join | join_group_via_invite | client.JoinGroupWithLink() |
| Leave group | POST /api/group/leave | leave_group | client.LeaveGroup() |

### 4. Status/Stories
| Feature | Go Endpoint | MCP Tool | whatsmeow Method |
|---------|-------------|----------|------------------|
| Get status list | GET /api/status/list | get_status_updates | Event handler for status |
| Post text status | POST /api/status/text | post_text_status | client.SendMessage() to status@broadcast |
| Post image status | POST /api/status/image | post_image_status | client.SendMessage() to status@broadcast |
| Delete status | POST /api/status/delete | delete_status | client.SendMessage() with delete |

### 5. Profile & Privacy
| Feature | Go Endpoint | MCP Tool | whatsmeow Method |
|---------|-------------|----------|------------------|
| Get profile | GET /api/profile | get_profile | client.Store |
| Update profile name | POST /api/profile/name | update_profile_name | client.SetStatusMessage() |
| Update profile photo | POST /api/profile/photo | update_profile_photo | client.SetProfilePhoto() |
| Update about | POST /api/profile/about | update_about | client.SetStatusMessage() |
| Get privacy settings | GET /api/privacy | get_privacy_settings | client.GetPrivacySettings() |
| Update privacy | POST /api/privacy | update_privacy_settings | client.SetPrivacySettings() |
| Block contact | POST /api/contact/block | block_contact | client.UpdateBlocklist() |
| Unblock contact | POST /api/contact/unblock | unblock_contact | client.UpdateBlocklist() |
| Get blocked list | GET /api/contact/blocked | get_blocked_contacts | client.GetBlocklist() |

### 6. Presence & Receipts
| Feature | Go Endpoint | MCP Tool | whatsmeow Method |
|---------|-------------|----------|------------------|
| Subscribe presence | POST /api/presence/subscribe | subscribe_presence | client.SubscribePresence() |
| Send typing | POST /api/presence/typing | send_typing_indicator | client.SendChatPresence() |
| Send recording | POST /api/presence/recording | send_recording_indicator | client.SendChatPresence() |
| Set online | POST /api/presence/online | set_online | client.SendPresence() |
| Set offline | POST /api/presence/offline | set_offline | client.SendPresence() |

### 7. Additional Utilities
| Feature | Go Endpoint | MCP Tool | whatsmeow Method |
|---------|-------------|----------|------------------|
| Share location | POST /api/message/location | send_location | client.SendMessage() with LocationMessage |
| Share contact | POST /api/message/contact | send_contact | client.SendMessage() with ContactMessage |
| Check if on WhatsApp | POST /api/contact/check | check_phone_registered | client.IsOnWhatsApp() |
| Get contact info | GET /api/contact/info | get_contact_info | client.GetContactInfo() |

## Database Schema Updates

```sql
-- Add to existing schema
ALTER TABLE chats ADD COLUMN is_pinned BOOLEAN DEFAULT 0;
ALTER TABLE chats ADD COLUMN is_archived BOOLEAN DEFAULT 0;
ALTER TABLE chats ADD COLUMN is_muted BOOLEAN DEFAULT 0;
ALTER TABLE chats ADD COLUMN mute_until TIMESTAMP;

ALTER TABLE messages ADD COLUMN is_starred BOOLEAN DEFAULT 0;
ALTER TABLE messages ADD COLUMN reply_to_id TEXT;
ALTER TABLE messages ADD COLUMN is_forwarded BOOLEAN DEFAULT 0;
ALTER TABLE messages ADD COLUMN is_edited BOOLEAN DEFAULT 0;

CREATE TABLE IF NOT EXISTS reactions (
    message_id TEXT,
    chat_jid TEXT,
    sender TEXT,
    emoji TEXT,
    timestamp TIMESTAMP,
    PRIMARY KEY (message_id, chat_jid, sender)
);

CREATE TABLE IF NOT EXISTS status_updates (
    id TEXT PRIMARY KEY,
    sender TEXT,
    content TEXT,
    media_type TEXT,
    media_url TEXT,
    timestamp TIMESTAMP,
    expires_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS blocked_contacts (
    jid TEXT PRIMARY KEY,
    blocked_at TIMESTAMP
);
```

## Implementation Order

1. **Phase 1: Message Features** - React, reply, forward, delete, star, edit
2. **Phase 2: Chat Management** - Pin, archive, mute, mark read, delete
3. **Phase 3: Group Management** - Create, members, admin, info, invite
4. **Phase 4: Profile & Privacy** - Profile, about, block, privacy settings
5. **Phase 5: Presence** - Online, typing, recording indicators
6. **Phase 6: Status/Stories** - View, post, delete status
7. **Phase 7: Utilities** - Location, contact sharing, phone check

## Files to Modify

### Go Bridge
- `whatsapp-bridge/main.go` - Add all new REST endpoints and handlers

### Python MCP Server
- `whatsapp-mcp-server/main.py` - Add all new MCP tools
- `whatsapp-mcp-server/whatsapp.py` - Add HTTP client functions for new endpoints

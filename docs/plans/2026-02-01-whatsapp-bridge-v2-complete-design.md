# WhatsApp Bridge V2 - Complete Implementation Design

**Date:** 2026-02-01
**Status:** Approved for Implementation
**Goal:** Production-grade WhatsApp bridge with native MCP stdio, 55 tools, FSM architecture

## Decisions

| Decision | Choice |
|----------|--------|
| Transport | Native MCP stdio (single Go binary) |
| Whatsmeow | Fresh implementation following V2 patterns |
| Tool scope | Full 55 tools |
| Database | Fresh start, new session |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Claude Code                          │
└────────────────────────┬────────────────────────────────┘
                         │ MCP Protocol (stdio)
┌────────────────────────▼────────────────────────────────┐
│              MCP Transport (pkg/mcp/)                    │
│  - JSON-RPC over stdin/stdout                           │
│  - Tool registration & dispatch                          │
│  - Resource handling                                     │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│              Bridge Core (internal/bridge/)              │
│  ┌──────────┐  ┌───────────────┐  ┌──────────────┐     │
│  │   FSM    │  │ WhatsmeowClient│  │    Health    │     │
│  │ (9 states)│  │  (55 methods)  │  │   Monitor    │     │
│  └──────────┘  └───────────────┘  └──────────────┘     │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│              Storage (internal/store/)                   │
│  Sessions │ Messages │ Chats │ Contacts │ Groups │ State│
│                    SQLite                                │
└─────────────────────────────────────────────────────────┘
```

## Directory Structure

```
whatsapp-bridge-v2/
├── cmd/bridge/
│   └── main.go                 # Entry point
├── pkg/
│   ├── mcp/                    # NEW: MCP transport
│   │   ├── transport.go        # Stdio read/write
│   │   ├── server.go           # MCP server lifecycle
│   │   ├── protocol.go         # JSON-RPC types
│   │   └── handlers.go         # initialize, tools/list, tools/call
│   └── api/
│       ├── mcp.go              # Tool dispatch (existing, expand)
│       ├── tools.go            # 55 tool definitions (existing, expand)
│       └── errors.go           # Structured errors (existing)
├── internal/
│   ├── bridge/
│   │   ├── bridge.go           # Core bridge (existing)
│   │   ├── whatsmeow.go        # NEW: WhatsmeowClient implementation
│   │   ├── events.go           # Event types (existing)
│   │   └── client.go           # Interface (existing)
│   ├── state/                  # FSM (existing, complete)
│   ├── store/
│   │   ├── repository.go       # Interfaces (existing, expand)
│   │   ├── models.go           # Models (existing, expand)
│   │   ├── sqlite.go           # SQLite store (existing, expand)
│   │   ├── messages.go         # NEW: Message repository
│   │   ├── chats.go            # NEW: Chat repository
│   │   ├── contacts.go         # NEW: Contact repository
│   │   ├── groups.go           # NEW: Group repository
│   │   └── status.go           # NEW: Status repository
│   ├── health/                 # Health monitor (existing)
│   └── config/                 # Configuration (existing)
├── migrations/
│   └── 001_complete.sql        # NEW: Full schema
├── config.yaml                 # Runtime config
├── go.mod                      # Add whatsmeow dep
└── README.md                   # Update docs
```

## Tool Catalog (55 Tools)

### Messaging (8)
- send_message, reply_to_message, forward_message, edit_message
- delete_message, react_to_message, star_message, unstar_message

### Chats (10)
- list_chats, get_chat, list_messages, archive_chat, unarchive_chat
- pin_chat, unpin_chat, mute_chat, unmute_chat, mark_chat_read, delete_chat

### Contacts (6)
- search_contacts, get_contact, block_contact, unblock_contact
- get_blocked_contacts, check_phone_registered

### Groups (13)
- create_group, get_group_info, leave_group
- add_group_members, remove_group_members
- promote_admin, demote_admin
- set_group_name, set_group_topic, set_group_photo
- get_invite_link, revoke_invite_link, join_via_invite

### Media (7)
- send_image, send_video, send_audio, send_document
- send_location, send_contact_card, download_media

### Presence (5)
- subscribe_presence, send_typing, send_recording
- set_online, set_offline

### Status (4)
- post_text_status, post_image_status
- get_status_updates, delete_status

### Bridge (2)
- get_bridge_status, get_connection_history

## Database Schema

```sql
-- Session (whatsmeow manages internally)

-- Messages
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    chat_jid TEXT NOT NULL,
    sender TEXT NOT NULL,
    content TEXT,
    timestamp DATETIME NOT NULL,
    is_from_me BOOLEAN DEFAULT FALSE,
    media_type TEXT,
    media_url TEXT,
    media_key BLOB,
    file_sha256 BLOB,
    file_length INTEGER,
    quoted_id TEXT,
    is_starred BOOLEAN DEFAULT FALSE,
    is_deleted BOOLEAN DEFAULT FALSE,
    reactions TEXT,  -- JSON array
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Chats
CREATE TABLE chats (
    jid TEXT PRIMARY KEY,
    name TEXT,
    is_group BOOLEAN DEFAULT FALSE,
    is_archived BOOLEAN DEFAULT FALSE,
    is_pinned BOOLEAN DEFAULT FALSE,
    is_muted BOOLEAN DEFAULT FALSE,
    mute_until DATETIME,
    unread_count INTEGER DEFAULT 0,
    last_message_at DATETIME,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Contacts
CREATE TABLE contacts (
    jid TEXT PRIMARY KEY,
    name TEXT,
    push_name TEXT,
    phone TEXT,
    is_blocked BOOLEAN DEFAULT FALSE,
    is_saved BOOLEAN DEFAULT FALSE,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Groups
CREATE TABLE groups (
    jid TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    topic TEXT,
    created_at DATETIME,
    created_by TEXT,
    invite_link TEXT,
    is_announce BOOLEAN DEFAULT FALSE,
    is_locked BOOLEAN DEFAULT FALSE,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE group_participants (
    group_jid TEXT NOT NULL,
    user_jid TEXT NOT NULL,
    role TEXT DEFAULT 'member',  -- member, admin, superadmin
    joined_at DATETIME,
    PRIMARY KEY (group_jid, user_jid),
    FOREIGN KEY (group_jid) REFERENCES groups(jid)
);

-- Status Updates
CREATE TABLE status_updates (
    id TEXT PRIMARY KEY,
    sender_jid TEXT NOT NULL,
    media_type TEXT,
    content TEXT,
    posted_at DATETIME NOT NULL,
    expires_at DATETIME,
    viewed BOOLEAN DEFAULT FALSE
);

-- FSM State
CREATE TABLE state_current (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    state TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE state_transitions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_state TEXT NOT NULL,
    to_state TEXT NOT NULL,
    trigger TEXT NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_messages_chat ON messages(chat_jid, timestamp DESC);
CREATE INDEX idx_messages_starred ON messages(is_starred) WHERE is_starred = TRUE;
CREATE INDEX idx_chats_last_message ON chats(last_message_at DESC);
CREATE INDEX idx_contacts_blocked ON contacts(is_blocked) WHERE is_blocked = TRUE;
```

## FSM Event Mapping

| whatsmeow Event | FSM Trigger | New State |
|-----------------|-------------|-----------|
| Connected | TriggerConnect | Connecting |
| QR | TriggerQRRequired | QRPending |
| PairSuccess | TriggerAuthenticated | Authenticating |
| HistorySync complete | TriggerSyncComplete | Ready |
| Disconnected | TriggerConnectionLost | Reconnecting |
| LoggedOut | TriggerLogout | LoggedOut |
| StreamError (429) | TriggerBanDetected | TemporaryBan |

## Configuration

```yaml
# Paths
data_dir: ./data
session_file: whatsapp.db
messages_file: messages.db

# Connection
connect_timeout: 30s
history_sync: true
history_days: 30

# Health & Reconnection
keepalive_interval: 30s
reconnect_max_retries: 10
reconnect_base_delay: 1s
reconnect_max_delay: 5m

# Logging
log_level: info
log_format: json

# MCP
mcp_mode: stdio
```

## Implementation Phases

### Phase 1: MCP Transport
- Create pkg/mcp/ with stdio transport
- JSON-RPC message handling
- Wire to existing tool handlers
- Test: Claude Code connects, get_bridge_status works

### Phase 2: WhatsApp Core
- Add whatsmeow to go.mod
- Implement WhatsmeowClient
- Wire events to FSM
- Test: QR display, authentication works

### Phase 3: Storage Expansion
- Full schema migration
- All repository implementations
- Test: data persists correctly

### Phase 4: Core Tools (20)
- Messaging: send, reply, forward, edit, delete, react
- Chats: list, get, archive, pin, mute
- Contacts: search, get, block
- Test: basic operations work

### Phase 5: Extended Tools (35)
- Groups: all 13 tools
- Media: all 7 tools
- Presence: all 5 tools
- Status: all 4 tools
- Test: full functionality

### Phase 6: Polish
- Health monitoring refinements
- Graceful shutdown
- Error handling
- Documentation

## MCP Config for Claude Code

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "/path/to/whatsapp-mcp/whatsapp-bridge-v2/whatsapp-bridge-v2",
      "args": []
    }
  }
}
```

## Success Criteria

1. Single binary, no Python dependency
2. All 55 tools functional
3. FSM manages connection lifecycle
4. Self-healing reconnection
5. Clean QR code flow for new sessions
6. Message history syncs and persists

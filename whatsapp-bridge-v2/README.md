# WhatsApp Bridge V2

Production-grade WhatsApp bridge with complex state management, self-healing, and MCP integration.

## Features

- **Finite State Machine (FSM)** - Full lifecycle management with substates
- **Self-Healing** - Automatic reconnection with exponential backoff
- **Repository Pattern** - SQLite with PostgreSQL-ready abstraction
- **Health Monitoring** - Status endpoint, metrics, connection history
- **MCP Integration** - Claude Code tool integration

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Claude Code                        │
└────────────────────────┬────────────────────────────┘
                         │ MCP Protocol
┌────────────────────────▼────────────────────────────┐
│                  MCP Server (pkg/api/)               │
└────────────────────────┬────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────┐
│               Bridge Core (internal/)                │
│  ┌──────────┐  ┌────────────┐  ┌──────────────┐    │
│  │  State   │  │   Event    │  │   Health     │    │
│  │ Machine  │  │ Processor  │  │   Monitor    │    │
│  └──────────┘  └────────────┘  └──────────────┘    │
└────────────────────────┬────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────┐
│              Storage Layer (internal/store/)         │
│         SQLite with Repository Pattern               │
└─────────────────────────────────────────────────────┘
```

## State Machine

```
Disconnected → Connecting → [QRPending | Authenticating]
                                    ↓
                              Syncing → Ready
                                         ↓
                    ConnectionLost → Reconnecting
```

Terminal states: `LoggedOut`, `SessionExpired`, `TemporaryBan`, `FatalError`, `ShuttingDown`

## Directory Structure

```
whatsapp-bridge-v2/
├── cmd/bridge/          # Entry point
├── internal/
│   ├── state/           # FSM implementation
│   ├── store/           # Repository pattern + SQLite
│   ├── bridge/          # Core bridge logic
│   ├── health/          # Health monitoring
│   └── config/          # Viper configuration
├── pkg/api/             # MCP server
├── migrations/          # SQL migrations
├── config.example.yaml  # Example config
└── go.mod
```

## Configuration

Create `config.yaml` from `config.example.yaml`:

```yaml
session_path: ./store/whatsapp.db
store_path: ./store/messages.db
log_level: info
metrics_enabled: true
```

Priority: CLI flags > Environment (`WABRIDGE_*`) > Config file > Defaults

## Usage

```bash
# Build
go build -o whatsapp-bridge ./cmd/bridge

# Run with default config
./whatsapp-bridge

# Run with custom config
./whatsapp-bridge --config /path/to/config.yaml

# Override log level
./whatsapp-bridge --log-level debug
```

## MCP Tools (55 total)

### Messaging (8)
| Tool | Description |
|------|-------------|
| `send_message` | Send text message |
| `reply_to_message` | Reply to a specific message |
| `forward_message` | Forward message to another chat |
| `edit_message` | Edit a sent message |
| `delete_message` | Delete a message |
| `react_to_message` | Add emoji reaction |
| `star_message` | Star a message |
| `unstar_message` | Unstar a message |

### Chats (10)
| Tool | Description |
|------|-------------|
| `list_chats` | List all chats |
| `get_chat` | Get chat details |
| `list_messages` | Get messages from chat |
| `archive_chat` | Archive a chat |
| `unarchive_chat` | Unarchive a chat |
| `pin_chat` | Pin a chat |
| `unpin_chat` | Unpin a chat |
| `mute_chat` | Mute chat notifications |
| `unmute_chat` | Unmute chat |
| `mark_chat_read` | Mark chat as read |
| `delete_chat` | Delete a chat |

### Contacts (6)
| Tool | Description |
|------|-------------|
| `search_contacts` | Search contacts |
| `get_contact` | Get contact details |
| `block_contact` | Block a contact |
| `unblock_contact` | Unblock a contact |
| `get_blocked_contacts` | List blocked contacts |
| `check_phone_registered` | Check if phone is on WhatsApp |

### Groups (13)
| Tool | Description |
|------|-------------|
| `create_group` | Create a new group |
| `get_group_info` | Get group info |
| `leave_group` | Leave a group |
| `add_group_members` | Add members |
| `remove_group_members` | Remove members |
| `promote_admin` | Promote to admin |
| `demote_admin` | Demote from admin |
| `set_group_name` | Change group name |
| `set_group_topic` | Change group topic |
| `set_group_photo` | Change group photo |
| `get_invite_link` | Get invite link |
| `revoke_invite_link` | Revoke invite link |
| `join_via_invite` | Join via invite link |

### Media (7)
| Tool | Description |
|------|-------------|
| `send_image` | Send an image |
| `send_video` | Send a video |
| `send_audio` | Send audio/voice message |
| `send_document` | Send a document |
| `send_location` | Send location |
| `send_contact_card` | Send contact card |
| `download_media` | Download media from message |

### Presence (5)
| Tool | Description |
|------|-------------|
| `subscribe_presence` | Subscribe to presence updates |
| `send_typing` | Send typing indicator |
| `send_recording` | Send recording indicator |
| `set_online` | Set presence online |
| `set_offline` | Set presence offline |

### Status (4)
| Tool | Description |
|------|-------------|
| `post_text_status` | Post text status |
| `post_image_status` | Post image status |
| `get_status_updates` | Get status updates |
| `delete_status` | Delete status |

### Bridge (2)
| Tool | Description |
|------|-------------|
| `get_bridge_status` | Get health status |
| `get_connection_history` | Get state transitions |

## Development

```bash
# Run tests
go test ./...

# Run with verbose
go test ./... -v

# Run specific package
go test ./internal/state/... -v
```

## License

MIT

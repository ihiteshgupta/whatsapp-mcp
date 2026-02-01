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

## MCP Tools

| Tool | Description |
|------|-------------|
| `send_message` | Send text message |
| `list_chats` | List all chats |
| `get_chat` | Get chat details |
| `search_contacts` | Search contacts |
| `archive_chat` | Archive/unarchive chat |
| `block_contact` | Block/unblock contact |
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

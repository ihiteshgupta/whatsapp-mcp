# WhatsApp Bridge V2 - Complete Redesign

**Date:** 2026-02-01
**Status:** Approved
**Goal:** Production-grade WhatsApp bridge with complex state management, self-healing, and flexible architecture

## Overview

Complete rebuild of the WhatsApp MCP bridge with:
- Finite State Machine (FSM) for connection lifecycle
- Self-healing with automatic reconnection and health monitoring
- Repository pattern with SQLite (PostgreSQL-ready abstraction)
- Layered architecture with clean separation of concerns
- TDD-first development approach
- Viper-style configuration management

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Claude Code                             │
└─────────────────────────┬───────────────────────────────────┘
                          │ MCP Protocol
┌─────────────────────────▼───────────────────────────────────┐
│                     MCP Server (pkg/api/)                    │
│  - Tool registration                                         │
│  - State-aware handlers                                      │
│  - Structured error responses                                │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                    Bridge Core (internal/bridge/)            │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │ State       │  │ Event        │  │ Health            │  │
│  │ Machine     │◄─┤ Processor    │  │ Monitor           │  │
│  │ (stateless) │  │              │  │                   │  │
│  └─────────────┘  └──────────────┘  └───────────────────┘  │
│         │                │                    │              │
│         └────────────────┼────────────────────┘              │
│                          │                                   │
│  ┌───────────────────────▼──────────────────────────────┐   │
│  │              whatsmeow Client Wrapper                 │   │
│  └───────────────────────┬──────────────────────────────┘   │
└──────────────────────────┼──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                    Storage Layer (internal/store/)           │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌──────────┐ │
│  │ Session    │ │ Message    │ │ Chat       │ │ Contact  │ │
│  │ Repository │ │ Repository │ │ Repository │ │ Repository│ │
│  └─────┬──────┘ └─────┬──────┘ └─────┬──────┘ └────┬─────┘ │
│        └──────────────┴──────────────┴─────────────┘        │
│                           │                                  │
│              ┌────────────▼────────────┐                    │
│              │   SQLite Implementation  │                    │
│              │   (PostgreSQL-ready)     │                    │
│              └─────────────────────────┘                    │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
whatsapp-bridge/
├── cmd/
│   └── bridge/
│       └── main.go              # Entry point, CLI flags
├── internal/
│   ├── state/
│   │   ├── machine.go           # FSM setup and configuration
│   │   ├── machine_test.go
│   │   ├── states.go            # State definitions
│   │   ├── triggers.go          # Trigger definitions
│   │   └── guards.go            # Guard functions
│   ├── store/
│   │   ├── repository.go        # Interface definitions
│   │   ├── models.go            # Domain models
│   │   ├── sqlite.go            # SQLite implementation
│   │   ├── sqlite_test.go
│   │   └── migrations/
│   │       ├── 001_initial.up.sql
│   │       └── 001_initial.down.sql
│   ├── bridge/
│   │   ├── bridge.go            # Core bridge struct
│   │   ├── bridge_test.go
│   │   ├── client.go            # whatsmeow wrapper
│   │   ├── events.go            # Event types and bus
│   │   ├── handlers.go          # Event handlers
│   │   └── handlers_test.go
│   ├── health/
│   │   ├── monitor.go           # Health monitoring
│   │   ├── monitor_test.go
│   │   ├── metrics.go           # Prometheus metrics
│   │   └── keepalive.go         # Connection keepalive
│   └── config/
│       ├── config.go            # Viper configuration
│       └── config_test.go
├── pkg/
│   └── api/
│       ├── mcp.go               # MCP server
│       ├── mcp_test.go
│       ├── tools.go             # Tool definitions
│       └── errors.go            # Structured errors
├── migrations/
│   └── sqlite/
│       ├── 001_initial.up.sql
│       └── 001_initial.down.sql
├── config.example.yaml
├── go.mod
├── go.sum
└── README.md
```

## State Machine Design

### States

```go
type State string

const (
    // Primary states
    StateDisconnected  State = "disconnected"
    StateConnecting    State = "connecting"
    StateConnected     State = "connected"
    StateReconnecting  State = "reconnecting"

    // Substates of Connected
    StateQRPending     State = "qr_pending"
    StateAuthenticating State = "authenticating"
    StateSyncing       State = "syncing"
    StateReady         State = "ready"

    // Terminal/Error states
    StateLoggedOut     State = "logged_out"
    StateSessionExpired State = "session_expired"
    StateTemporaryBan  State = "temporary_ban"
    StateShuttingDown  State = "shutting_down"
    StateFatalError    State = "fatal_error"
)
```

### State Diagram

```
                                    ┌─────────────┐
                                    │ Disconnected│◄────────────────┐
                                    └──────┬──────┘                 │
                                           │ Connect                │
                                    ┌──────▼──────┐                 │
                              ┌─────┤ Connecting  ├─────┐           │
                              │     └─────────────┘     │           │
                     QRRequired│                        │Connected  │
                              │                         │           │
                    ┌─────────▼─────────┐    ┌─────────▼─────────┐ │
                    │    QRPending      │    │  Authenticating   │ │
                    │  (show QR code)   │    │                   │ │
                    └─────────┬─────────┘    └─────────┬─────────┘ │
                              │ QRScanned              │Authenticated
                              │                        │           │
                              └────────────┬───────────┘           │
                                           │                       │
                                    ┌──────▼──────┐                │
                                    │   Syncing   │                │
                                    │ (load data) │                │
                                    └──────┬──────┘                │
                                           │ SyncComplete          │
                                    ┌──────▼──────┐                │
                         ┌──────────┤    Ready    │◄───────┐       │
                         │          │ (operational)│        │       │
                         │          └──────┬──────┘        │       │
              ConnectionLost               │         Reconnected   │
                         │                 │               │       │
                  ┌──────▼──────┐          │        ┌──────┴──────┐│
                  │ Reconnecting├──────────┴───────►│ Reconnecting││
                  │  (backoff)  │ MaxRetriesExceeded└─────────────┘│
                  └──────┬──────┘                                  │
                         │                                         │
                         └─────────────────────────────────────────┘

    Terminal States:
    ┌─────────────┐  ┌───────────────┐  ┌──────────────┐  ┌────────────┐
    │  LoggedOut  │  │ SessionExpired│  │ TemporaryBan │  │ FatalError │
    └─────────────┘  └───────────────┘  └──────────────┘  └────────────┘
```

### Triggers

```go
type Trigger string

const (
    TriggerConnect        Trigger = "connect"
    TriggerDisconnect     Trigger = "disconnect"
    TriggerQRRequired     Trigger = "qr_required"
    TriggerQRScanned      Trigger = "qr_scanned"
    TriggerAuthenticated  Trigger = "authenticated"
    TriggerSyncComplete   Trigger = "sync_complete"
    TriggerConnectionLost Trigger = "connection_lost"
    TriggerReconnect      Trigger = "reconnect"
    TriggerReconnected    Trigger = "reconnected"
    TriggerSessionInvalid Trigger = "session_invalid"
    TriggerBanDetected    Trigger = "ban_detected"
    TriggerBanLifted      Trigger = "ban_lifted"
    TriggerLogout         Trigger = "logout"
    TriggerShutdown       Trigger = "shutdown"
    TriggerFatalError     Trigger = "fatal_error"
)
```

## Storage Layer

### Repository Interfaces

```go
type SessionRepository interface {
    GetSession(ctx context.Context) (*Session, error)
    SaveSession(ctx context.Context, s *Session) error
    DeleteSession(ctx context.Context) error
}

type MessageRepository interface {
    Store(ctx context.Context, msg *Message) error
    GetByChat(ctx context.Context, chatJID string, opts QueryOpts) ([]Message, error)
    GetByID(ctx context.Context, chatJID, msgID string) (*Message, error)
    Search(ctx context.Context, query string, opts QueryOpts) ([]Message, error)
    Delete(ctx context.Context, chatJID, msgID string) error
}

type ChatRepository interface {
    Upsert(ctx context.Context, chat *Chat) error
    GetAll(ctx context.Context) ([]Chat, error)
    GetByJID(ctx context.Context, jid string) (*Chat, error)
    UpdateLastMessage(ctx context.Context, jid string, t time.Time) error
    Archive(ctx context.Context, jid string, archived bool) error
    Pin(ctx context.Context, jid string, pinned bool) error
    Mute(ctx context.Context, jid string, until *time.Time) error
}

type ContactRepository interface {
    Upsert(ctx context.Context, contact *Contact) error
    Search(ctx context.Context, query string) ([]Contact, error)
    GetByJID(ctx context.Context, jid string) (*Contact, error)
    Block(ctx context.Context, jid string, blocked bool) error
    GetBlocked(ctx context.Context) ([]Contact, error)
}

type StateRepository interface {
    GetState(ctx context.Context) (State, error)
    SaveState(ctx context.Context, s State) error
    LogTransition(ctx context.Context, from, to State, trigger string) error
    GetTransitionHistory(ctx context.Context, limit int) ([]Transition, error)
}
```

### Query Options

```go
type QueryOpts struct {
    Limit     int
    Offset    int
    Before    *time.Time
    After     *time.Time
    MediaOnly bool
    FromMe    *bool
}
```

### Domain Models

```go
type Message struct {
    ID            string
    ChatJID       string
    Sender        string
    Content       string
    Timestamp     time.Time
    IsFromMe      bool
    MediaType     string
    Filename      string
    MediaURL      string
    MediaKey      []byte
    QuotedID      string
    QuotedSender  string
}

type Chat struct {
    JID             string
    Name            string
    IsGroup         bool
    LastMessageTime time.Time
    UnreadCount     int
    Archived        bool
    Pinned          bool
    MutedUntil      *time.Time
}

type Contact struct {
    JID          string
    Name         string
    PushName     string
    BusinessName string
    Blocked      bool
}

type Transition struct {
    ID        int64
    FromState State
    ToState   State
    Trigger   string
    Timestamp time.Time
    Error     string
}
```

## Bridge Core

### Bridge Structure

```go
type Bridge struct {
    client    *whatsmeow.Client
    state     *stateless.StateMachine
    store     *Store
    events    chan Event
    health    *HealthMonitor
    config    *Config
    log       *slog.Logger

    mu        sync.RWMutex
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
}

type Store struct {
    Sessions  SessionRepository
    Messages  MessageRepository
    Chats     ChatRepository
    Contacts  ContactRepository
    State     StateRepository
}
```

### Event System

```go
type Event struct {
    Type      EventType
    Payload   any
    Timestamp time.Time
}

type EventType int

const (
    EventMessage EventType = iota
    EventReceipt
    EventPresence
    EventGroupUpdate
    EventConnectionChange
    EventHistorySync
    EventCallOffer
    EventChatArchive
)
```

## Health Monitoring

### HealthMonitor Structure

```go
type HealthMonitor struct {
    bridge           *Bridge
    metrics          *Metrics

    keepaliveInterval time.Duration
    keepaliveTicker   *time.Ticker

    reconnectBackoff  *backoff.ExponentialBackOff

    lastPong         time.Time
    lastMessage      time.Time
    reconnectCount   int
    startTime        time.Time

    mu               sync.RWMutex
}
```

### Metrics

```go
type Metrics struct {
    ConnectionState     *prometheus.GaugeVec
    StateTransitions    *prometheus.CounterVec
    MessagesReceived    prometheus.Counter
    MessagesSent        prometheus.Counter
    ReconnectAttempts   prometheus.Counter
    LastMessageTime     prometheus.Gauge
    Uptime              prometheus.Gauge
    EventProcessingTime prometheus.Histogram
}
```

### Health Response

```go
type HealthStatus struct {
    State           string    `json:"state"`
    Connected       bool      `json:"connected"`
    UptimeSeconds   int64     `json:"uptime_seconds"`
    LastMessage     time.Time `json:"last_message"`
    ReconnectCount  int       `json:"reconnect_count"`
    MessagesTotal   int64     `json:"messages_total"`
}
```

## MCP Server

### Tool Definitions

```go
// Messaging
send_message      - Send text message to recipient
reply_message     - Reply to a specific message
forward_message   - Forward message to another chat
edit_message      - Edit a sent message
delete_message    - Delete a message
react_to_message  - React with emoji

// Chats
list_chats        - List all chats with metadata
get_chat          - Get specific chat details
archive_chat      - Archive/unarchive a chat
pin_chat          - Pin/unpin a chat
mute_chat         - Mute chat for duration
mark_read         - Mark messages as read

// Contacts
search_contacts   - Search contacts by name/phone
get_contact       - Get contact details
block_contact     - Block/unblock contact
get_blocked       - List blocked contacts

// Groups
create_group      - Create new group
get_group_info    - Get group details and members
add_members       - Add members to group
remove_members    - Remove members from group
set_group_name    - Update group name
set_group_topic   - Update group description
leave_group       - Leave a group
get_invite_link   - Get group invite link

// Media
send_file         - Send image/video/document
send_audio        - Send voice message
download_media    - Download media from message
send_location     - Send location

// Status
get_bridge_status    - Get bridge health status
get_connection_history - Get state transition history
```

### Structured Errors

```go
type MCPError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Retry   bool   `json:"retry"`
}

// Error codes
const (
    ErrNotReady       = "NOT_READY"
    ErrInvalidJID     = "INVALID_JID"
    ErrMessageFailed  = "MESSAGE_FAILED"
    ErrMediaFailed    = "MEDIA_FAILED"
    ErrNotFound       = "NOT_FOUND"
    ErrRateLimited    = "RATE_LIMITED"
    ErrSessionExpired = "SESSION_EXPIRED"
)
```

## Configuration

### Config Structure

```go
type Config struct {
    // Paths
    SessionPath string `mapstructure:"session_path"`
    StorePath   string `mapstructure:"store_path"`

    // Connection
    ConnectTimeout time.Duration `mapstructure:"connect_timeout"`

    // Health & Reconnection
    KeepaliveInterval   time.Duration `mapstructure:"keepalive_interval"`
    ReconnectMaxRetries int           `mapstructure:"reconnect_max_retries"`
    ReconnectBaseDelay  time.Duration `mapstructure:"reconnect_base_delay"`
    ReconnectMaxDelay   time.Duration `mapstructure:"reconnect_max_delay"`

    // Logging
    LogLevel  string `mapstructure:"log_level"`
    LogFormat string `mapstructure:"log_format"`

    // Metrics
    MetricsEnabled bool `mapstructure:"metrics_enabled"`
    MetricsPort    int  `mapstructure:"metrics_port"`

    // MCP
    MCPEnabled bool `mapstructure:"mcp_enabled"`
}
```

### Example config.yaml

```yaml
session_path: ./store/whatsapp.db
store_path: ./store/messages.db

connect_timeout: 30s

keepalive_interval: 30s
reconnect_max_retries: 10
reconnect_base_delay: 1s
reconnect_max_delay: 5m

log_level: info
log_format: json

metrics_enabled: true
metrics_port: 9090

mcp_enabled: true
```

### Priority Order

1. CLI flags (`--log-level=debug`)
2. Environment variables (`WABRIDGE_LOG_LEVEL=debug`)
3. Config file (`config.yaml`)
4. Defaults

## Testing Strategy

### TDD Workflow

1. Write failing test for the feature
2. Implement minimum code to pass
3. Refactor while keeping tests green
4. Repeat

### Test Categories

**Unit Tests:**
- State machine transitions and guards
- Repository implementations (in-memory SQLite)
- Event handlers
- Config parsing

**Integration Tests:**
- Full bridge with mocked whatsmeow client
- State transitions on simulated events
- Message persistence through full flow
- Reconnection behavior

**Test Fixtures:**

```go
// Fake whatsmeow client
type FakeClient struct {
    connected    bool
    sentMessages []Message
    events       chan interface{}
    mu           sync.Mutex
}

func (f *FakeClient) SendMessage(...) error { ... }
func (f *FakeClient) Connect() error { ... }
func (f *FakeClient) IsConnected() bool { ... }

// In-memory store
func NewTestStore(t *testing.T) *Store {
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    runMigrations(db)
    return NewStore(db)
}
```

## Implementation Phases

### Phase 1: Foundation
- [ ] Project structure and go.mod
- [ ] State machine with all states and transitions
- [ ] State machine tests
- [ ] Config loading with Viper
- [ ] Config tests

### Phase 2: Storage
- [ ] Repository interfaces
- [ ] Domain models
- [ ] SQLite migrations
- [ ] SQLite implementation
- [ ] Repository tests

### Phase 3: Bridge Core
- [ ] Bridge struct and initialization
- [ ] whatsmeow client wrapper
- [ ] Event bus and processor
- [ ] Event handlers
- [ ] Bridge tests with fake client

### Phase 4: Health & Self-Healing
- [ ] Health monitor
- [ ] Keepalive detection
- [ ] Reconnection with backoff
- [ ] Metrics collection
- [ ] Health tests

### Phase 5: MCP Integration
- [ ] MCP server setup
- [ ] Tool registration
- [ ] State-aware handlers
- [ ] Structured errors
- [ ] MCP tests

### Phase 6: Polish
- [ ] Logging throughout
- [ ] Graceful shutdown
- [ ] Documentation
- [ ] Integration tests
- [ ] README update

## Dependencies

```go
require (
    go.mau.fi/whatsmeow v0.0.0-latest
    github.com/qmuntal/stateless v1.7.0
    github.com/mattn/go-sqlite3 v1.14.22
    github.com/golang-migrate/migrate/v4 v4.17.0
    github.com/spf13/viper v1.18.2
    github.com/prometheus/client_golang v1.19.0
    github.com/cenkalti/backoff/v4 v4.2.1
    github.com/stretchr/testify v1.9.0
)
```

## Success Criteria

1. All state transitions work correctly with proper guards
2. Self-healing recovers from disconnections automatically
3. Messages persist correctly through the storage layer
4. MCP tools work with state-awareness
5. Health endpoint reports accurate status
6. Metrics are exposed for monitoring
7. All tests pass with good coverage
8. Clean shutdown without data loss

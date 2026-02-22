-- Chats table
CREATE TABLE IF NOT EXISTS chats (
    jid TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    is_group BOOLEAN NOT NULL DEFAULT FALSE,
    last_message_time TIMESTAMP,
    unread_count INTEGER NOT NULL DEFAULT 0,
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    pinned BOOLEAN NOT NULL DEFAULT FALSE,
    muted_until TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chats_last_message ON chats(last_message_time DESC);
CREATE INDEX IF NOT EXISTS idx_chats_archived ON chats(archived);
CREATE INDEX IF NOT EXISTS idx_chats_pinned ON chats(pinned);

-- Messages table
CREATE TABLE IF NOT EXISTS messages (
    id TEXT NOT NULL,
    chat_jid TEXT NOT NULL,
    sender TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    timestamp TIMESTAMP NOT NULL,
    is_from_me BOOLEAN NOT NULL DEFAULT FALSE,
    media_type TEXT NOT NULL DEFAULT '',
    filename TEXT NOT NULL DEFAULT '',
    media_url TEXT NOT NULL DEFAULT '',
    media_key BLOB,
    quoted_id TEXT NOT NULL DEFAULT '',
    quoted_sender TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (id, chat_jid),
    FOREIGN KEY (chat_jid) REFERENCES chats(jid) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_timestamp ON messages(chat_jid, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender);
CREATE INDEX IF NOT EXISTS idx_messages_content ON messages(content);

-- Contacts table
CREATE TABLE IF NOT EXISTS contacts (
    jid TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    push_name TEXT NOT NULL DEFAULT '',
    business_name TEXT NOT NULL DEFAULT '',
    blocked BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_contacts_name ON contacts(name);
CREATE INDEX IF NOT EXISTS idx_contacts_blocked ON contacts(blocked);

-- State table
CREATE TABLE IF NOT EXISTS bridge_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    state TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- Insert default state
INSERT OR IGNORE INTO bridge_state (id, state, updated_at)
VALUES (1, 'disconnected', CURRENT_TIMESTAMP);

-- Transitions history table
CREATE TABLE IF NOT EXISTS transitions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_state TEXT NOT NULL,
    to_state TEXT NOT NULL,
    trigger TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_transitions_timestamp ON transitions(timestamp DESC);

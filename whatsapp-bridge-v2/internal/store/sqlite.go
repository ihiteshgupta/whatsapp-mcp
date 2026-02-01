package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/state"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("record not found")

// SQLiteStore implements all repositories using SQLite.
type SQLiteStore struct {
	db       *sql.DB
	Messages *SQLiteMessageRepo
	Chats    *SQLiteChatRepo
	Contacts *SQLiteContactRepo
	State    *SQLiteStateRepo
}

// NewSQLiteStore creates a new SQLite-backed store.
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dsn+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	store := &SQLiteStore{
		db:       db,
		Messages: &SQLiteMessageRepo{db: db},
		Chats:    &SQLiteChatRepo{db: db},
		Contacts: &SQLiteContactRepo{db: db},
		State:    &SQLiteStateRepo{db: db},
	}

	return store, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func runMigrations(db *sql.DB) error {
	migration := `
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

	-- Contacts table
	CREATE TABLE IF NOT EXISTS contacts (
		jid TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		push_name TEXT NOT NULL DEFAULT '',
		business_name TEXT NOT NULL DEFAULT '',
		blocked BOOLEAN NOT NULL DEFAULT FALSE
	);

	-- State table
	CREATE TABLE IF NOT EXISTS bridge_state (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		state TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

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
	`
	_, err := db.Exec(migration)
	return err
}

// SQLiteMessageRepo implements MessageRepository.
type SQLiteMessageRepo struct {
	db *sql.DB
}

func (r *SQLiteMessageRepo) Store(ctx context.Context, msg *Message) error {
	query := `
		INSERT OR REPLACE INTO messages
		(id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, media_url, media_key, quoted_id, quoted_sender)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.ChatJID, msg.Sender, msg.Content, msg.Timestamp, msg.IsFromMe,
		msg.MediaType, msg.Filename, msg.MediaURL, msg.MediaKey, msg.QuotedID, msg.QuotedSender,
	)
	return err
}

func (r *SQLiteMessageRepo) GetByChat(ctx context.Context, chatJID string, opts QueryOpts) ([]Message, error) {
	query := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, media_url, media_key, quoted_id, quoted_sender
		FROM messages
		WHERE chat_jid = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, query, chatJID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (r *SQLiteMessageRepo) GetByID(ctx context.Context, chatJID, msgID string) (*Message, error) {
	query := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, media_url, media_key, quoted_id, quoted_sender
		FROM messages
		WHERE chat_jid = ? AND id = ?
	`
	row := r.db.QueryRowContext(ctx, query, chatJID, msgID)

	var msg Message
	err := row.Scan(
		&msg.ID, &msg.ChatJID, &msg.Sender, &msg.Content, &msg.Timestamp, &msg.IsFromMe,
		&msg.MediaType, &msg.Filename, &msg.MediaURL, &msg.MediaKey, &msg.QuotedID, &msg.QuotedSender,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *SQLiteMessageRepo) Search(ctx context.Context, query string, opts QueryOpts) ([]Message, error) {
	sqlQuery := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, media_url, media_key, quoted_id, quoted_sender
		FROM messages
		WHERE content LIKE ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, sqlQuery, "%"+query+"%", opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (r *SQLiteMessageRepo) Delete(ctx context.Context, chatJID, msgID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM messages WHERE chat_jid = ? AND id = ?", chatJID, msgID)
	return err
}

func (r *SQLiteMessageRepo) Count(ctx context.Context, chatJID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages WHERE chat_jid = ?", chatJID).Scan(&count)
	return count, err
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	var messages []Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(
			&msg.ID, &msg.ChatJID, &msg.Sender, &msg.Content, &msg.Timestamp, &msg.IsFromMe,
			&msg.MediaType, &msg.Filename, &msg.MediaURL, &msg.MediaKey, &msg.QuotedID, &msg.QuotedSender,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// SQLiteChatRepo implements ChatRepository.
type SQLiteChatRepo struct {
	db *sql.DB
}

func (r *SQLiteChatRepo) Upsert(ctx context.Context, chat *Chat) error {
	query := `
		INSERT INTO chats (jid, name, is_group, last_message_time, unread_count, archived, pinned, muted_until)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = excluded.name,
			is_group = excluded.is_group,
			last_message_time = excluded.last_message_time,
			unread_count = excluded.unread_count,
			archived = excluded.archived,
			pinned = excluded.pinned,
			muted_until = excluded.muted_until
	`
	_, err := r.db.ExecContext(ctx, query,
		chat.JID, chat.Name, chat.IsGroup, chat.LastMessageTime, chat.UnreadCount,
		chat.Archived, chat.Pinned, chat.MutedUntil,
	)
	return err
}

func (r *SQLiteChatRepo) GetAll(ctx context.Context) ([]Chat, error) {
	query := `
		SELECT jid, name, is_group, last_message_time, unread_count, archived, pinned, muted_until
		FROM chats
		ORDER BY last_message_time DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanChats(rows)
}

func (r *SQLiteChatRepo) GetByJID(ctx context.Context, jid string) (*Chat, error) {
	query := `
		SELECT jid, name, is_group, last_message_time, unread_count, archived, pinned, muted_until
		FROM chats WHERE jid = ?
	`
	row := r.db.QueryRowContext(ctx, query, jid)

	var chat Chat
	var lastMsgTime sql.NullTime
	var mutedUntil sql.NullTime

	err := row.Scan(&chat.JID, &chat.Name, &chat.IsGroup, &lastMsgTime, &chat.UnreadCount, &chat.Archived, &chat.Pinned, &mutedUntil)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if lastMsgTime.Valid {
		chat.LastMessageTime = lastMsgTime.Time
	}
	if mutedUntil.Valid {
		chat.MutedUntil = &mutedUntil.Time
	}

	return &chat, nil
}

func (r *SQLiteChatRepo) UpdateLastMessage(ctx context.Context, jid string, t time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE chats SET last_message_time = ? WHERE jid = ?", t, jid)
	return err
}

func (r *SQLiteChatRepo) Archive(ctx context.Context, jid string, archived bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE chats SET archived = ? WHERE jid = ?", archived, jid)
	return err
}

func (r *SQLiteChatRepo) Pin(ctx context.Context, jid string, pinned bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE chats SET pinned = ? WHERE jid = ?", pinned, jid)
	return err
}

func (r *SQLiteChatRepo) Mute(ctx context.Context, jid string, until *time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE chats SET muted_until = ? WHERE jid = ?", until, jid)
	return err
}

func (r *SQLiteChatRepo) Delete(ctx context.Context, jid string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM chats WHERE jid = ?", jid)
	return err
}

func (r *SQLiteChatRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chats").Scan(&count)
	return count, err
}

func scanChats(rows *sql.Rows) ([]Chat, error) {
	var chats []Chat
	for rows.Next() {
		var chat Chat
		var lastMsgTime sql.NullTime
		var mutedUntil sql.NullTime

		err := rows.Scan(&chat.JID, &chat.Name, &chat.IsGroup, &lastMsgTime, &chat.UnreadCount, &chat.Archived, &chat.Pinned, &mutedUntil)
		if err != nil {
			return nil, err
		}

		if lastMsgTime.Valid {
			chat.LastMessageTime = lastMsgTime.Time
		}
		if mutedUntil.Valid {
			chat.MutedUntil = &mutedUntil.Time
		}

		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

// SQLiteContactRepo implements ContactRepository.
type SQLiteContactRepo struct {
	db *sql.DB
}

func (r *SQLiteContactRepo) Upsert(ctx context.Context, contact *Contact) error {
	query := `
		INSERT INTO contacts (jid, name, push_name, business_name, blocked)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = excluded.name,
			push_name = excluded.push_name,
			business_name = excluded.business_name,
			blocked = excluded.blocked
	`
	_, err := r.db.ExecContext(ctx, query, contact.JID, contact.Name, contact.PushName, contact.BusinessName, contact.Blocked)
	return err
}

func (r *SQLiteContactRepo) Search(ctx context.Context, query string) ([]Contact, error) {
	sqlQuery := `
		SELECT jid, name, push_name, business_name, blocked
		FROM contacts
		WHERE name LIKE ? OR push_name LIKE ? OR business_name LIKE ?
	`
	pattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, sqlQuery, pattern, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanContacts(rows)
}

func (r *SQLiteContactRepo) GetByJID(ctx context.Context, jid string) (*Contact, error) {
	query := `SELECT jid, name, push_name, business_name, blocked FROM contacts WHERE jid = ?`
	row := r.db.QueryRowContext(ctx, query, jid)

	var contact Contact
	err := row.Scan(&contact.JID, &contact.Name, &contact.PushName, &contact.BusinessName, &contact.Blocked)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

func (r *SQLiteContactRepo) Block(ctx context.Context, jid string, blocked bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE contacts SET blocked = ? WHERE jid = ?", blocked, jid)
	return err
}

func (r *SQLiteContactRepo) GetBlocked(ctx context.Context) ([]Contact, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT jid, name, push_name, business_name, blocked FROM contacts WHERE blocked = TRUE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanContacts(rows)
}

func (r *SQLiteContactRepo) Delete(ctx context.Context, jid string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM contacts WHERE jid = ?", jid)
	return err
}

func (r *SQLiteContactRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM contacts").Scan(&count)
	return count, err
}

func scanContacts(rows *sql.Rows) ([]Contact, error) {
	var contacts []Contact
	for rows.Next() {
		var contact Contact
		err := rows.Scan(&contact.JID, &contact.Name, &contact.PushName, &contact.BusinessName, &contact.Blocked)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

// SQLiteStateRepo implements StateRepository.
type SQLiteStateRepo struct {
	db *sql.DB
}

func (r *SQLiteStateRepo) GetState(ctx context.Context) (state.State, error) {
	var s string
	err := r.db.QueryRowContext(ctx, "SELECT state FROM bridge_state WHERE id = 1").Scan(&s)
	if err != nil {
		return "", err
	}
	return state.State(s), nil
}

func (r *SQLiteStateRepo) SaveState(ctx context.Context, s state.State) error {
	_, err := r.db.ExecContext(ctx, "UPDATE bridge_state SET state = ?, updated_at = ? WHERE id = 1", string(s), time.Now())
	return err
}

func (r *SQLiteStateRepo) LogTransition(ctx context.Context, from, to state.State, trigger string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO transitions (from_state, to_state, trigger, timestamp) VALUES (?, ?, ?, ?)",
		string(from), string(to), trigger, time.Now(),
	)
	return err
}

func (r *SQLiteStateRepo) GetTransitionHistory(ctx context.Context, limit int) ([]Transition, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, from_state, to_state, trigger, timestamp, error FROM transitions ORDER BY timestamp DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transitions []Transition
	for rows.Next() {
		var t Transition
		var from, to string
		err := rows.Scan(&t.ID, &from, &to, &t.Trigger, &t.Timestamp, &t.Error)
		if err != nil {
			return nil, err
		}
		t.FromState = state.State(from)
		t.ToState = state.State(to)
		transitions = append(transitions, t)
	}
	return transitions, rows.Err()
}

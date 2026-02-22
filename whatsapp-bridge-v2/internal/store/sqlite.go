package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ihiteshgupta/whatsapp-mcp/whatsapp-bridge-v2/internal/state"
)

// SQLiteStore implements all repositories using SQLite.
type SQLiteStore struct {
	db       *sql.DB
	Messages *SQLiteMessageRepo
	Chats    *SQLiteChatRepo
	Contacts *SQLiteContactRepo
	Groups   *SQLiteGroupRepo
	Status   *SQLiteStatusRepo
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
		Groups:   &SQLiteGroupRepo{db: db},
		Status:   &SQLiteStatusRepo{db: db},
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
		muted BOOLEAN NOT NULL DEFAULT FALSE,
		muted_until TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
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
		file_sha256 BLOB,
		file_length INTEGER NOT NULL DEFAULT 0,
		quoted_id TEXT NOT NULL DEFAULT '',
		quoted_sender TEXT NOT NULL DEFAULT '',
		is_starred BOOLEAN NOT NULL DEFAULT FALSE,
		is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
		reactions TEXT NOT NULL DEFAULT '[]',
		PRIMARY KEY (id, chat_jid),
		FOREIGN KEY (chat_jid) REFERENCES chats(jid) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_messages_chat_timestamp ON messages(chat_jid, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_messages_starred ON messages(is_starred) WHERE is_starred = TRUE;

	-- Contacts table
	CREATE TABLE IF NOT EXISTS contacts (
		jid TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		push_name TEXT NOT NULL DEFAULT '',
		phone TEXT NOT NULL DEFAULT '',
		business_name TEXT NOT NULL DEFAULT '',
		blocked BOOLEAN NOT NULL DEFAULT FALSE,
		is_saved BOOLEAN NOT NULL DEFAULT FALSE,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_contacts_blocked ON contacts(blocked) WHERE blocked = TRUE;

	-- Groups table
	CREATE TABLE IF NOT EXISTS groups (
		jid TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		topic TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP,
		created_by TEXT NOT NULL DEFAULT '',
		invite_link TEXT NOT NULL DEFAULT '',
		is_announce BOOLEAN NOT NULL DEFAULT FALSE,
		is_locked BOOLEAN NOT NULL DEFAULT FALSE,
		participant_count INTEGER NOT NULL DEFAULT 0,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Group participants table
	CREATE TABLE IF NOT EXISTS group_participants (
		group_jid TEXT NOT NULL,
		user_jid TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'member',
		joined_at TIMESTAMP,
		PRIMARY KEY (group_jid, user_jid),
		FOREIGN KEY (group_jid) REFERENCES groups(jid) ON DELETE CASCADE
	);

	-- Status updates table
	CREATE TABLE IF NOT EXISTS status_updates (
		id TEXT PRIMARY KEY,
		sender_jid TEXT NOT NULL,
		media_type TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		posted_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		viewed BOOLEAN NOT NULL DEFAULT FALSE
	);

	CREATE INDEX IF NOT EXISTS idx_status_sender ON status_updates(sender_jid);
	CREATE INDEX IF NOT EXISTS idx_status_expires ON status_updates(expires_at);

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
		(id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, media_url, media_key, file_sha256, file_length, quoted_id, quoted_sender, is_starred, is_deleted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.ChatJID, msg.Sender, msg.Content, msg.Timestamp, msg.IsFromMe,
		msg.MediaType, msg.Filename, msg.MediaURL, msg.MediaKey, msg.FileSHA256, msg.FileLength,
		msg.QuotedID, msg.QuotedSender, msg.IsStarred, msg.IsDeleted,
	)
	return err
}

func (r *SQLiteMessageRepo) List(ctx context.Context, chatJID string, limit int, before string) ([]Message, error) {
	var query string
	var args []interface{}

	if before != "" {
		query = `
			SELECT id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, media_url, quoted_id, quoted_sender, is_starred, is_deleted
			FROM messages
			WHERE chat_jid = ? AND timestamp < (SELECT timestamp FROM messages WHERE id = ? AND chat_jid = ?)
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{chatJID, before, chatJID, limit}
	} else {
		query = `
			SELECT id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, media_url, quoted_id, quoted_sender, is_starred, is_deleted
			FROM messages
			WHERE chat_jid = ?
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{chatJID, limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (r *SQLiteMessageRepo) GetByID(ctx context.Context, chatJID, msgID string) (*Message, error) {
	query := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, media_url, quoted_id, quoted_sender, is_starred, is_deleted
		FROM messages
		WHERE chat_jid = ? AND id = ?
	`
	row := r.db.QueryRowContext(ctx, query, chatJID, msgID)

	var msg Message
	err := row.Scan(
		&msg.ID, &msg.ChatJID, &msg.Sender, &msg.Content, &msg.Timestamp, &msg.IsFromMe,
		&msg.MediaType, &msg.Filename, &msg.MediaURL, &msg.QuotedID, &msg.QuotedSender, &msg.IsStarred, &msg.IsDeleted,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *SQLiteMessageRepo) Search(ctx context.Context, query string, limit int) ([]Message, error) {
	sqlQuery := `
		SELECT id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, media_url, quoted_id, quoted_sender, is_starred, is_deleted
		FROM messages
		WHERE content LIKE ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, sqlQuery, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (r *SQLiteMessageRepo) SetStarred(ctx context.Context, chatJID, msgID string, starred bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE messages SET is_starred = ? WHERE chat_jid = ? AND id = ?", starred, chatJID, msgID)
	return err
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
			&msg.MediaType, &msg.Filename, &msg.MediaURL, &msg.QuotedID, &msg.QuotedSender, &msg.IsStarred, &msg.IsDeleted,
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
		INSERT INTO chats (jid, name, is_group, last_message_time, unread_count, archived, pinned, muted, muted_until, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = excluded.name,
			is_group = excluded.is_group,
			last_message_time = excluded.last_message_time,
			unread_count = excluded.unread_count,
			archived = excluded.archived,
			pinned = excluded.pinned,
			muted = excluded.muted,
			muted_until = excluded.muted_until,
			updated_at = excluded.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		chat.JID, chat.Name, chat.IsGroup, chat.LastMessageTime, chat.UnreadCount,
		chat.Archived, chat.Pinned, chat.Muted, chat.MutedUntil, time.Now(),
	)
	return err
}

func (r *SQLiteChatRepo) List(ctx context.Context, limit int) ([]Chat, error) {
	query := `
		SELECT jid, name, is_group, last_message_time, unread_count, archived, pinned, muted, muted_until, updated_at
		FROM chats
		ORDER BY last_message_time DESC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanChats(rows)
}

func (r *SQLiteChatRepo) GetByJID(ctx context.Context, jid string) (*Chat, error) {
	query := `
		SELECT jid, name, is_group, last_message_time, unread_count, archived, pinned, muted, muted_until, updated_at
		FROM chats WHERE jid = ?
	`
	row := r.db.QueryRowContext(ctx, query, jid)

	var chat Chat
	var lastMsgTime sql.NullTime
	var mutedUntil sql.NullTime

	err := row.Scan(&chat.JID, &chat.Name, &chat.IsGroup, &lastMsgTime, &chat.UnreadCount, &chat.Archived, &chat.Pinned, &chat.Muted, &mutedUntil, &chat.UpdatedAt)
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
	_, err := r.db.ExecContext(ctx, "UPDATE chats SET last_message_time = ?, updated_at = ? WHERE jid = ?", t, time.Now(), jid)
	return err
}

func (r *SQLiteChatRepo) Archive(ctx context.Context, jid string, archived bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE chats SET archived = ?, updated_at = ? WHERE jid = ?", archived, time.Now(), jid)
	return err
}

func (r *SQLiteChatRepo) Pin(ctx context.Context, jid string, pinned bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE chats SET pinned = ?, updated_at = ? WHERE jid = ?", pinned, time.Now(), jid)
	return err
}

func (r *SQLiteChatRepo) Mute(ctx context.Context, jid string, muted bool, until *time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE chats SET muted = ?, muted_until = ?, updated_at = ? WHERE jid = ?", muted, until, time.Now(), jid)
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

		err := rows.Scan(&chat.JID, &chat.Name, &chat.IsGroup, &lastMsgTime, &chat.UnreadCount, &chat.Archived, &chat.Pinned, &chat.Muted, &mutedUntil, &chat.UpdatedAt)
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
		INSERT INTO contacts (jid, name, push_name, phone, business_name, blocked, is_saved, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = excluded.name,
			push_name = excluded.push_name,
			phone = excluded.phone,
			business_name = excluded.business_name,
			blocked = excluded.blocked,
			is_saved = excluded.is_saved,
			updated_at = excluded.updated_at
	`
	_, err := r.db.ExecContext(ctx, query, contact.JID, contact.Name, contact.PushName, contact.Phone, contact.BusinessName, contact.Blocked, contact.IsSaved, time.Now())
	return err
}

func (r *SQLiteContactRepo) Search(ctx context.Context, query string, limit int) ([]Contact, error) {
	sqlQuery := `
		SELECT jid, name, push_name, phone, business_name, blocked, is_saved, updated_at
		FROM contacts
		WHERE name LIKE ? OR push_name LIKE ? OR business_name LIKE ? OR phone LIKE ?
		LIMIT ?
	`
	pattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, sqlQuery, pattern, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanContacts(rows)
}

func (r *SQLiteContactRepo) GetByJID(ctx context.Context, jid string) (*Contact, error) {
	query := `SELECT jid, name, push_name, phone, business_name, blocked, is_saved, updated_at FROM contacts WHERE jid = ?`
	row := r.db.QueryRowContext(ctx, query, jid)

	var contact Contact
	err := row.Scan(&contact.JID, &contact.Name, &contact.PushName, &contact.Phone, &contact.BusinessName, &contact.Blocked, &contact.IsSaved, &contact.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

func (r *SQLiteContactRepo) Block(ctx context.Context, jid string, blocked bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE contacts SET blocked = ?, updated_at = ? WHERE jid = ?", blocked, time.Now(), jid)
	return err
}

func (r *SQLiteContactRepo) GetBlocked(ctx context.Context) ([]Contact, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT jid, name, push_name, phone, business_name, blocked, is_saved, updated_at FROM contacts WHERE blocked = TRUE")
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
		err := rows.Scan(&contact.JID, &contact.Name, &contact.PushName, &contact.Phone, &contact.BusinessName, &contact.Blocked, &contact.IsSaved, &contact.UpdatedAt)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

// SQLiteGroupRepo implements GroupRepository.
type SQLiteGroupRepo struct {
	db *sql.DB
}

func (r *SQLiteGroupRepo) Upsert(ctx context.Context, group *Group) error {
	query := `
		INSERT INTO groups (jid, name, topic, created_at, created_by, invite_link, is_announce, is_locked, participant_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET
			name = excluded.name,
			topic = excluded.topic,
			invite_link = excluded.invite_link,
			is_announce = excluded.is_announce,
			is_locked = excluded.is_locked,
			participant_count = excluded.participant_count,
			updated_at = excluded.updated_at
	`
	_, err := r.db.ExecContext(ctx, query, group.JID, group.Name, group.Topic, group.CreatedAt, group.CreatedBy, group.InviteLink, group.IsAnnounce, group.IsLocked, group.ParticipantCount, time.Now())
	return err
}

func (r *SQLiteGroupRepo) GetByJID(ctx context.Context, jid string) (*Group, error) {
	query := `SELECT jid, name, topic, created_at, created_by, invite_link, is_announce, is_locked, participant_count, updated_at FROM groups WHERE jid = ?`
	row := r.db.QueryRowContext(ctx, query, jid)

	var group Group
	var createdAt sql.NullTime
	err := row.Scan(&group.JID, &group.Name, &group.Topic, &createdAt, &group.CreatedBy, &group.InviteLink, &group.IsAnnounce, &group.IsLocked, &group.ParticipantCount, &group.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if createdAt.Valid {
		group.CreatedAt = createdAt.Time
	}
	return &group, nil
}

func (r *SQLiteGroupRepo) UpdateParticipants(ctx context.Context, groupJID string, participants []GroupParticipant) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing participants
	_, err = tx.ExecContext(ctx, "DELETE FROM group_participants WHERE group_jid = ?", groupJID)
	if err != nil {
		return err
	}

	// Insert new participants
	for _, p := range participants {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO group_participants (group_jid, user_jid, role, joined_at) VALUES (?, ?, ?, ?)",
			groupJID, p.UserJID, p.Role, p.JoinedAt,
		)
		if err != nil {
			return err
		}
	}

	// Update participant count
	_, err = tx.ExecContext(ctx, "UPDATE groups SET participant_count = ?, updated_at = ? WHERE jid = ?", len(participants), time.Now(), groupJID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *SQLiteGroupRepo) GetParticipants(ctx context.Context, groupJID string) ([]GroupParticipant, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT group_jid, user_jid, role, joined_at FROM group_participants WHERE group_jid = ?", groupJID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []GroupParticipant
	for rows.Next() {
		var p GroupParticipant
		var joinedAt sql.NullTime
		err := rows.Scan(&p.GroupJID, &p.UserJID, &p.Role, &joinedAt)
		if err != nil {
			return nil, err
		}
		if joinedAt.Valid {
			p.JoinedAt = joinedAt.Time
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

func (r *SQLiteGroupRepo) Delete(ctx context.Context, jid string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM groups WHERE jid = ?", jid)
	return err
}

// SQLiteStatusRepo implements StatusRepository.
type SQLiteStatusRepo struct {
	db *sql.DB
}

func (r *SQLiteStatusRepo) Store(ctx context.Context, status *StatusUpdate) error {
	query := `
		INSERT OR REPLACE INTO status_updates (id, sender_jid, media_type, content, posted_at, expires_at, viewed)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query, status.ID, status.SenderJID, status.MediaType, status.Content, status.PostedAt, status.ExpiresAt, status.Viewed)
	return err
}

func (r *SQLiteStatusRepo) GetAll(ctx context.Context) ([]StatusUpdate, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, sender_jid, media_type, content, posted_at, expires_at, viewed FROM status_updates WHERE expires_at > ? ORDER BY posted_at DESC", time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanStatuses(rows)
}

func (r *SQLiteStatusRepo) GetByContact(ctx context.Context, contactJID string) ([]StatusUpdate, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, sender_jid, media_type, content, posted_at, expires_at, viewed FROM status_updates WHERE sender_jid = ? AND expires_at > ? ORDER BY posted_at DESC", contactJID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanStatuses(rows)
}

func (r *SQLiteStatusRepo) Delete(ctx context.Context, statusID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM status_updates WHERE id = ?", statusID)
	return err
}

func (r *SQLiteStatusRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM status_updates WHERE expires_at <= ?", time.Now())
	return err
}

func scanStatuses(rows *sql.Rows) ([]StatusUpdate, error) {
	var statuses []StatusUpdate
	for rows.Next() {
		var s StatusUpdate
		err := rows.Scan(&s.ID, &s.SenderJID, &s.MediaType, &s.Content, &s.PostedAt, &s.ExpiresAt, &s.Viewed)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, s)
	}
	return statuses, rows.Err()
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

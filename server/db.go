package server

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChatMessage struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversation_id"`
	ParentMessageID string    `json:"parent_message_id"`
	Role            string    `json:"role"`
	Content         string    `json:"content"`
	Rounds          string    `json:"rounds"`
	CreatedAt       time.Time `json:"created_at"`
}

type DB struct {
	db *sql.DB
}

func NewDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	d := &DB{db: db}
	if err := d.initSchema(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		archived INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS chat_messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		parent_message_id TEXT,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		rounds TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id)
	);

	CREATE INDEX IF NOT EXISTS idx_messages_conversation ON chat_messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_messages_parent ON chat_messages(parent_message_id);
	`

	_, err := d.db.Exec(schema)
	return err
}

func (d *DB) CreateConversation(ctx context.Context, conv *Conversation) error {
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO conversations (id, title, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		conv.ID, conv.Title, conv.Archived, conv.CreatedAt, conv.UpdatedAt)
	return err
}

func (d *DB) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	conv := &Conversation{}
	err := d.db.QueryRowContext(ctx,
		"SELECT id, title, archived, created_at, updated_at FROM conversations WHERE id = ?", id).
		Scan(&conv.ID, &conv.Title, &conv.Archived, &conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return conv, nil
}

func (d *DB) ListConversations(ctx context.Context) ([]*Conversation, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT id, title, archived, created_at, updated_at FROM conversations ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []*Conversation
	for rows.Next() {
		conv := &Conversation{}
		if err := rows.Scan(&conv.ID, &conv.Title, &conv.Archived, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, conv)
	}
	return convs, rows.Err()
}

func (d *DB) UpdateConversation(ctx context.Context, id string, updatedAt time.Time) error {
	_, err := d.db.ExecContext(ctx,
		"UPDATE conversations SET updated_at = ? WHERE id = ?", updatedAt, id)
	return err
}

func (d *DB) UpdateConversationTitle(ctx context.Context, id string, title string) error {
	_, err := d.db.ExecContext(ctx,
		"UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?", title, time.Now(), id)
	return err
}

func (d *DB) ArchiveConversation(ctx context.Context, id string, archived bool) error {
	_, err := d.db.ExecContext(ctx,
		"UPDATE conversations SET archived = ?, updated_at = ? WHERE id = ?", archived, time.Now(), id)
	return err
}

func (d *DB) DeleteConversation(ctx context.Context, id string) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM chat_messages WHERE conversation_id = ?", id); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM conversations WHERE id = ?", id); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *DB) ListMessages(ctx context.Context, conversationID string) ([]*ChatMessage, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT id, conversation_id, parent_message_id, role, content, rounds, created_at FROM chat_messages WHERE conversation_id = ? ORDER BY created_at ASC",
		conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*ChatMessage
	for rows.Next() {
		msg := &ChatMessage{}
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.ParentMessageID, &msg.Role, &msg.Content, &msg.Rounds, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

func (d *DB) CreateMessage(ctx context.Context, msg *ChatMessage) error {
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO chat_messages (id, conversation_id, parent_message_id, role, content, rounds, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		msg.ID, msg.ConversationID, msg.ParentMessageID, msg.Role, msg.Content, msg.Rounds, msg.CreatedAt)
	return err
}

func (d *DB) GetMessageChain(ctx context.Context, conversationID, messageID string) ([]*ChatMessage, error) {
	var messages []*ChatMessage
	currentID := messageID

	for currentID != "" {
		msg := &ChatMessage{}
		err := d.db.QueryRowContext(ctx,
			"SELECT id, conversation_id, parent_message_id, role, content, rounds, created_at FROM chat_messages WHERE id = ?",
			currentID).Scan(&msg.ID, &msg.ConversationID, &msg.ParentMessageID, &msg.Role, &msg.Content, &msg.Rounds, &msg.CreatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}
			return nil, err
		}
		messages = append([]*ChatMessage{msg}, messages...)
		currentID = msg.ParentMessageID
	}

	return messages, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

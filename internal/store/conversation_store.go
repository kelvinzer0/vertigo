package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Message represents a single message in a conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Conversation represents a single conversation history.
type Conversation struct {
	ID          string
	Messages    []Message
	LastUpdated time.Time
}

// ConversationStore manages conversation histories using a SQLite database.
type ConversationStore struct {
	db *sql.DB
}

// NewConversationStore creates a new ConversationStore with a database connection.
func NewConversationStore(db *sql.DB) *ConversationStore {
	return &ConversationStore{
		db: db,
	}
}

// GetConversation retrieves a conversation by ID. Creates a new one if not found.
func (cs *ConversationStore) GetConversation(id string) (*Conversation, error) {
	if id == "" {
		id = uuid.New().String() // Generate a new ID if not provided
	}

	conv := &Conversation{ID: id, Messages: []Message{}}

	// Try to load existing conversation
	row := cs.db.QueryRow("SELECT last_updated FROM conversations WHERE id = ?", id)
	var lastUpdated int64
	err := row.Scan(&lastUpdated)
	if err == sql.ErrNoRows {
		// Conversation does not exist, create it
		_, err = cs.db.Exec("INSERT INTO conversations (id, last_updated) VALUES (?, ?)", id, time.Now().Unix())
		if err != nil {
			return nil, fmt.Errorf("failed to create new conversation: %w", err)
		}
		conv.LastUpdated = time.Now()
		return conv, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query conversation: %w", err)
	}

	conv.LastUpdated = time.Unix(lastUpdated, 0)

	// Load messages for the conversation
	rows, err := cs.db.Query("SELECT role, content FROM messages WHERE conversation_id = ? ORDER BY timestamp ASC", id)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.Role, &msg.Content); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		conv.Messages = append(conv.Messages, msg)
	}

	return conv, nil
}

// AddMessage adds a message to a conversation's history and persists it to DB.
func (cs *ConversationStore) AddMessage(conversationID string, role, content string) error {
	conv, err := cs.GetConversation(conversationID)
	if err != nil {
		return fmt.Errorf("failed to get conversation to add message: %w", err)
	}

	_, err = cs.db.Exec("INSERT INTO messages (conversation_id, role, content, timestamp) VALUES (?, ?, ?, ?)",
		conversationID, role, content, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Update last_updated for the conversation
	_, err = cs.db.Exec("UPDATE conversations SET last_updated = ? WHERE id = ?", time.Now().Unix(), conversationID)
	if err != nil {
		return fmt.Errorf("failed to update conversation last_updated: %w", err)
	}

	conv.Messages = append(conv.Messages, Message{Role: role, Content: content})
	return nil
}

// ClearConversation removes a conversation and its messages from the store.
func (cs *ConversationStore) ClearConversation(id string) error {
	tx, err := cs.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback on error

	_, err = tx.Exec("DELETE FROM messages WHERE conversation_id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	_, err = tx.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return tx.Commit()
}
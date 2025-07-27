package store

import (
	"sync"
	"time"
)

// Message represents a single message in a conversation.
type Message struct {
	Role string `json:"role"`
	Content string `json:"content"`
}

// Conversation represents a single conversation history.
type Conversation struct {
	Messages []Message
	LastUpdated time.Time
}

// ConversationStore manages conversation histories in memory.
type ConversationStore struct {
	store sync.Map // map[string]*Conversation
	mu    sync.Mutex // Protects access to the map for operations like cleanup
}

// NewConversationStore creates a new in-memory conversation store.
func NewConversationStore() *ConversationStore {
	cs := &ConversationStore{}
	// Optional: Start a goroutine for periodic cleanup of old conversations
	// go cs.cleanupOldConversations(1 * time.Hour, 24 * time.Hour)
	return cs
}

// GetConversation retrieves a conversation by ID. Creates a new one if not found.
func (cs *ConversationStore) GetConversation(id string) *Conversation {
	if actual, loaded := cs.store.Load(id); loaded {
		conv := actual.(*Conversation)
		conv.LastUpdated = time.Now()
		return conv
	}

	newConv := &Conversation{
		Messages:    []Message{},
		LastUpdated: time.Now(),
	}
	cs.store.Store(id, newConv)
	return newConv
}

// AddMessage adds a message to a conversation's history.
func (cs *ConversationStore) AddMessage(conversationID string, role, content string) {
	conv := cs.GetConversation(conversationID)
	conv.Messages = append(conv.Messages, Message{Role: role, Content: content})
	conv.LastUpdated = time.Now()
}

// ClearConversation removes a conversation from the store.
func (cs *ConversationStore) ClearConversation(id string) {
	cs.store.Delete(id)
}

// cleanupOldConversations periodically removes conversations older than a certain duration.
// func (cs *ConversationStore) cleanupOldConversations(checkInterval, maxAge time.Duration) {
// 	for range time.Tick(checkInterval) {
// 		cs.mu.Lock()
// 		now := time.Now()
// 		cs.store.Range(func(key, value interface{}) bool {
// 			conv := value.(*Conversation)
// 			if now.Sub(conv.LastUpdated) > maxAge {
// 				cs.store.Delete(key)
// 			}
// 			return true
// 		})
// 		cs.mu.Unlock()
// 	}
// }

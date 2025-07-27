package proxy

import (
	"encoding/json"
	"fmt"
	"io"

	"vertigo/internal/gemini"
	"vertigo/internal/store"
	"time"

	"github.com/sirupsen/logrus"
)

// Manager handles API key rotation, model selection, and request forwarding.
type Manager struct {
	KeyManager      *KeyManager
	ConversationStore *store.ConversationStore
	GeminiClient    *gemini.Client
	Log             *logrus.Logger
}

// NewManager creates a new proxy Manager.
func NewManager(keyManager *KeyManager, convStore *store.ConversationStore, logger *logrus.Logger) *Manager {
	return &Manager{
		KeyManager:      keyManager,
		ConversationStore: convStore,
		GeminiClient:    gemini.NewClient(logger),
		Log:             logger,
	}
}

// ProcessRequest processes an incoming request, selects a model, rotates API keys, and forwards to Gemini.
func (pm *Manager) ProcessRequest(requestBody []byte, conversationID string, stream bool) (io.ReadCloser, error) {
	// Select the model and potentially modify the request body
	_, modifiedBodyBytes, err := SelectModel(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to select model: %w", err)
	}

	var reqBodyMap map[string]interface{}
	if err := json.Unmarshal(modifiedBodyBytes, &reqBodyMap); err != nil {
		return nil, fmt.Errorf("failed to parse modified request body: %w", err)
	}

	// Get conversation history and add to request
	conv, err := pm.ConversationStore.GetConversation(conversationID)
	if err != nil {
		pm.Log.Printf("Error getting conversation: %v", err)
		// Continue without conversation history if there's an error
	}

	if conv != nil && len(conv.Messages) > 0 {
		// Assuming the request body has a "messages" field
		if messages, ok := reqBodyMap["messages"].([]interface{}); ok {
			for _, msg := range conv.Messages {
				messages = append([]interface{}{map[string]string{"role": msg.Role, "content": msg.Content}}, messages...)
			}
			reqBodyMap["messages"] = messages
		}
	}

	finalRequestBody, err := json.Marshal(reqBodyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal final request body: %w", err)
	}

	pm.Log.Debugf("Sending request to Gemini API: %s", finalRequestBody)

	// Get the next API key
	apiKey := pm.KeyManager.GetNextAvailableKey()
	if apiKey == "" {
		return nil, fmt.Errorf("no API keys available")
	}

	// Send request to Gemini API
	geminiResponseReader, err := pm.GeminiClient.ChatCompletions(apiKey, finalRequestBody, stream)
	if err != nil {
		pm.Log.Errorf("Gemini API call failed for key %s: %v", apiKey, err)
		pm.KeyManager.MarkKeyAsBad(apiKey, 5*time.Minute) // Mark key as bad for 5 minutes
		return nil, fmt.Errorf("failed to get response from Gemini API: %w", err)
	}

	return geminiResponseReader, nil
}

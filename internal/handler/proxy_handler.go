package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"vertigo/internal/gemini"
	"vertigo/internal/proxy"
	"vertigo/internal/store"

	"github.com/sirupsen/logrus"
)

// Define a custom context key type to avoid collisions.
type contextKey string

const ( 
	conversationIDContextKey contextKey = "conversationID"
)

// OpenAIChatRequest represents the incoming request format for OpenAI chat completions.
type OpenAIChatRequest struct {
	Model          string        `json:"model"`
	Messages       []store.Message `json:"messages"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`	
	ConversationID string        `json:"conversation_id,omitempty"` // New field for context sharing
	// Add other relevant fields like temperature, max_tokens, etc.
}

// NewProxyHandler creates a new reverse proxy handler.
func NewProxyHandler(keyRotator *proxy.KeyRotator, convStore *store.ConversationStore, log *logrus.Logger) http.HandlerFunc {
	target, _ := url.Parse("https://generativelanguage.googleapis.com")

	reverseProxy := httputil.NewSingleHostReverseProxy(target)

	reverseProxy.Director = func(req *http.Request) {
		// Read the original request body
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorf("Failed to read chat request body: %v", err)
			return
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body)) // Restore body for later use if needed

		var openAIReq OpenAIChatRequest
		if err := json.Unmarshal(body, &openAIReq); err != nil {
			log.Errorf("Failed to unmarshal OpenAI chat request: %v", err)
			return
		}

		// Determine the actual Gemini model to use (vertigo-1.0-blast logic)
		// This returns the original request body with the 'model' field updated
		_, modifiedOriginalBody, err := proxy.SelectModel(body)
		if err != nil {
			log.Errorf("Failed to select model for chat: %v", err)
			return	
		}

		// --- Conversation Context Handling ---
		conversationID := openAIReq.ConversationID
		if conversationID == "" {
			// Generate a new conversation ID if not provided by the client
			conversationID = time.Now().Format("20060102150405") // Simple ID for now
		}

		// Store conversationID in request context for ModifyResponse to access
		ctx := context.WithValue(req.Context(), conversationIDContextKey, conversationID)
		req = req.WithContext(ctx)

		conversation, err := convStore.GetConversation(conversationID)
		if err != nil {
			log.Errorf("Failed to get conversation %s: %v", conversationID, err)
			return
		}

		// Append current user message to conversation history in store
		if len(openAIReq.Messages) > 0 {
			lastUserMessage := openAIReq.Messages[len(openAIReq.Messages)-1]
			if lastUserMessage.Role == "user" {
				err = convStore.AddMessage(conversationID, lastUserMessage.Role, lastUserMessage.Content)
				if err != nil {
					log.Errorf("Failed to add user message to conversation %s: %v", conversationID, err)
					return
				}
			}
		}

		// Re-fetch conversation to get the latest state including the just-added user message
		conversation, err = convStore.GetConversation(conversationID)
		if err != nil {
			log.Errorf("Failed to re-fetch conversation %s after adding user message: %v", conversationID, err)
			return
		}

		// Now, construct the final outgoing request body for Google's OpenAI-compatible endpoint.
		// We start with the modifiedOriginalBody (which has the correct model name)
		// and then inject the full conversation history into its 'messages' field.
		var finalOutgoingReq OpenAIChatRequest // Use OpenAIChatRequest as the target structure
		if err := json.Unmarshal(modifiedOriginalBody, &finalOutgoingReq); err != nil {
			log.Errorf("Failed to unmarshal modifiedOriginalBody: %v", err)
			return
		}
		finalOutgoingReq.Messages = conversation.Messages // Overwrite messages with full conversation history

		finalOutgoingBody, err := json.Marshal(finalOutgoingReq)
		if err != nil {
			log.Errorf("Failed to marshal final outgoing chat request: %v", err)
			return
		}

		// Set the modified body for the outgoing request
		req.Body = ioutil.NopCloser(bytes.NewBuffer(finalOutgoingBody))
		req.ContentLength = int64(len(finalOutgoingBody))
		req.Header.Set("Content-Type", "application/json")

		// Set the API key
		apiKey := keyRotator.GetNextKey()
		req.Header.Set("Authorization", "Bearer "+apiKey)

		// *** CRITICAL FIX: Explicitly set the entire req.URL to ensure the correct path is used ***
		req.URL = &url.URL{
			Scheme: target.Scheme,
			Host:   target.Host,
			Path:   "/v1beta/openai/chat/completions",
		}
		req.Host = target.Host // Ensure Host header is correct
	}

	reverseProxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return nil
		}

		// Retrieve conversationID from the request context
		conversationID, ok := resp.Request.Context().Value(conversationIDContextKey).(string)
		if !ok || conversationID == "" {
			log.Warn("Conversation ID not found in context for response.")
			return nil // Cannot update conversation history without ID
		}

		// Read the response body from Gemini
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("Failed to read Gemini chat response body: %v", err)
			return err
		}
		resp.Body.Close() // Close the original body

		var geminiResp gemini.ChatResponse
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			log.Errorf("Failed to unmarshal Gemini chat response: %v", err)
			return err
		}

		// Extract Gemini's response message
		var geminiResponseMessage string
		if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
			geminiResponseMessage = geminiResp.Candidates[0].Content.Parts[0].Text
		}

		// --- Update Conversation Context with Assistant's Response ---
		if geminiResponseMessage != "" {
			err = convStore.AddMessage(conversationID, "assistant", geminiResponseMessage)
			if err != nil {
				log.Errorf("Failed to add assistant message to conversation %s: %v", conversationID, err)
				// Do not return error here, as we still want to send the response to the client
			}
		}

		// Re-marshal the Gemini response back to OpenAI format for the client
		var openAIResp struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
				Index        int    `json:"index"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		openAIResp.ID = "chatcmpl-" + time.Now().Format("20060102150405")
		openAIResp.Object = "chat.completion"
		openAIResp.Created = time.Now().Unix()
		openAIResp.Model = "vertigo-1.0-blast" // Or the actual model used

		openAIResp.Choices = make([]struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
			Index        int    `json:"index"`
		}, 1)
		openAIResp.Choices[0].Message.Role = "assistant"
		openAIResp.Choices[0].Message.Content = geminiResponseMessage
		openAIResp.Choices[0].FinishReason = "stop"
		openAIResp.Choices[0].Index = 0

		openAIResp.Usage.PromptTokens = geminiResp.UsageMetadata.PromptTokenCount
		openAIResp.Usage.CompletionTokens = geminiResp.UsageMetadata.TotalTokenCount - geminiResp.UsageMetadata.PromptTokenCount
		openAIResp.Usage.TotalTokens = geminiResp.UsageMetadata.TotalTokenCount

		modifiedBody, err := json.Marshal(openAIResp)
		if err != nil {
			return err
		}

		resp.Body = ioutil.NopCloser(bytes.NewBuffer(modifiedBody))
		resp.ContentLength = int64(len(modifiedBody))
		resp.Header.Set("Content-Type", "application/json")

		return nil
	}

	return func(w http.ResponseWriter, r *http.Request) {
		reverseProxy.ServeHTTP(w, r)
	}
}

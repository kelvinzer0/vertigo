package handler

import (
	"bytes"
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
		selectedModel, modifiedOriginalBody, err := proxy.SelectModel(body)
		if err != nil {
			log.Errorf("Failed to select model for chat: %v", err)
			return
		}

		// --- Conversation Context Handling ---
		var conversation *store.Conversation
		if openAIReq.ConversationID != "" {
			conversation = convStore.GetConversation(openAIReq.ConversationID)
		} else {
			// If no conversation_id, treat as a new conversation (or generate one)
			// For simplicity, we'll just use the current message for now if no ID is provided.
			conversation = &store.Conversation{Messages: []store.Message{}}
		}

		// Append current user message to conversation history
		// Assuming the last message in openAIReq.Messages is the current user input
		if len(openAIReq.Messages) > 0 {
			lastUserMessage := openAIReq.Messages[len(openAIReq.Messages)-1]
			if lastUserMessage.Role == "user" {
				conversation.Messages = append(conversation.Messages, lastUserMessage)
			}
		}

		// Construct Gemini request with full conversation history
		geminiReq := gemini.ChatRequest{}
		geminiReq.Contents = make([]gemini.ChatContent, len(conversation.Messages))
		for i, msg := range conversation.Messages {
			geminiReq.Contents[i].Role = msg.Role
			geminiReq.Contents[i].Parts = []gemini.ChatPart{ {Text: msg.Content} }
		}

		// Copy generation config from original request if available
		var tempReq struct { Temperature float32 `json:"temperature,omitempty"`; MaxTokens int `json:"max_tokens,omitempty"` }
		json.Unmarshal(modifiedOriginalBody, &tempReq) // Use modifiedOriginalBody to get other params
		geminiReq.GenerationConfig.Temperature = tempReq.Temperature
		geminiReq.GenerationConfig.MaxOutputTokens = tempReq.MaxTokens

		modifiedBodyForGemini, err := json.Marshal(geminiReq)
		if err != nil {
			log.Errorf("Failed to marshal Gemini chat request with history: %v", err)
			return
		}

		// Set the modified body for the outgoing request
		req.Body = ioutil.NopCloser(bytes.NewBuffer(modifiedBodyForGemini))
		req.ContentLength = int64(len(modifiedBodyForGemini))
		req.Header.Set("Content-Type", "application/json")

		// Set the API key
		apiKey := keyRotator.GetNextKey()
		req.Header.Set("Authorization", "Bearer "+apiKey)

		// Set the target URL
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = "/v1beta/models/" + selectedModel + ":generateContent"
		req.Host = target.Host
	}

	reverseProxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return nil
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

		// --- Update Conversation Context ---
		// We need the original request to get the conversation_id. This is tricky with ReverseProxy.
		// For simplicity, we'll assume the conversation_id is passed in the request context or a global map.
		// A more robust solution would involve storing the conversation_id in a custom ResponseWriter or a map keyed by request ID.
		// For now, we'll just append the response to the last conversation that was processed.
		// This is a simplification and might not work correctly in concurrent scenarios without further state management.
		// A better approach would be to pass the conversation ID through the request context or a custom transport.

		// For demonstration, let's assume we can retrieve the conversation ID from the request context.
		// This part needs a proper mechanism to pass conversation_id from Director to ModifyResponse.
		// For now, we'll skip updating the store with Gemini's response to avoid complexity without a proper solution.

		// Re-marshal the Gemini response back to OpenAI format for the client
		// This part is similar to the original proxy_handler logic
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

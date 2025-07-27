package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"vertigo/internal/proxy"

	"github.com/sirupsen/logrus"
)

// OpenAIAPI represents the OpenAI-compatible API handlers.
type OpenAIAPI struct {
	ProxyManager *proxy.Manager
	Log          *logrus.Logger
}

// NewOpenAIAPI creates a new OpenAIAPI instance.
func NewOpenAIAPI(proxyManager *proxy.Manager, logger *logrus.Logger) *OpenAIAPI {
	return &OpenAIAPI{
		ProxyManager: proxyManager,
		Log:          logger,
	}
}

// ChatCompletionsHandler handles requests to the /openai/v1/chat/completions endpoint.
func (api *OpenAIAPI) ChatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		api.Log.Errorf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	var reqBodyMap map[string]interface{}
	if err := json.Unmarshal(body, &reqBodyMap); err != nil {
		api.Log.Errorf("Failed to unmarshal request body: %v", err)
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	stream := false
	if s, ok := reqBodyMap["stream"].(bool); ok && s {
		stream = true
	}

	// Extract conversation ID from headers or generate a new one
	conversationID := r.Header.Get("X-Conversation-ID")
	if conversationID == "" {
		// In a real application, you might want to generate a unique ID and return it to the client.
		// For now, we'll use a placeholder.
		conversationID = "default-conversation"
	}

	// Process the request using the proxy manager
	geminiResponseReader, err := api.ProxyManager.ProcessRequest(body, conversationID, stream)
	if err != nil {
		api.Log.Errorf("Failed to process request: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if stream {
		defer geminiResponseReader.Close() // Ensure the reader is closed after streaming
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		scanner := bufio.NewScanner(geminiResponseReader)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				jsonStr := strings.TrimPrefix(line, "data: ")
				if jsonStr == "[DONE]" {
					break
				}

				var geminiChunk map[string]interface{}
				if err := json.Unmarshal([]byte(jsonStr), &geminiChunk); err != nil {
					api.Log.Errorf("Failed to unmarshal Gemini stream chunk: %v", err)
					continue
				}

				api.Log.Debugf("Gemini Chunk: %+v", geminiChunk)

				// Extract content and finish_reason safely
				content := ""
				finishReason := interface{}(nil) // Use interface{} for nil or string

				if choices, ok := geminiChunk["choices"].([]interface{}); ok && len(choices) > 0 {
					api.Log.Debugf("Choices found: %+v", choices)
					if firstChoice, ok := choices[0].(map[string]interface{}); ok {
						api.Log.Debugf("First Choice: %+v", firstChoice)
						if delta, ok := firstChoice["delta"].(map[string]interface{}); ok {
							api.Log.Debugf("Delta: %+v", delta)
							if c, ok := delta["content"].(string); ok {
								content = c
								api.Log.Debugf("Content extracted: %s", content)
							}
						}
						if fr, ok := firstChoice["finish_reason"]; ok {
							finishReason = fr
							api.Log.Debugf("Finish Reason extracted: %+v", finishReason)
						}
					}
				}

				// Transform Gemini chunk to OpenAI SSE format
				openAIChunk := map[string]interface{}{
					"id":      "chatcmpl-test", // Placeholder ID
					"object":  "chat.completion.chunk",
					"created": 1678886400,
					"model":   reqBodyMap["model"],
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"delta": map[string]string{
								"content": content,
							},
							"finish_reason": finishReason,
						},
					},
				}

				jsonBytes, err := json.Marshal(openAIChunk);
				if err != nil {
					api.Log.Errorf("Failed to marshal OpenAI chunk: %v", err)
					continue
				}

				fmt.Fprintf(w, "data: %s\n\n", jsonBytes)
				w.(http.Flusher).Flush()
			}
		}

		if err := scanner.Err(); err != nil {
			api.Log.Errorf("Error reading Gemini stream: %v", err)
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.(http.Flusher).Flush()

	} else {
		// Non-streaming response (existing logic)
		geminiResponse, err := io.ReadAll(geminiResponseReader)
		if err != nil {
			api.Log.Errorf("Failed to read Gemini response: %v", err)
			http.Error(w, "Failed to read Gemini response", http.StatusInternalServerError)
			return
		}

		// Set headers before writing any body content
		w.Header().Del("Content-Type") // Ensure no conflicting Content-Type header exists
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if len(geminiResponse) == 0 {
			api.Log.Warnf("Received empty Gemini response for non-streaming request.")
			json.NewEncoder(w).Encode(map[string]string{"error": "empty response from Gemini API"})
			return
		}

		api.Log.Debugf("Raw Gemini Response (non-streaming): %s", geminiResponse) // Log raw response

		// Unmarshal and re-marshal to ensure valid JSON output
		var jsonResponse interface{}
		if err := json.Unmarshal(geminiResponse, &jsonResponse); err != nil {
			api.Log.Errorf("Failed to unmarshal Gemini response: %v", err)
			http.Error(w, "Failed to process Gemini response", http.StatusInternalServerError)
			return
		}

		finalResponse, err := json.Marshal(jsonResponse)
		if err != nil {
			api.Log.Errorf("Failed to marshal final response: %v", err)
			http.Error(w, "Failed to process Gemini response", http.StatusInternalServerError)
			return
		}

		api.Log.Debugf("Final Response (non-streaming): %s", finalResponse) // Log the final response
		w.Write(finalResponse)
	}
}

// ModelsHandler handles requests to the /openai/v1/models endpoint.
func (api *OpenAIAPI) ModelsHandler(w http.ResponseWriter, r *http.Request) {
	// This is a simplified implementation. In a real scenario, you might dynamically
	// fetch available models from Gemini API or maintain a more sophisticated list.
	models := []map[string]interface{}{
		{"id": "vertigo-1.0-blast", "object": "model", "created": 1678886400, "owned_by": "vertigo"},
		{"id": "gemini-2.0-flash", "object": "model", "created": 1678886400, "owned_by": "google"},
		{"id": "gemini-2.5-flash-lite", "object": "model", "created": 1678886400, "owned_by": "google"},
		{"id": "gemini-2.5-flash", "object": "model", "created": 1678886400, "owned_by": "google"},
		{"id": "gemini-2.5-pro", "object": "model", "created": 1678886400, "owned_by": "google"},
	}

	resp := map[string]interface{}{
		"object": "list",
		"data":   models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

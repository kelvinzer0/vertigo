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

	"github.com/sirupsen/logrus"
)

// OpenAICompletionsRequest represents the incoming request format for /v1/completions.
type OpenAICompletionsRequest struct {
	Model       string        `json:"model"`
	Prompt      interface{}   `json:"prompt"` // Can be string or array of strings
	MaxTokens   int           `json:"max_tokens"`
	Temperature float32       `json:"temperature"`
	// Add other relevant fields as needed
}

// OpenAICompletionsResponse represents the outgoing response format for /v1/completions.
type OpenAICompletionsResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Text         string `json:"text"`
		Index        int    `json:"index"`
		LogProbs     interface{} `json:"logprobs"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewCompletionsHandler creates a new reverse proxy handler for the /v1/completions endpoint.
func NewCompletionsHandler(keyRotator *proxy.KeyRotator, log *logrus.Logger) http.HandlerFunc {
	// Map OpenAI legacy models to Gemini models
	modelMap := map[string]string{
		"text-davinci-003": proxy.ModelGeminiPro,
		"gpt-3.5-turbo-instruct": proxy.ModelGeminiPro,
		// Add more mappings as needed
	}

	target, _ := url.Parse("https://generativelanguage.googleapis.com")

	reverseProxy := httputil.NewSingleHostReverseProxy(target)

	reverseProxy.Director = func(req *http.Request) {
		// Read the original request body
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorf("Failed to read completions request body: %v", err)
			return
		}

		// Unmarshal into OpenAI completions format
		var openAIReq OpenAICompletionsRequest
		if err := json.Unmarshal(body, &openAIReq); err != nil {
			log.Errorf("Failed to unmarshal OpenAI completions request: %v", err)
			return
		}

		// Transform to Gemini chat format
		geminiReq := gemini.ChatRequest{}
		geminiReq.Contents = make([]gemini.ChatContent, 1)
		geminiReq.Contents[0].Role = "user"
		
		switch p := openAIReq.Prompt.(type) {
		case string:
			geminiReq.Contents[0].Parts = []gemini.ChatPart{ {Text: p} }
		case []interface{}:
			// Handle array of strings for prompt
			var fullPrompt string
			for _, item := range p {
				if s, ok := item.(string); ok {
					fullPrompt += s + "\n"
				}
			}
			geminiReq.Contents[0].Parts = []gemini.ChatPart{ {Text: fullPrompt} }
		}

		geminiReq.GenerationConfig.MaxOutputTokens = openAIReq.MaxTokens
		geminiReq.GenerationConfig.Temperature = openAIReq.Temperature

		modifiedBody, err := json.Marshal(geminiReq)
		if err != nil {
			log.Errorf("Failed to marshal Gemini chat request: %v", err)
			return
		}

		// Set the modified body for the outgoing request
		req.Body = ioutil.NopCloser(bytes.NewBuffer(modifiedBody))
		req.ContentLength = int64(len(modifiedBody))
		req.Header.Set("Content-Type", "application/json")

		// Get the next API key using the rotator
		apiKey := keyRotator.GetNextKey()
		req.Header.Set("Authorization", "Bearer "+apiKey)

		// Set the correct target URL for Gemini's chat API
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = "/v1beta/models/" + modelMap[openAIReq.Model] + ":generateContent"
		req.Host = target.Host
	}

	reverseProxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return nil
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close() // We must close the original body

		var geminiResp gemini.ChatResponse
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			return err
		}

		openAIResp := OpenAICompletionsResponse{
			ID:      "cmpl-" + time.Now().Format("20060102150405"), // Generate a unique ID
			Object:  "text_completion",
			Created: time.Now().Unix(),
			Model:   "text-davinci-003", // Or the original model from request if stored
		}

		if len(geminiResp.Candidates) > 0 {
			openAIResp.Choices = make([]struct {
				Text         string `json:"text"`
				Index        int    `json:"index"`
				LogProbs     interface{} `json:"logprobs"`
				FinishReason string `json:"finish_reason"`
			}, 1)
			openAIResp.Choices[0].Text = geminiResp.Candidates[0].Content.Parts[0].Text
			openAIResp.Choices[0].Index = 0
			openAIResp.Choices[0].FinishReason = "stop" // Default for now
		}

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

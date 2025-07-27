package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const ( 
	GeminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
)

// Client for interacting with the Gemini API.
type Client struct {
	HTTPClient *http.Client
	Log        *logrus.Logger
}

// NewClient creates a new Gemini API client.
func NewClient(logger *logrus.Logger) *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		Log: logger,
	}
}

// ChatCompletions sends a chat completions request to the Gemini API.
func (c *Client) ChatCompletions(apiKey string, requestBody []byte, stream bool) (io.ReadCloser, error) {
	c.Log.Debugf("Gemini API Request (stream=%t): %s", stream, requestBody)

	req, err := http.NewRequest("POST", GeminiAPIURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if stream {
		// Modify the request body to include the stream parameter
		var reqMap map[string]interface{}
		if err := json.Unmarshal(requestBody, &reqMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal request body for streaming: %w", err)
		}
		reqMap["stream"] = true
		modifiedBody, err := json.Marshal(reqMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal modified request body for streaming: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewBuffer(modifiedBody))
		req.ContentLength = int64(len(modifiedBody))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	c.Log.Debugf("Gemini API Response Status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		c.Log.Errorf("Gemini API Error Response Body: %s", respBody)
		return nil, fmt.Errorf("Gemini API returned non-200 status: %d, body: %s", resp.StatusCode, respBody)
	}

	// If not streaming, read the entire body and return a new reader
	if !stream {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		resp.Body.Close()
		c.Log.Debugf("Gemini API Full Response Body (non-stream): %s", respBody)
		return io.NopCloser(bytes.NewBuffer(respBody)), nil
	}

	// For streaming, return the original response body reader
	return resp.Body, nil
}

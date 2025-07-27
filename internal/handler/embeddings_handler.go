package handler

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"

	"vertigo/internal/proxy"

	"github.com/sirupsen/logrus"
)

// OpenAIEmbeddingRequest represents the incoming request format from an OpenAI client.
type OpenAIEmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

// GoogleEmbeddingRequest represents the format for the Google AI API.
type GoogleEmbeddingRequest struct {
	Content struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"content"`
}

// GoogleEmbeddingResponse represents the successful response from Google's API.
type GoogleEmbeddingResponse struct {
	Embedding struct {
		Value []float32 `json:"value"`
	} `json:"embedding"`
}

// OpenAIEmbeddingResponse represents the format expected by the OpenAI client.
type OpenAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// NewEmbeddingHandler creates a new reverse proxy handler for the embeddings endpoint.
func NewEmbeddingHandler(keyRotator *proxy.KeyRotator, log *logrus.Logger) http.HandlerFunc {
	const targetModel = "text-embedding-004"
	const openAIModelName = "text-embedding-ada-002" // The model we are mimicking

	target, _ := url.Parse("https://generativelanguage.googleapis.com")

	reverseProxy := httputil.NewSingleHostReverseProxy(target)

	reverseProxy.Director = func(req *http.Request) {
		// ... (director logic remains the same)
	}

	reverseProxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			// If the status code is not 200, we don't modify the response
			return nil
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close() // We must close the original body

		var googleResp GoogleEmbeddingResponse
		if err := json.Unmarshal(body, &googleResp); err != nil {
			return err
		}

		// Transform to OpenAI's format
		openAIResp := OpenAIEmbeddingResponse{
			Object: "list",
			Model:  openAIModelName,
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{
					Object:    "embedding",
					Embedding: googleResp.Embedding.Value,
					Index:     0,
				},
			},
			Usage: struct {
				PromptTokens int `json:"prompt_tokens"`
				TotalTokens  int `json:"total_tokens"`
			}{
				// Google's API doesn't provide token usage for embeddings, so we use 0.
				PromptTokens: 0,
				TotalTokens:  0,
			},
		}

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

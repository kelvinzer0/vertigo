package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"vertigo/internal/proxy"
)

// Model represents the structure of a single model in the OpenAI-compatible API.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelList represents the structure of the list of models.
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

var availableModels = []Model{
	{ID: proxy.ModelVertigoBlast, Object: "model", Created: time.Now().Unix(), OwnedBy: "vertigo"},
	{ID: proxy.ModelGeminiPro, Object: "model", Created: time.Now().Unix(), OwnedBy: "google"},
	{ID: proxy.ModelGeminiFlashPro, Object: "model", Created: time.Now().Unix(), OwnedBy: "google"},
	{ID: proxy.ModelGeminiFlash, Object: "model", Created: time.Now().Unix(), OwnedBy: "google"},
}

// ModelsHandler handles requests to /v1/models and /v1/models/{model_id}.
func ModelsHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/openai/v1/models")
	path = strings.Trim(path, "/")

	if path == "" {
		// Request is for /v1/models (List models)
		listModels(w, r)
	} else {
		// Request is for /v1/models/{model_id} (Retrieve model)
		retrieveModel(w, r, path)
	}
}

func listModels(w http.ResponseWriter, r *http.Request) {
	resp := ModelList{
		Object: "list",
		Data:   availableModels,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func retrieveModel(w http.ResponseWriter, r *http.Request, modelID string) {
	for _, model := range availableModels {
		if model.ID == modelID {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(model)
			return
		}
	}

	// Model not found
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "model not found"})
}

package proxy

import (
	"encoding/json"
)

const (
	ModelVertigoBlast     = "vertigo-1.0-blast"
	ModelGemini20Flash    = "gemini-2.0-flash"
	ModelGemini25FlashLite = "gemini-2.5-flash-lite"
	ModelGemini25Flash    = "gemini-2.5-flash"
	ModelGemini25Pro      = "gemini-2.5-pro"
)

// RequestBody represents the relevant fields from the incoming JSON request.
type RequestBody struct {
	Model            string `json:"model"`
	ReasoningEffort  string `json:"reasoning_effort,omitempty"`
}

// SelectModel determines the correct Gemini model to use based on the request body.
// It returns the model name and the modified request body.
func SelectModel(body []byte) (string, []byte, error) {
	var reqBody RequestBody
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return "", nil, err
	}

	if reqBody.Model != ModelVertigoBlast {
		// If the model is not vertigo-1.0-blast, we don't need to do anything.
		return reqBody.Model, body, nil
	}

	selectedModel := ModelGemini25Flash // Default model
	switch reqBody.ReasoningEffort {
	case "low":
		selectedModel = ModelGemini20Flash
	case "medium":
		selectedModel = ModelGemini25Flash
	case "high":
		selectedModel = ModelGemini25Pro
	}

	// Create a new map to represent the modified request body
	var bodyMap map[string]interface{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		return "", nil, err
	}

	// Set the new model and remove the reasoning_effort field
	bodyMap["model"] = selectedModel
	delete(bodyMap, "reasoning_effort")

	modifiedBody, err := json.Marshal(bodyMap)
	if err != nil {
		return "", nil, err
	}

	return selectedModel, modifiedBody, nil
}

package gemini

// ChatPart represents a part in Gemini's content.
type ChatPart struct {
	Text string `json:"text"`
}

// ChatContent represents a content block in Gemini's chat request.
type ChatContent struct {
	Role  string     `json:"role"`
	Parts []ChatPart `json:"parts"`
}

// ChatRequest represents the outgoing request format for Gemini's chat/completions.
type ChatRequest struct {
	Contents []ChatContent `json:"contents"`
	GenerationConfig struct {
		Temperature     float32 `json:"temperature,omitempty"`
		MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	} `json:"generationConfig"`
}

// CandidatePart represents a part in Gemini's candidate content.
type CandidatePart struct {
	Text string `json:"text"`
}

// CandidateContent represents a content block in Gemini's chat response candidate.
type CandidateContent struct {
	Parts []CandidatePart `json:"parts"`
}

// ChatResponse represents the incoming response format from Gemini's chat/completions.
type ChatResponse struct {
	Candidates []struct {
		Content CandidateContent `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount int `json:"promptTokenCount"`
		TotalTokenCount  int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

package ai

// GenerationRequest contains the context used to generate a new training email.
type GenerationRequest struct {
	TargetAudience         string `json:"target_audience"`
	RecipientRole          string `json:"recipient_role"`
	OrganizationContext    string `json:"organization_context"`
	Scenario               string `json:"scenario"`
	CustomScenario         string `json:"custom_scenario"`
	Language               string `json:"language"`
	TargetDifficulty       string `json:"target_difficulty"`
	AdditionalInstructions string `json:"additional_instructions"`
}

// Email represents an email produced by the AI generator.
type Email struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
}

// RevisionRequest contains the current email and the changes requested by the user.
type RevisionRequest struct {
	Email               Email  `json:"email"`
	Feedback            string `json:"feedback"`
	TargetAudience      string `json:"target_audience"`
	RecipientRole       string `json:"recipient_role"`
	OrganizationContext string `json:"organization_context"`
	Scenario            string `json:"scenario"`
	CustomScenario      string `json:"custom_scenario"`
	Language            string `json:"language"`
	TargetDifficulty    string `json:"target_difficulty"`
}

// Message represents a message sent to or received from the Ollama chat API.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the request body sent to Ollama.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Format   string    `json:"format,omitempty"`
}

// chatResponse contains the fields we need from an Ollama response.
type chatResponse struct {
	Model      string  `json:"model"`
	Message    Message `json:"message"`
	Done       bool    `json:"done"`
	DoneReason string  `json:"done_reason"`
}

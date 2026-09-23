package ai

import "testing"

func TestDetectMissingGreeting(t *testing.T) {
	tests := []struct {
		name     string
		email    Email
		expected int
	}{
		{"english greeting", Email{Text: "Hello {{.FirstName}},\nPlease review the document."}, 0},
		{"russian greeting", Email{Text: "Здравствуйте, {{.FirstName}}!\nПожалуйста, проверьте документ."}, 0},
		{"personalized opening", Email{Text: "{{.FirstName}},\nplease review the document."}, 0},
		{"missing greeting", Email{Text: "Your document is ready for review."}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectMissingGreeting(tt.email)
			if result.MinValue != tt.expected || result.MaxValue != tt.expected {
				t.Fatalf("expected %d, got %d-%d", tt.expected, result.MinValue, result.MaxValue)
			}
		})
	}
}

func TestDetectMissingPersonalization(t *testing.T) {
	withVariable := detectMissingPersonalization(Email{Text: "Hello {{.FirstName}},"})
	if withVariable.MaxValue != 0 {
		t.Fatalf("expected no missing-personalization cue")
	}

	withoutVariable := detectMissingPersonalization(Email{Text: "Hello team,"})
	if withoutVariable.MinValue != 1 || withoutVariable.MaxValue != 1 {
		t.Fatalf("expected exact cue 1, got %d-%d", withoutVariable.MinValue, withoutVariable.MaxValue)
	}
}

func TestDetectHiddenURLLinks(t *testing.T) {
	email := Email{HTML: `
		<a href="{{.URL}}">Review document</a>
		<a href="https://example.com">https://example.com</a>
		<a href="https://other.example/login">https://example.com/login</a>
		<a href="mailto:help@example.com">help@example.com</a>
	`}
	result := detectHiddenURLLinks(email)
	if result.MinValue != 2 || result.MaxValue != 2 {
		t.Fatalf("expected 2 hidden URL links, got %d-%d", result.MinValue, result.MaxValue)
	}
}

func TestDetectAttachments(t *testing.T) {
	context := EvaluationContext{Attachments: AttachmentContext{
		Usage: AttachmentUsageUsed,
		Files: []AttachmentMetadata{{Name: "invoice.pdf"}, {Name: "archive.zip"}},
	}}
	result := detectAttachments(context)
	if result.MinValue != 2 || result.MaxValue != 2 {
		t.Fatalf("expected attachment count 2, got %d-%d", result.MinValue, result.MaxValue)
	}
}

func TestDetectAttachmentsUnknownUsesClassificationCeiling(t *testing.T) {
	result := detectAttachments(EvaluationContext{})
	if result.MinValue != 0 || result.MaxValue != CueCountManyMin {
		t.Fatalf("expected attachment range 0-%d, got %d-%d", CueCountManyMin, result.MinValue, result.MaxValue)
	}
}

func TestDetectDeterministicCueCriteriaReturnsTechnicalAndContentCriteria(t *testing.T) {
	input := EvaluationInput{
		Email: Email{Text: "No greeting or personalization.", HTML: `<a href="{{.URL}}">Open</a>`},
		EvaluationContext: EvaluationContext{
			SimulatedSender: &SenderIdentity{Email: "training@example.test"},
			ExpectedSender:  &SenderIdentity{Email: "training@example.test"},
			Link:            LinkContext{Usage: LinkUsageNone},
			Attachments:     AttachmentContext{Usage: AttachmentUsageNone},
		},
	}
	results := DetectDeterministicCueCriteria(input)
	if len(results) != 4 {
		t.Fatalf("expected 4 deterministic criteria, got %d", len(results))
	}
}

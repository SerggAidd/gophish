package ai

import "testing"

func TestDetectMissingGreeting(t *testing.T) {
	tests := []struct {
		name     string
		email    Email
		expected int
	}{
		{
			name: "english greeting",
			email: Email{
				Text: "Hello {{.FirstName}},\nPlease review the document.",
			},
			expected: 0,
		},
		{
			name: "russian greeting",
			email: Email{
				Text: "Здравствуйте, {{.FirstName}}!\nПожалуйста, проверьте документ.",
			},
			expected: 0,
		},
		{
			name: "personalized opening",
			email: Email{
				Text: "{{.FirstName}},\nplease review the document.",
			},
			expected: 0,
		},
		{
			name: "missing greeting",
			email: Email{
				Text: "Your document is ready for review.",
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectMissingGreeting(tt.email)

			if result.ID != CriterionMissingGreeting {
				t.Fatalf(
					"expected criterion %q, got %q",
					CriterionMissingGreeting,
					result.ID,
				)
			}

			if result.Source != CueSourceDeterministic {
				t.Fatalf(
					"expected source %q, got %q",
					CueSourceDeterministic,
					result.Source,
				)
			}

			if result.Value != tt.expected {
				t.Fatalf(
					"expected value %d, got %d",
					tt.expected,
					result.Value,
				)
			}
		})
	}
}

func TestDetectMissingPersonalization(t *testing.T) {
	tests := []struct {
		name     string
		email    Email
		expected int
	}{
		{
			name: "first name variable",
			email: Email{
				Text: "Hello {{.FirstName}},",
			},
			expected: 0,
		},
		{
			name: "position variable",
			email: Email{
				Subject: "Notice for {{.Position}}",
			},
			expected: 0,
		},
		{
			name: "missing personalization",
			email: Email{
				Text: "Hello team,",
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectMissingPersonalization(tt.email)

			if result.Value != tt.expected {
				t.Fatalf(
					"expected value %d, got %d",
					tt.expected,
					result.Value,
				)
			}
		})
	}
}

func TestDetectHiddenURLLinks(t *testing.T) {
	email := Email{
		HTML: `
			<p>
				<a href="{{.URL}}">Review document</a>
			</p>
			<p>
				<a href="https://example.com">https://example.com</a>
			</p>
			<p>
				<a href="https://other.example/login">https://example.com/login</a>
			</p>
			<p>
				<a href="mailto:help@example.com">help@example.com</a>
			</p>
		`,
	}

	result := detectHiddenURLLinks(email)

	if result.ID != CriterionHiddenURLLinks {
		t.Fatalf(
			"expected criterion %q, got %q",
			CriterionHiddenURLLinks,
			result.ID,
		)
	}

	if result.Source != CueSourceDeterministic {
		t.Fatalf(
			"expected source %q, got %q",
			CueSourceDeterministic,
			result.Source,
		)
	}

	if result.Value != 2 {
		t.Fatalf(
			"expected 2 hidden URL links, got %d",
			result.Value,
		)
	}

	if len(result.Evidence) != 2 {
		t.Fatalf(
			"expected 2 evidence entries, got %d",
			len(result.Evidence),
		)
	}
}

func TestDetectHiddenURLLinksMatchingDisplayedURL(t *testing.T) {
	email := Email{
		HTML: `<a href="https://www.example.com/">example.com</a>`,
	}

	result := detectHiddenURLLinks(email)

	if result.Value != 0 {
		t.Fatalf(
			"expected no hidden URL links, got %d",
			result.Value,
		)
	}
}

func TestDetectMissingGreetingFallsBackToHTML(t *testing.T) {
	email := Email{
		HTML: `<p>Dear {{.FirstName}},</p><p>Please review the document.</p>`,
	}

	result := detectMissingGreeting(email)

	if result.Value != 0 {
		t.Fatalf(
			"expected greeting to be detected from HTML, got %d",
			result.Value,
		)
	}
}

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const revisionGuardMaxAttempts = 2
const generationHTMLMaxAttempts = 2

// RevisionGroundingError reports a revision that repeatedly violated
// application-level grounding constraints. DifficultyAgent treats this as a
// rejected candidate rather than as a fatal workflow error.
type RevisionGroundingError struct {
	Attempts int
	Err      error
}

func (e *RevisionGroundingError) Error() string {
	if e == nil {
		return "revision violated grounding constraints"
	}
	return fmt.Sprintf("revision violated grounding constraints after %d attempts: %v", e.Attempts, e.Err)
}

func (e *RevisionGroundingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Generator creates and revises training email templates using an LLM.
type Generator struct {
	client *Client
}

func NewGenerator(client *Client) *Generator {
	return &Generator{client: client}
}

// Generate creates a training email without additional campaign context. It is
// kept for the existing API flow; new difficulty-aware flows should prefer
// GenerateWithContext.
func (g *Generator) Generate(
	ctx context.Context,
	req GenerationRequest,
) (Email, error) {
	return g.generate(ctx, req, nil)
}

// GenerateWithContext creates a training email while respecting fixed campaign
// factors already supplied for later difficulty evaluation.
func (g *Generator) GenerateWithContext(
	ctx context.Context,
	req GenerationRequest,
	evaluationContext EvaluationContext,
) (Email, error) {
	if err := evaluationContext.Validate(); err != nil {
		return Email{}, fmt.Errorf("validate evaluation context: %w", err)
	}
	return g.generate(ctx, req, &evaluationContext)
}

func (g *Generator) generate(
	ctx context.Context,
	req GenerationRequest,
	evaluationContext *EvaluationContext,
) (Email, error) {
	messages := []Message{
		{Role: "system", Content: generationSystemPrompt},
		{Role: "user", Content: buildGenerationPromptWithContext(req, evaluationContext)},
	}

	for attempt := 1; attempt <= generationHTMLMaxAttempts; attempt++ {
		response, err := g.client.Chat(ctx, messages, emailResponseSchema)
		if err != nil {
			return Email{}, fmt.Errorf("generate email: %w", err)
		}

		email, err := parseEmail(response)
		if err == nil {
			return email, nil
		}
		if !errors.Is(err, errUnusableEmailHTML) || attempt == generationHTMLMaxAttempts {
			return Email{}, fmt.Errorf("parse generated email after %d attempt(s): %w", attempt, err)
		}
		messages = append(messages,
			Message{Role: "assistant", Content: response},
			Message{Role: "user", Content: "The HTML field contained no readable email body. Return the complete email again with the full message visible in both the plain-text and HTML fields. Preserve the supplied facts and campaign context."},
		)
	}
	return Email{}, fmt.Errorf("generate email: no usable HTML after %d attempts", generationHTMLMaxAttempts)
}

func (g *Generator) Revise(
	ctx context.Context,
	req RevisionRequest,
) (Email, error) {
	messages := []Message{
		{Role: "system", Content: revisionSystemPrompt},
		{Role: "user", Content: buildRevisionPrompt(req)},
	}

	var lastValidationErr error
	for attempt := 1; attempt <= revisionGuardMaxAttempts; attempt++ {
		response, err := g.client.Chat(ctx, messages, emailResponseSchema)
		if err != nil {
			return Email{}, fmt.Errorf("revise email: %w", err)
		}

		email, err := parseEmail(response)
		if err != nil && !errors.Is(err, errUnusableEmailHTML) {
			return Email{}, fmt.Errorf("parse revised email: %w", err)
		}

		if err == nil {
			err = validateRevisionRequestCandidate(req, email)
		}
		if err == nil {
			return email, nil
		}
		lastValidationErr = err
		if attempt == revisionGuardMaxAttempts {
			break
		}
		messages = append(
			messages,
			Message{Role: "assistant", Content: response},
			Message{
				Role: "user",
				Content: fmt.Sprintf(
					"The revision violated application constraints: %s\nReturn a corrected revision with a complete, readable HTML body. Do not invent new email addresses, phone numbers, contact details, domains, URLs, or evaluator-only identity data.",
					err,
				),
			},
		)
	}

	return Email{}, &RevisionGroundingError{
		Attempts: revisionGuardMaxAttempts,
		Err:      lastValidationErr,
	}
}

func parseEmail(response string) (Email, error) {
	var email Email
	if err := json.Unmarshal([]byte(response), &email); err != nil {
		return Email{}, fmt.Errorf("invalid JSON returned by model: %w", err)
	}
	if strings.TrimSpace(email.Subject) == "" {
		return Email{}, fmt.Errorf("model returned an empty subject")
	}
	if strings.TrimSpace(email.Text) == "" {
		return Email{}, fmt.Errorf("model returned an empty plain text body")
	}
	if strings.TrimSpace(email.HTML) == "" {
		return Email{}, fmt.Errorf("model returned %w", errUnusableEmailHTML)
	}
	if err := validateEmailHTMLContent(email.HTML); err != nil {
		return Email{}, fmt.Errorf("model returned %w", err)
	}
	return email, nil
}

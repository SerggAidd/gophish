package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Generator creates and revises training email templates using an LLM.
type Generator struct {
	client *Client
}

// NewGenerator creates a new email generator.
func NewGenerator(client *Client) *Generator {
	return &Generator{
		client: client,
	}
}

// Generate creates a new training email using the supplied context.
func (g *Generator) Generate(
	ctx context.Context,
	req GenerationRequest,
) (Email, error) {
	messages := []Message{
		{
			Role:    "system",
			Content: generationSystemPrompt,
		},
		{
			Role:    "user",
			Content: buildGenerationPrompt(req),
		},
	}

	response, err := g.client.Chat(ctx, messages, true)
	if err != nil {
		return Email{}, fmt.Errorf("generate email: %w", err)
	}

	email, err := parseEmail(response)
	if err != nil {
		return Email{}, fmt.Errorf("parse generated email: %w", err)
	}

	return email, nil
}

// Revise updates an existing training email based on user feedback.
func (g *Generator) Revise(
	ctx context.Context,
	req RevisionRequest,
) (Email, error) {
	messages := []Message{
		{
			Role:    "system",
			Content: revisionSystemPrompt,
		},
		{
			Role:    "user",
			Content: buildRevisionPrompt(req),
		},
	}

	response, err := g.client.Chat(ctx, messages, true)
	if err != nil {
		return Email{}, fmt.Errorf("revise email: %w", err)
	}

	email, err := parseEmail(response)
	if err != nil {
		return Email{}, fmt.Errorf("parse revised email: %w", err)
	}

	return email, nil
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
		return Email{}, fmt.Errorf("model returned an empty HTML body")
	}

	return email, nil
}

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseEmailRejectsHTMLWithNoReadableMessage(t *testing.T) {
	email := Email{
		Subject: "Procurement review",
		Text:    "Please review the procurement request.",
		HTML:    `<table width="100%" style="font-family:Arial; color:#333;">`,
	}
	response, err := json.Marshal(email)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseEmail(string(response)); !errors.Is(err, errUnusableEmailHTML) {
		t.Fatalf("expected an unusable HTML error, got %v", err)
	}
	if err := validateInitialGenerationCandidate(email, EvaluationContext{}); !errors.Is(err, errUnusableEmailHTML) {
		t.Fatalf("agent must reject a draft with no HTML message, got %v", err)
	}
	email.HTML = `<table><tr><td>Please review the procurement request.</td></tr></table>`
	response, err = json.Marshal(email)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseEmail(string(response)); err != nil {
		t.Fatalf("expected a readable HTML email to pass, got %v", err)
	}
}

func TestGeneratorRetriesOnlyUnusableHTMLCandidate(t *testing.T) {
	invalid := Email{Subject: "Review", Text: "Read the notice.", HTML: `<table style="color:#333;">`}
	valid := Email{Subject: "Review", Text: "Read the notice.", HTML: "<p>Read the notice.</p>"}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		candidate := invalid
		if calls > 1 {
			candidate = valid
		}
		body, err := json.Marshal(candidate)
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]string{"content": string(body)},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	generated, err := NewGenerator(NewClient(server.URL, "fake")).Generate(context.Background(), GenerationRequest{})
	if err != nil {
		t.Fatalf("expected recovery from the invalid HTML candidate, got %v", err)
	}
	if calls != 2 || generated.HTML != valid.HTML {
		t.Fatalf("expected a second, usable draft; calls=%d, email=%#v", calls, generated)
	}
}

func TestRevisionRejectsUnusableHTMLWithoutLosingValidDraft(t *testing.T) {
	request := RevisionRequest{
		Email: Email{Subject: "Notice", Text: "Review the policy.", HTML: "<p>Review the policy.</p>"},
	}
	invalid := Email{Subject: "Updated notice", Text: "Please read the update.", HTML: "<table>"}
	valid := Email{Subject: "Updated notice", Text: "Please read the update.", HTML: "<p>Please read the update.</p>"}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		candidate := invalid
		if calls > 1 {
			candidate = valid
		}
		body, err := json.Marshal(candidate)
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]string{"content": string(body)},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	revised, err := NewGenerator(NewClient(server.URL, "fake")).Revise(context.Background(), request)
	if err != nil {
		t.Fatalf("expected recovery from the invalid HTML revision, got %v", err)
	}
	if calls != 2 || revised.HTML != valid.HTML {
		t.Fatalf("expected a second, usable revision; calls=%d, email=%#v", calls, revised)
	}
}

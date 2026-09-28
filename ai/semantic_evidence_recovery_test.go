package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSemanticCueEvaluationRecoversUnsupportedPositiveAfterRepair(t *testing.T) {
	modelResults := validSemanticCueModelResults()
	modelResults[CriterionPosesAsAuthority] = semanticCueModelResult{MinValue: 1, MaxValue: 1}
	modelResults[CriterionInconsistencies] = semanticCueModelResult{
		MinValue: 1, MaxValue: 1, Evidence: []string{"The instruction contradicts the subject."},
	}
	response := marshalSemanticCueResponse(t, modelResults)
	server, requests := semanticCueTestServer(t, "poses_as_authority", response, `{"min_value":1,"max_value":1,"evidence":[]}`)
	defer server.Close()

	results, err := NewSemanticCueEvaluator(NewClient(server.URL, "test")).Evaluate(
		context.Background(), EvaluationInput{Email: Email{
			Subject: "Policy Review Request - Action Required",
			Text:    "Please review the policy. No further action required.\n\nRegards,\nAlice Smith\nIT Support\nalice@windropolis.corporate",
		}},
	)
	if err != nil {
		t.Fatalf("a missing evidence string after repair should become unresolved: %v", err)
	}
	if *requests != 2 {
		t.Fatalf("expected initial request and repair, got %d", *requests)
	}
	for _, result := range results {
		if result.ID == CriterionPosesAsAuthority {
			if result.MinValue != 0 || result.MaxValue != 1 || result.Source != CueSourceHybrid ||
				result.EvaluationRepair != "unsupported_positive" ||
				len(result.Evidence) != 1 || !strings.Contains(result.Evidence[0], "omitted supporting evidence") {
				t.Fatalf("unsupported positive must be clearly unresolved: %#v", result)
			}
		}
		if result.ID == CriterionInconsistencies && (result.MinValue != 1 || result.MaxValue != 1) {
			t.Fatalf("supported independent finding was changed: %#v", result)
		}
	}
}

func TestSemanticCueEvaluationPrefersValidRepair(t *testing.T) {
	modelResults := validSemanticCueModelResults()
	modelResults[CriterionPosesAsAuthority] = semanticCueModelResult{MinValue: 1, MaxValue: 1}
	initial := marshalSemanticCueResponse(t, modelResults)
	server, requests := semanticCueTestServer(t, "poses_as_authority", initial, `{"min_value":1,"max_value":1,"evidence":["Alice Smith is a familiar IT colleague."]}`)
	defer server.Close()

	results, err := NewSemanticCueEvaluator(NewClient(server.URL, "test")).Evaluate(
		context.Background(), EvaluationInput{
			Email:             Email{Text: "Regards,\nAlice Smith\nIT Support\nalice@windropolis.corporate"},
			GenerationContext: GenerationRequest{SenderContext: "Alice Smith is a familiar IT colleague."},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if *requests != 2 {
		t.Fatalf("expected two model calls, got %d", *requests)
	}
	for _, result := range results {
		if result.ID == CriterionPosesAsAuthority {
			if result.MinValue != 1 || result.MaxValue != 1 || result.Source != CueSourceLLM ||
				result.EvaluationRepair != "focused_response" ||
				len(result.Evidence) != 1 || !strings.Contains(result.Evidence[0], "Alice Smith") {
				t.Fatalf("a supported repair must remain positive: %#v", result)
			}
			return
		}
	}
	t.Fatal("authority criterion missing")
}

func TestSemanticCueEvidenceFallbackLeavesOtherValidationStrict(t *testing.T) {
	modelResults := validSemanticCueModelResults()
	modelResults[CriterionPosesAsAuthority] = semanticCueModelResult{MinValue: 1, MaxValue: 1}
	delete(modelResults, CriterionThreats)
	response := marshalSemanticCueResponse(t, modelResults)
	server, requests := semanticCueTestServer(t, "semantic cue results", response, response)
	defer server.Close()

	_, err := NewSemanticCueEvaluator(NewClient(server.URL, "test")).Evaluate(
		context.Background(), EvaluationInput{},
	)
	if err == nil || !strings.Contains(err.Error(), "semantic cue results") {
		t.Fatalf("a missing criterion must still fail after repair, got %v", err)
	}
	if *requests != 2 {
		t.Fatalf("expected two model calls, got %d", *requests)
	}
}

func TestSemanticCueEvidenceFallbackCountedCriterionIsUnknown(t *testing.T) {
	modelResults := validSemanticCueModelResults()
	modelResults[CriterionTimePressure] = semanticCueModelResult{MinValue: 3, MaxValue: 3}
	results, err := parseSemanticCueResponseWithEvidenceFallback(marshalSemanticCueResponse(t, modelResults))
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.ID == CriterionTimePressure {
			if result.MinValue != 0 || result.MaxValue != 15 || result.Source != CueSourceHybrid ||
				result.EvaluationRepair != "unsupported_positive" {
				t.Fatalf("unsupported count must be uncertain up to the classification ceiling: %#v", result)
			}
			return
		}
	}
	t.Fatal("time pressure criterion missing")
}

func TestSemanticCueFocusedRepairDoesNotAcceptChangedCount(t *testing.T) {
	modelResults := validSemanticCueModelResults()
	modelResults[CriterionTimePressure] = semanticCueModelResult{MinValue: 2, MaxValue: 2}
	server, requests := semanticCueTestServer(t, "time_pressure", marshalSemanticCueResponse(t, modelResults),
		`{"min_value":1,"max_value":1,"evidence":["The subject says 'Immediately'."]}`)
	defer server.Close()

	results, err := NewSemanticCueEvaluator(NewClient(server.URL, "test")).Evaluate(
		context.Background(), EvaluationInput{Email: Email{Subject: "Policy review", Text: "Review the policy.\nRegards,\nAlice Smith\nIT Support\nalice@windropolis.corporate"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if *requests != 2 {
		t.Fatalf("expected primary and focused calls, got %d", *requests)
	}
	for _, result := range results {
		if result.ID == CriterionTimePressure {
			if result.MinValue != 0 || result.MaxValue != 15 || result.Source != CueSourceHybrid ||
				result.EvaluationRepair != "unsupported_positive" {
				t.Fatalf("a conflicting focused count must remain unresolved: %#v", result)
			}
			return
		}
	}
	t.Fatal("time pressure criterion missing")
}

func semanticCueTestServer(t *testing.T, repairContains string, responses ...string) (*httptest.Server, *int) {
	t.Helper()
	requests := new(int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode Ollama request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		index := *requests
		*requests = *requests + 1
		if index >= len(responses) {
			t.Errorf("unexpected Ollama call %d", index+1)
			http.Error(w, "unexpected call", http.StatusBadRequest)
			return
		}
		if index > 0 {
			if len(req.Messages) != 2 && len(req.Messages) != 4 {
				t.Errorf("unexpected repair message count: %d", len(req.Messages))
			} else if !strings.Contains(req.Messages[len(req.Messages)-1].Content, repairContains) {
				t.Errorf("repair request did not identify %q", repairContains)
			}
			if len(req.Messages) == 2 {
				format, ok := req.Format.(map[string]interface{})
				if !ok || format["properties"] == nil || format["type"] != "object" {
					t.Errorf("focused repair did not request a single criterion schema: %#v", req.Format)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(chatResponse{
			Message: Message{Content: responses[index]}, Done: true, DoneReason: "stop",
		}); err != nil {
			t.Errorf("encode Ollama response: %v", err)
		}
	}))
	return server, requests
}

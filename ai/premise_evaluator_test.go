package ai

import (
	"encoding/json"
	"testing"
)

func TestParsePremiseAlignmentResponse(t *testing.T) {
	results := []premiseModelResult{
		{ID: PremiseElementWorkplaceProcess, Resolved: true, Score: 6, Explanation: "Matches a common workflow."},
		{ID: PremiseElementWorkplaceRelevance, Resolved: true, Score: 8, Explanation: "Highly relevant to the role."},
		{ID: PremiseElementSituationalAlignment, Resolved: false, Score: 0, Explanation: "Current situation is not specified."},
		{ID: PremiseElementConsequences, Resolved: true, Score: 4, Explanation: "Moderate perceived consequences."},
	}
	data, err := json.Marshal(premiseModelResponse{Results: results})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	parsed, err := parsePremiseAlignmentResponse(string(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed) != 4 {
		t.Fatalf("expected 4 results, got %d", len(parsed))
	}
	if parsed[0].Score == nil || *parsed[0].Score != 6 {
		t.Fatalf("expected resolved score 6, got %#v", parsed[0].Score)
	}
	if parsed[2].Score != nil {
		t.Fatalf("expected unresolved element score to be nil")
	}
}

func TestParsePremiseAlignmentResponseRejectsDuplicate(t *testing.T) {
	results := []premiseModelResult{
		{ID: PremiseElementWorkplaceProcess, Resolved: true, Score: 2, Explanation: "a"},
		{ID: PremiseElementWorkplaceProcess, Resolved: true, Score: 2, Explanation: "b"},
		{ID: PremiseElementSituationalAlignment, Resolved: true, Score: 2, Explanation: "c"},
		{ID: PremiseElementConsequences, Resolved: true, Score: 2, Explanation: "d"},
	}
	data, _ := json.Marshal(premiseModelResponse{Results: results})
	if _, err := parsePremiseAlignmentResponse(string(data)); err == nil {
		t.Fatal("expected duplicate element error")
	}
}

func TestPriorExposureExplanationUnknown(t *testing.T) {
	if priorExposureExplanation(TrainingExposureUnknown) == "" {
		t.Fatal("expected explanation")
	}
}

func TestApplyPremiseContextRulesMakesSituationUnknownWithoutSituationContext(t *testing.T) {
	score := 8
	results := []PremiseAlignmentElementResult{
		{ID: PremiseElementWorkplaceProcess, Score: &score},
		{ID: PremiseElementWorkplaceRelevance, Score: &score},
		{ID: PremiseElementSituationalAlignment, Score: &score, Explanation: "Incorrectly inferred from scenario."},
		{ID: PremiseElementConsequences, Score: &score},
	}

	updated := applyPremiseContextRules(results, EvaluationInput{})
	for _, result := range updated {
		if result.ID != PremiseElementSituationalAlignment {
			continue
		}
		if result.Score != nil {
			t.Fatalf("expected situational alignment to be unresolved, got %#v", result.Score)
		}
		if result.Explanation == "" {
			t.Fatal("expected unresolved explanation")
		}
		return
	}

	t.Fatal("situational alignment element not found")
}

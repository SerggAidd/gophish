package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSemanticCueResponse(t *testing.T) {
	modelResults := validSemanticCueModelResults()
	setSemanticModelResult(t, modelResults, CriterionTimePressure, 2, 2, []string{"Two urgent expressions."})
	setSemanticModelResult(t, modelResults, CriterionMissingBranding, 0, 1, []string{"Organization branding expectations are not fully specified."})

	results, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, modelResults))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byID := make(map[CueCriterionID]CueCriterionResult)
	for _, result := range results {
		byID[result.ID] = result
	}
	if byID[CriterionTimePressure].MinValue != 2 || byID[CriterionTimePressure].MaxValue != 2 {
		t.Fatalf("unexpected time-pressure result: %#v", byID[CriterionTimePressure])
	}
	if byID[CriterionMissingBranding].MinValue != 0 || byID[CriterionMissingBranding].MaxValue != 1 {
		t.Fatalf("unexpected branding range: %#v", byID[CriterionMissingBranding])
	}
}

func TestParseSemanticCueResponseMissingCriterion(t *testing.T) {
	results := validSemanticCueModelResults()
	delete(results, CriterionLimitedTimeOffers)
	_, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err == nil {
		t.Fatal("expected missing-criterion error")
	}
}

func TestParseSemanticCueResponseUnexpectedCriterion(t *testing.T) {
	results := validSemanticCueModelResults()
	delete(results, CriterionLimitedTimeOffers)
	results[CriterionMissingGreeting] = semanticCueModelResult{}
	_, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err == nil {
		t.Fatal("expected unexpected-criterion error")
	}
}

func TestParseSemanticCueResponseInvalidBinaryRange(t *testing.T) {
	results := validSemanticCueModelResults()
	setSemanticModelResult(t, results, CriterionMimicsBusinessProcess, 0, 2, []string{"Invalid range."})
	_, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err == nil {
		t.Fatal("expected binary range validation error")
	}
}

func TestParseSemanticCueResponseExactPositiveWithoutEvidence(t *testing.T) {
	results := validSemanticCueModelResults()
	setSemanticModelResult(t, results, CriterionThreats, 1, 1, nil)
	_, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err == nil {
		t.Fatal("expected evidence error")
	}
}

func TestParseSemanticCueResponseUnresolvedWithoutEvidenceUsesFallback(t *testing.T) {
	results := validSemanticCueModelResults()
	setSemanticModelResult(t, results, CriterionThreats, 0, 1, nil)

	parsed, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, result := range parsed {
		if result.ID != CriterionThreats {
			continue
		}
		if result.MinValue != 0 || result.MaxValue != 1 || len(result.Evidence) == 0 {
			t.Fatalf("unexpected unresolved result: %#v", result)
		}
		return
	}

	t.Fatal("threats criterion not found")
}

func TestSemanticCueCriteriaCoverAllNonDeterministicCriteria(t *testing.T) {
	deterministic := map[CueCriterionID]bool{
		CriterionAttachments:            true,
		CriterionMissingGreeting:        true,
		CriterionMissingPersonalization: true,
		CriterionHiddenURLLinks:         true,
	}
	semantic := make(map[CueCriterionID]bool)
	for _, id := range semanticCueCriterionIDs {
		semantic[id] = true
	}
	for _, definition := range NISTCueCriteria {
		if !deterministic[definition.ID] && !semantic[definition.ID] {
			t.Fatalf("criterion %q is not assigned to a detector", definition.ID)
		}
	}
}

func TestParseSemanticCueResponseMarksDomainSpoofingHybrid(t *testing.T) {
	results := validSemanticCueModelResults()
	setSemanticModelResult(t, results, CriterionSenderDomainSpoofing, 1, 1, []string{"rncrosoft.com plausibly imitates microsoft.com."})
	setSemanticModelResult(t, results, CriterionSpoofedLinkDomains, 1, 1, []string{"login.rncrosoft.com plausibly imitates microsoft.com."})

	parsed, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byID := make(map[CueCriterionID]CueCriterionResult)
	for _, result := range parsed {
		byID[result.ID] = result
	}

	for _, id := range []CueCriterionID{CriterionSenderDomainSpoofing, CriterionSpoofedLinkDomains} {
		if byID[id].Source != CueSourceHybrid {
			t.Fatalf("criterion %q: expected hybrid source, got %q", id, byID[id].Source)
		}
	}
}

func TestFormatDomainComparisonContext(t *testing.T) {
	context := EvaluationContext{
		SimulatedSender: &SenderIdentity{Email: "support@rncrosoft.com"},
		ExpectedSender:  &SenderIdentity{Email: "support@microsoft.com"},
		Link: LinkContext{
			Usage:          LinkUsageUsed,
			SimulatedURL:   "https://login.rncrosoft.com/path",
			ExpectedDomain: "microsoft.com",
		},
	}

	value := formatDomainComparisonContext(context)
	for _, expected := range []string{"rncrosoft.com", "microsoft.com", "different"} {
		if !strings.Contains(value, expected) {
			t.Fatalf("expected %q in domain comparison context: %s", expected, value)
		}
	}
}

func validSemanticCueModelResults() map[CueCriterionID]semanticCueModelResult {
	results := make(map[CueCriterionID]semanticCueModelResult, len(semanticCueCriterionIDs))
	for _, id := range semanticCueCriterionIDs {
		results[id] = semanticCueModelResult{MinValue: 0, MaxValue: 0, Evidence: []string{}}
	}
	return results
}

func setSemanticModelResult(t *testing.T, results map[CueCriterionID]semanticCueModelResult, id CueCriterionID, minValue, maxValue int, evidence []string) {
	t.Helper()
	if _, ok := results[id]; !ok {
		t.Fatalf("criterion %q not found", id)
	}
	results[id] = semanticCueModelResult{MinValue: minValue, MaxValue: maxValue, Evidence: evidence}
}

func marshalSemanticCueResponse(t *testing.T, results map[CueCriterionID]semanticCueModelResult) string {
	t.Helper()
	data, err := json.Marshal(semanticCueModelResponse{Results: results})
	if err != nil {
		t.Fatalf("marshal semantic response: %v", err)
	}
	return string(data)
}

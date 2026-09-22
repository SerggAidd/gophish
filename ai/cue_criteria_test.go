package ai

import "testing"

func TestNISTCueCriteria(t *testing.T) {
	if len(NISTCueCriteria) != 27 {
		t.Fatalf(
			"expected 27 NIST cue criteria, got %d",
			len(NISTCueCriteria),
		)
	}

	knownCues := make(map[CueID]struct{})
	for _, definition := range NISTCueDefinitions {
		knownCues[definition.ID] = struct{}{}
	}

	seenCriteria := make(map[CueCriterionID]struct{})

	for _, criterion := range NISTCueCriteria {
		if criterion.ID == "" {
			t.Fatal("cue criterion has empty ID")
		}

		if criterion.Name == "" {
			t.Fatalf(
				"criterion %q has empty name",
				criterion.ID,
			)
		}

		if _, exists := knownCues[criterion.CueID]; !exists {
			t.Fatalf(
				"criterion %q references unknown cue %q",
				criterion.ID,
				criterion.CueID,
			)
		}

		switch criterion.Kind {
		case CueCriterionBinary, CueCriterionCounted:
		default:
			t.Fatalf(
				"criterion %q has invalid kind %q",
				criterion.ID,
				criterion.Kind,
			)
		}

		if _, exists := seenCriteria[criterion.ID]; exists {
			t.Fatalf(
				"duplicate cue criterion ID: %q",
				criterion.ID,
			)
		}

		seenCriteria[criterion.ID] = struct{}{}
	}
}

func TestBuildCueResultsAggregatesCriteria(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:       CriterionSpellingErrors,
			Value:    3,
			Source:   CueSourceLLM,
			Evidence: []string{"error one", "error two", "error three"},
		},
		{
			ID:       CriterionGrammarErrors,
			Value:    2,
			Source:   CueSourceLLM,
			Evidence: []string{"grammar one", "grammar two"},
		},
		{
			ID:       CriterionSenderDomainSpoofing,
			Value:    1,
			Source:   CueSourceLLM,
			Evidence: []string{"sender domain"},
		},
		{
			ID:       CriterionSpoofedLinkDomains,
			Value:    2,
			Source:   CueSourceDeterministic,
			Evidence: []string{"link one", "link two"},
		},
	}

	cueResults, err := BuildCueResults(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cueResults) != 2 {
		t.Fatalf(
			"expected 2 aggregated cue results, got %d",
			len(cueResults),
		)
	}

	var spellingGrammar *CueResult
	var domainSpoofing *CueResult

	for i := range cueResults {
		switch cueResults[i].ID {
		case CueSpellingGrammar:
			spellingGrammar = &cueResults[i]
		case CueDomainSpoofing:
			domainSpoofing = &cueResults[i]
		}
	}

	if spellingGrammar == nil {
		t.Fatal("missing spelling and grammar cue result")
	}

	if spellingGrammar.Count != 5 {
		t.Fatalf(
			"expected spelling and grammar count 5, got %d",
			spellingGrammar.Count,
		)
	}

	if spellingGrammar.Source != CueSourceLLM {
		t.Fatalf(
			"expected spelling and grammar source %q, got %q",
			CueSourceLLM,
			spellingGrammar.Source,
		)
	}

	if domainSpoofing == nil {
		t.Fatal("missing domain spoofing cue result")
	}

	if domainSpoofing.Count != 3 {
		t.Fatalf(
			"expected domain spoofing count 3, got %d",
			domainSpoofing.Count,
		)
	}

	if domainSpoofing.Source != CueSourceHybrid {
		t.Fatalf(
			"expected domain spoofing source %q, got %q",
			CueSourceHybrid,
			domainSpoofing.Source,
		)
	}
}

func TestBuildCueResultsIgnoresZeroValues(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:     CriterionThreats,
			Value:  0,
			Source: CueSourceLLM,
		},
		{
			ID:     CriterionHiddenURLLinks,
			Value:  1,
			Source: CueSourceDeterministic,
		},
	}

	cueResults, err := BuildCueResults(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cueResults) != 1 {
		t.Fatalf(
			"expected 1 cue result, got %d",
			len(cueResults),
		)
	}

	if cueResults[0].ID != CueURLHyperlinking {
		t.Fatalf(
			"expected cue %q, got %q",
			CueURLHyperlinking,
			cueResults[0].ID,
		)
	}
}

func TestBuildCueResultsRejectsInvalidBinaryValue(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:     CriterionMissingGreeting,
			Value:  2,
			Source: CueSourceLLM,
		},
	}

	_, err := BuildCueResults(results)
	if err == nil {
		t.Fatal("expected error for invalid binary value")
	}
}

func TestBuildCueResultsRejectsNegativeValue(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:     CriterionTimePressure,
			Value:  -1,
			Source: CueSourceLLM,
		},
	}

	_, err := BuildCueResults(results)
	if err == nil {
		t.Fatal("expected error for negative criterion value")
	}
}

func TestBuildCueResultsRejectsUnknownCriterion(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:     CueCriterionID("unknown"),
			Value:  1,
			Source: CueSourceLLM,
		},
	}

	_, err := BuildCueResults(results)
	if err == nil {
		t.Fatal("expected error for unknown criterion")
	}
}

func TestBuildCueResultsRejectsDuplicateCriterion(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:     CriterionThreats,
			Value:  1,
			Source: CueSourceLLM,
		},
		{
			ID:     CriterionThreats,
			Value:  1,
			Source: CueSourceLLM,
		},
	}

	_, err := BuildCueResults(results)
	if err == nil {
		t.Fatal("expected error for duplicate criterion")
	}
}

func TestBuildCueResultsRejectsInvalidSource(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:     CriterionThreats,
			Value:  1,
			Source: CueDetectionSource("unknown"),
		},
	}

	_, err := BuildCueResults(results)
	if err == nil {
		t.Fatal("expected error for invalid detection source")
	}
}

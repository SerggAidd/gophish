package ai

import "testing"

func TestNISTCueCriteria(t *testing.T) {
	if len(NISTCueCriteria) != 27 {
		t.Fatalf("expected 27 NIST cue criteria, got %d", len(NISTCueCriteria))
	}

	knownCues := make(map[CueID]struct{})
	for _, definition := range NISTCueDefinitions {
		knownCues[definition.ID] = struct{}{}
	}

	seenCriteria := make(map[CueCriterionID]struct{})
	for _, criterion := range NISTCueCriteria {
		if criterion.ID == "" || criterion.Name == "" {
			t.Fatalf("invalid cue criterion definition: %#v", criterion)
		}
		if _, exists := knownCues[criterion.CueID]; !exists {
			t.Fatalf("criterion %q references unknown cue %q", criterion.ID, criterion.CueID)
		}
		switch criterion.Kind {
		case CueCriterionBinary, CueCriterionCounted:
		default:
			t.Fatalf("criterion %q has invalid kind %q", criterion.ID, criterion.Kind)
		}
		if _, exists := seenCriteria[criterion.ID]; exists {
			t.Fatalf("duplicate cue criterion ID: %q", criterion.ID)
		}
		seenCriteria[criterion.ID] = struct{}{}
	}
}

func TestBuildCueResultsAggregatesRanges(t *testing.T) {
	results := []CueCriterionResult{
		{ID: CriterionSpellingErrors, MinValue: 3, MaxValue: 3, Source: CueSourceLLM},
		{ID: CriterionGrammarErrors, MinValue: 1, MaxValue: 2, Source: CueSourceLLM},
		{ID: CriterionSenderDomainSpoofing, MinValue: 1, MaxValue: 1, Source: CueSourceDeterministic},
		{ID: CriterionSpoofedLinkDomains, MinValue: 0, MaxValue: 1, Source: CueSourceDeterministic},
	}

	cueResults, err := BuildCueResults(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byID := make(map[CueID]CueResult)
	for _, result := range cueResults {
		byID[result.ID] = result
	}

	spelling := byID[CueSpellingGrammar]
	if spelling.MinCount != 4 || spelling.MaxCount != 5 {
		t.Fatalf("expected spelling/grammar range 4-5, got %d-%d", spelling.MinCount, spelling.MaxCount)
	}

	domain := byID[CueDomainSpoofing]
	if domain.MinCount != 1 || domain.MaxCount != 2 {
		t.Fatalf("expected domain spoofing range 1-2, got %d-%d", domain.MinCount, domain.MaxCount)
	}
}

func TestBuildCueResultsIgnoresExactZero(t *testing.T) {
	results := []CueCriterionResult{
		{ID: CriterionThreats, MinValue: 0, MaxValue: 0, Source: CueSourceLLM},
		{ID: CriterionHiddenURLLinks, MinValue: 1, MaxValue: 1, Source: CueSourceDeterministic},
	}

	cueResults, err := BuildCueResults(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cueResults) != 1 || cueResults[0].ID != CueURLHyperlinking {
		t.Fatalf("unexpected cue results: %#v", cueResults)
	}
}

func TestBuildCueResultsKeepsUnresolvedZeroToOne(t *testing.T) {
	results := []CueCriterionResult{
		{ID: CriterionSenderDomainSpoofing, MinValue: 0, MaxValue: 1, Source: CueSourceDeterministic},
	}

	cueResults, err := BuildCueResults(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cueResults) != 1 || cueResults[0].MinCount != 0 || cueResults[0].MaxCount != 1 {
		t.Fatalf("expected unresolved cue range 0-1, got %#v", cueResults)
	}
}

func TestBuildCueResultsRejectsInvalidBinaryRange(t *testing.T) {
	_, err := BuildCueResults([]CueCriterionResult{{
		ID: CriterionMissingGreeting, MinValue: 0, MaxValue: 2, Source: CueSourceLLM,
	}})
	if err == nil {
		t.Fatal("expected error for invalid binary range")
	}
}

func TestBuildCueResultsRejectsNegativeRange(t *testing.T) {
	_, err := BuildCueResults([]CueCriterionResult{{
		ID: CriterionTimePressure, MinValue: -1, MaxValue: 1, Source: CueSourceLLM,
	}})
	if err == nil {
		t.Fatal("expected error for negative range")
	}
}

func TestBuildCueResultsRejectsInvertedRange(t *testing.T) {
	_, err := BuildCueResults([]CueCriterionResult{{
		ID: CriterionTimePressure, MinValue: 2, MaxValue: 1, Source: CueSourceLLM,
	}})
	if err == nil {
		t.Fatal("expected error for inverted range")
	}
}

func TestBuildCueResultsRejectsUnknownCriterion(t *testing.T) {
	_, err := BuildCueResults([]CueCriterionResult{{
		ID: CueCriterionID("unknown"), MinValue: 1, MaxValue: 1, Source: CueSourceLLM,
	}})
	if err == nil {
		t.Fatal("expected error for unknown criterion")
	}
}

func TestBuildCueResultsRejectsDuplicateCriterion(t *testing.T) {
	_, err := BuildCueResults([]CueCriterionResult{
		{ID: CriterionThreats, MinValue: 1, MaxValue: 1, Source: CueSourceLLM},
		{ID: CriterionThreats, MinValue: 1, MaxValue: 1, Source: CueSourceLLM},
	})
	if err == nil {
		t.Fatal("expected error for duplicate criterion")
	}
}

func TestBuildCueResultsRejectsInvalidSource(t *testing.T) {
	_, err := BuildCueResults([]CueCriterionResult{{
		ID: CriterionThreats, MinValue: 1, MaxValue: 1, Source: CueDetectionSource("unknown"),
	}})
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
}

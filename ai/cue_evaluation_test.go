package ai

import "testing"

func TestBuildCueEvaluationExactCategories(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		expected CueCategory
	}{
		{"few", 4, CueCategoryFew},
		{"some", 10, CueCategorySome},
		{"many", 15, CueCategoryMany},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluation, err := BuildCueEvaluation([]CueResult{{
				ID: CueID("test"), MinCount: tt.count, MaxCount: tt.count, Source: CueSourceDeterministic,
			}})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !evaluation.CategoryResolved || evaluation.Category != tt.expected {
				t.Fatalf("expected resolved category %q, got %#v", tt.expected, evaluation)
			}
		})
	}
}

func TestBuildCueEvaluationRangeCrossesBoundary(t *testing.T) {
	evaluation, err := BuildCueEvaluation([]CueResult{{
		ID: CueID("test"), MinCount: 7, MaxCount: 9, Source: CueSourceHybrid,
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluation.CategoryResolved {
		t.Fatal("expected unresolved category")
	}
	if evaluation.MinCategory != CueCategoryFew || evaluation.MaxCategory != CueCategorySome {
		t.Fatalf("expected Few..Some, got %q..%q", evaluation.MinCategory, evaluation.MaxCategory)
	}
	if len(evaluation.PossibleCategories) != 2 {
		t.Fatalf("expected 2 possible categories, got %d", len(evaluation.PossibleCategories))
	}
}

func TestBuildCueEvaluationZeroIsRepresentedButUnclassified(t *testing.T) {
	evaluation, err := BuildCueEvaluation(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !evaluation.ZeroCountPossible || evaluation.CategoryResolved {
		t.Fatalf("unexpected zero-cue evaluation: %#v", evaluation)
	}
	if len(evaluation.PossibleCategories) != 0 {
		t.Fatalf("expected no NIST category for exact zero cues")
	}
}

func TestBuildCueEvaluationRangeFromZeroToFew(t *testing.T) {
	evaluation, err := BuildCueEvaluation([]CueResult{{
		ID: CueID("test"), MinCount: 0, MaxCount: 1, Source: CueSourceDeterministic,
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !evaluation.ZeroCountPossible || evaluation.CategoryResolved {
		t.Fatalf("expected unresolved zero-or-few evaluation: %#v", evaluation)
	}
	if len(evaluation.PossibleCategories) != 1 || evaluation.PossibleCategories[0] != CueCategoryFew {
		t.Fatalf("expected Few as positive-count category, got %#v", evaluation.PossibleCategories)
	}
}

func TestBuildCueEvaluationRejectsInvalidRange(t *testing.T) {
	_, err := BuildCueEvaluation([]CueResult{{
		ID: CueID("test"), MinCount: 3, MaxCount: 2, Source: CueSourceLLM,
	}})
	if err == nil {
		t.Fatal("expected error for invalid range")
	}
}

func TestBuildCueEvaluationRejectsDuplicateID(t *testing.T) {
	_, err := BuildCueEvaluation([]CueResult{
		{ID: CueID("test"), MinCount: 1, MaxCount: 1, Source: CueSourceDeterministic},
		{ID: CueID("test"), MinCount: 1, MaxCount: 1, Source: CueSourceLLM},
	})
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

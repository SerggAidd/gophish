package ai

import "testing"

func TestBuildCueEvaluationFew(t *testing.T) {
	results := []CueResult{
		{
			ID:     CueID("test_cue_one"),
			Count:  1,
			Source: CueSourceDeterministic,
		},
		{
			ID:     CueID("test_cue_two"),
			Count:  3,
			Source: CueSourceLLM,
		},
	}

	evaluation, err := BuildCueEvaluation(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evaluation.TotalCount != 4 {
		t.Fatalf("expected total count 4, got %d", evaluation.TotalCount)
	}

	if evaluation.Category != CueCategoryFew {
		t.Fatalf(
			"expected category %q, got %q",
			CueCategoryFew,
			evaluation.Category,
		)
	}
}

func TestBuildCueEvaluationSome(t *testing.T) {
	results := []CueResult{
		{
			ID:     CueID("test_cue_one"),
			Count:  4,
			Source: CueSourceDeterministic,
		},
		{
			ID:     CueID("test_cue_two"),
			Count:  6,
			Source: CueSourceLLM,
		},
	}

	evaluation, err := BuildCueEvaluation(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evaluation.TotalCount != 10 {
		t.Fatalf("expected total count 10, got %d", evaluation.TotalCount)
	}

	if evaluation.Category != CueCategorySome {
		t.Fatalf(
			"expected category %q, got %q",
			CueCategorySome,
			evaluation.Category,
		)
	}
}

func TestBuildCueEvaluationMany(t *testing.T) {
	results := []CueResult{
		{
			ID:     CueID("test_cue_one"),
			Count:  8,
			Source: CueSourceLLM,
		},
		{
			ID:     CueID("test_cue_two"),
			Count:  7,
			Source: CueSourceDeterministic,
		},
	}

	evaluation, err := BuildCueEvaluation(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evaluation.TotalCount != 15 {
		t.Fatalf("expected total count 15, got %d", evaluation.TotalCount)
	}

	if evaluation.Category != CueCategoryMany {
		t.Fatalf(
			"expected category %q, got %q",
			CueCategoryMany,
			evaluation.Category,
		)
	}
}

func TestBuildCueEvaluationEmpty(t *testing.T) {
	_, err := BuildCueEvaluation(nil)
	if err == nil {
		t.Fatal("expected error for empty cue evaluation")
	}
}

func TestBuildCueEvaluationNegativeCount(t *testing.T) {
	results := []CueResult{
		{
			ID:     CueID("test_cue"),
			Count:  -1,
			Source: CueSourceLLM,
		},
	}

	_, err := BuildCueEvaluation(results)
	if err == nil {
		t.Fatal("expected error for negative cue count")
	}
}

func TestBuildCueEvaluationEmptyID(t *testing.T) {
	results := []CueResult{
		{
			Count:  1,
			Source: CueSourceDeterministic,
		},
	}

	_, err := BuildCueEvaluation(results)
	if err == nil {
		t.Fatal("expected error for empty cue ID")
	}
}

func TestBuildCueEvaluationDuplicateID(t *testing.T) {
	results := []CueResult{
		{
			ID:     CueID("test_cue"),
			Count:  1,
			Source: CueSourceDeterministic,
		},
		{
			ID:     CueID("test_cue"),
			Count:  1,
			Source: CueSourceLLM,
		},
	}

	_, err := BuildCueEvaluation(results)
	if err == nil {
		t.Fatal("expected error for duplicate cue ID")
	}
}

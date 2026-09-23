package ai

import "testing"

func TestBuildDifficultyEvaluationResolved(t *testing.T) {
	cues := CueEvaluation{
		MinCount: 4, MaxCount: 4,
		Category: CueCategoryFew, MinCategory: CueCategoryFew, MaxCategory: CueCategoryFew,
		PossibleCategories: []CueCategory{CueCategoryFew}, CategoryResolved: true,
	}
	premise := PremiseAlignmentEvaluation{
		MinScore: 20, MaxScore: 20,
		Category: PremiseAlignmentStrong, MinCategory: PremiseAlignmentStrong, MaxCategory: PremiseAlignmentStrong,
		CategoryResolved: true,
	}

	evaluation, err := BuildDifficultyEvaluation(cues, premise)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !evaluation.Resolved || evaluation.DetectionDifficulty != DifficultyVeryDifficult {
		t.Fatalf("unexpected evaluation: %#v", evaluation)
	}
}

func TestBuildDifficultyEvaluationCombinesCueAndPremiseRanges(t *testing.T) {
	cues := CueEvaluation{MinCount: 7, MaxCount: 10}
	premise := PremiseAlignmentEvaluation{MinScore: 12, MaxScore: 20}

	evaluation, err := BuildDifficultyEvaluation(cues, premise)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluation.Resolved {
		t.Fatal("expected unresolved difficulty")
	}
	if len(evaluation.PossibleDifficulties) < 2 {
		t.Fatalf("expected multiple possible difficulties, got %#v", evaluation.PossibleDifficulties)
	}
}

func TestBuildDifficultyEvaluationPremiseRangeCanStillResolve(t *testing.T) {
	cues := CueEvaluation{MinCount: 4, MaxCount: 4}
	premise := PremiseAlignmentEvaluation{MinScore: 12, MaxScore: 20}

	evaluation, err := BuildDifficultyEvaluation(cues, premise)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !evaluation.Resolved || evaluation.DetectionDifficulty != DifficultyVeryDifficult {
		t.Fatalf("expected resolved very difficult, got %#v", evaluation)
	}
}

func TestBuildDifficultyEvaluationZeroCuePossibilityPreventsResolution(t *testing.T) {
	cues := CueEvaluation{MinCount: 0, MaxCount: 4, ZeroCountPossible: true}
	premise := PremiseAlignmentEvaluation{MinScore: 20, MaxScore: 20}

	evaluation, err := BuildDifficultyEvaluation(cues, premise)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluation.Resolved {
		t.Fatal("zero-cue possibility must prevent exact NIST resolution")
	}
	if !evaluation.UnclassifiedCueCountPossible {
		t.Fatal("expected unclassified cue-count flag")
	}
}

func TestBuildDifficultyEvaluationExactZeroCueCount(t *testing.T) {
	cues := CueEvaluation{MinCount: 0, MaxCount: 0, ZeroCountPossible: true}
	premise := PremiseAlignmentEvaluation{MinScore: 20, MaxScore: 20}

	evaluation, err := BuildDifficultyEvaluation(cues, premise)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evaluation.Resolved || len(evaluation.PossibleDifficulties) != 0 {
		t.Fatalf("expected unclassified difficulty, got %#v", evaluation)
	}
}

package ai

import "testing"

func TestBuildDifficultyEvaluationResolved(t *testing.T) {
	cues := CueEvaluation{
		TotalCount: 4,
		Category:   CueCategoryFew,
	}

	premise := PremiseAlignmentEvaluation{
		MinScore:         20,
		MaxScore:         20,
		Category:         PremiseAlignmentStrong,
		MinCategory:      PremiseAlignmentStrong,
		MaxCategory:      PremiseAlignmentStrong,
		CategoryResolved: true,
	}

	evaluation, err := BuildDifficultyEvaluation(cues, premise)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !evaluation.Resolved {
		t.Fatal("expected difficulty to be resolved")
	}

	if evaluation.DetectionDifficulty != DifficultyVeryDifficult {
		t.Fatalf(
			"expected difficulty %q, got %q",
			DifficultyVeryDifficult,
			evaluation.DetectionDifficulty,
		)
	}

	if len(evaluation.PossibleDifficulties) != 1 {
		t.Fatalf(
			"expected 1 possible difficulty, got %d",
			len(evaluation.PossibleDifficulties),
		)
	}
}

func TestBuildDifficultyEvaluationUnresolved(t *testing.T) {
	cues := CueEvaluation{
		TotalCount: 10,
		Category:   CueCategorySome,
	}

	premise := PremiseAlignmentEvaluation{
		MinScore:         12,
		MaxScore:         20,
		MinCategory:      PremiseAlignmentMedium,
		MaxCategory:      PremiseAlignmentStrong,
		CategoryResolved: false,
	}

	evaluation, err := BuildDifficultyEvaluation(cues, premise)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evaluation.Resolved {
		t.Fatal("expected difficulty to be unresolved")
	}

	if evaluation.DetectionDifficulty != "" {
		t.Fatalf(
			"expected no exact difficulty, got %q",
			evaluation.DetectionDifficulty,
		)
	}

	if len(evaluation.PossibleDifficulties) != 2 {
		t.Fatalf(
			"expected 2 possible difficulties, got %d",
			len(evaluation.PossibleDifficulties),
		)
	}

	expected := map[DetectionDifficulty]bool{
		DifficultyModeratelyDifficult: true,
		DifficultyVeryDifficult:       true,
	}

	for _, difficulty := range evaluation.PossibleDifficulties {
		if !expected[difficulty] {
			t.Fatalf(
				"unexpected possible difficulty: %q",
				difficulty,
			)
		}
	}
}

func TestBuildDifficultyEvaluationPremiseRangeStillResolved(t *testing.T) {
	cues := CueEvaluation{
		TotalCount: 4,
		Category:   CueCategoryFew,
	}

	premise := PremiseAlignmentEvaluation{
		MinScore:         12,
		MaxScore:         20,
		MinCategory:      PremiseAlignmentMedium,
		MaxCategory:      PremiseAlignmentStrong,
		CategoryResolved: false,
	}

	evaluation, err := BuildDifficultyEvaluation(cues, premise)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !evaluation.Resolved {
		t.Fatal("expected difficulty to remain resolved")
	}

	if evaluation.DetectionDifficulty != DifficultyVeryDifficult {
		t.Fatalf(
			"expected difficulty %q, got %q",
			DifficultyVeryDifficult,
			evaluation.DetectionDifficulty,
		)
	}
}

func TestBuildDifficultyEvaluationInvalidCueCategory(t *testing.T) {
	cues := CueEvaluation{
		Category: CueCategory("invalid"),
	}

	premise := PremiseAlignmentEvaluation{
		MinScore: 20,
		MaxScore: 20,
	}

	_, err := BuildDifficultyEvaluation(cues, premise)
	if err == nil {
		t.Fatal("expected error for invalid cue category")
	}
}

func TestPossiblePremiseAlignmentCategoriesInvalidRange(t *testing.T) {
	evaluation := PremiseAlignmentEvaluation{
		MinScore: 20,
		MaxScore: 12,
	}

	_, err := possiblePremiseAlignmentCategories(evaluation)
	if err == nil {
		t.Fatal("expected error for invalid score range")
	}
}

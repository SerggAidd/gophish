package ai

import "testing"

func TestBuildPremiseAlignmentEvaluationExact(t *testing.T) {
	results := []PremiseAlignmentElementResult{
		{
			ID:    PremiseElementWorkplaceProcess,
			Score: intPointer(8),
		},
		{
			ID:    PremiseElementWorkplaceRelevance,
			Score: intPointer(8),
		},
		{
			ID:    PremiseElementSituationalAlignment,
			Score: intPointer(6),
		},
		{
			ID:    PremiseElementConsequences,
			Score: intPointer(6),
		},
		{
			ID:    PremiseElementPriorExposure,
			Score: intPointer(8),
		},
	}

	evaluation, err := BuildPremiseAlignmentEvaluation(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evaluation.MinScore != 20 || evaluation.MaxScore != 20 {
		t.Fatalf(
			"expected exact score 20, got range %d-%d",
			evaluation.MinScore,
			evaluation.MaxScore,
		)
	}

	if !evaluation.CategoryResolved {
		t.Fatal("expected premise alignment category to be resolved")
	}

	if evaluation.Category != PremiseAlignmentStrong {
		t.Fatalf(
			"expected category %q, got %q",
			PremiseAlignmentStrong,
			evaluation.Category,
		)
	}
}

func TestBuildPremiseAlignmentEvaluationUnknownExposureResolved(t *testing.T) {
	results := []PremiseAlignmentElementResult{
		{
			ID:    PremiseElementWorkplaceProcess,
			Score: intPointer(8),
		},
		{
			ID:    PremiseElementWorkplaceRelevance,
			Score: intPointer(8),
		},
		{
			ID:    PremiseElementSituationalAlignment,
			Score: intPointer(6),
		},
		{
			ID:    PremiseElementConsequences,
			Score: intPointer(6),
		},
		{
			ID:    PremiseElementPriorExposure,
			Score: nil,
		},
	}

	evaluation, err := BuildPremiseAlignmentEvaluation(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evaluation.MinScore != 20 || evaluation.MaxScore != 28 {
		t.Fatalf(
			"expected score range 20-28, got %d-%d",
			evaluation.MinScore,
			evaluation.MaxScore,
		)
	}

	if !evaluation.CategoryResolved {
		t.Fatal("expected category to remain resolved")
	}

	if evaluation.Category != PremiseAlignmentStrong {
		t.Fatalf(
			"expected category %q, got %q",
			PremiseAlignmentStrong,
			evaluation.Category,
		)
	}
}

func TestBuildPremiseAlignmentEvaluationUnknownExposureUnresolved(t *testing.T) {
	results := []PremiseAlignmentElementResult{
		{
			ID:    PremiseElementWorkplaceProcess,
			Score: intPointer(8),
		},
		{
			ID:    PremiseElementWorkplaceRelevance,
			Score: intPointer(6),
		},
		{
			ID:    PremiseElementSituationalAlignment,
			Score: intPointer(4),
		},
		{
			ID:    PremiseElementConsequences,
			Score: intPointer(2),
		},
		{
			ID:    PremiseElementPriorExposure,
			Score: nil,
		},
	}

	evaluation, err := BuildPremiseAlignmentEvaluation(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if evaluation.MinScore != 12 || evaluation.MaxScore != 20 {
		t.Fatalf(
			"expected score range 12-20, got %d-%d",
			evaluation.MinScore,
			evaluation.MaxScore,
		)
	}

	if evaluation.CategoryResolved {
		t.Fatal("expected category to be unresolved")
	}

	if evaluation.MinCategory != PremiseAlignmentMedium {
		t.Fatalf(
			"expected minimum category %q, got %q",
			PremiseAlignmentMedium,
			evaluation.MinCategory,
		)
	}

	if evaluation.MaxCategory != PremiseAlignmentStrong {
		t.Fatalf(
			"expected maximum category %q, got %q",
			PremiseAlignmentStrong,
			evaluation.MaxCategory,
		)
	}

	if evaluation.Category != "" {
		t.Fatalf(
			"expected no exact category, got %q",
			evaluation.Category,
		)
	}
}

func TestTrainingExposureScore(t *testing.T) {
	tests := []struct {
		name     string
		exposure TrainingExposure
		expected *int
	}{
		{
			name:     "unknown",
			exposure: TrainingExposureUnknown,
			expected: nil,
		},
		{
			name:     "none",
			exposure: TrainingExposureNone,
			expected: intPointer(0),
		},
		{
			name:     "low",
			exposure: TrainingExposureLow,
			expected: intPointer(2),
		},
		{
			name:     "moderate",
			exposure: TrainingExposureModerate,
			expected: intPointer(4),
		},
		{
			name:     "significant",
			exposure: TrainingExposureSignificant,
			expected: intPointer(6),
		},
		{
			name:     "extreme",
			exposure: TrainingExposureExtreme,
			expected: intPointer(8),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := TrainingExposureScore(tt.exposure)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expected == nil {
				if score != nil {
					t.Fatalf("expected unknown score, got %d", *score)
				}
				return
			}

			if score == nil {
				t.Fatal("expected score, got nil")
			}

			if *score != *tt.expected {
				t.Fatalf(
					"expected score %d, got %d",
					*tt.expected,
					*score,
				)
			}
		})
	}
}

func TestTrainingExposureScoreInvalid(t *testing.T) {
	_, err := TrainingExposureScore(TrainingExposure("invalid"))
	if err == nil {
		t.Fatal("expected error for invalid training exposure")
	}
}

func TestPremiseAlignmentCategoryFromScore(t *testing.T) {
	tests := []struct {
		score    int
		expected PremiseAlignmentCategory
	}{
		{-8, PremiseAlignmentWeak},
		{10, PremiseAlignmentWeak},
		{11, PremiseAlignmentMedium},
		{17, PremiseAlignmentMedium},
		{18, PremiseAlignmentStrong},
		{32, PremiseAlignmentStrong},
	}

	for _, tt := range tests {
		category, err := PremiseAlignmentCategoryFromScore(tt.score)
		if err != nil {
			t.Fatalf("unexpected error for score %d: %v", tt.score, err)
		}

		if category != tt.expected {
			t.Fatalf(
				"score %d: expected %q, got %q",
				tt.score,
				tt.expected,
				category,
			)
		}
	}
}

func TestBuildPremiseAlignmentEvaluationInvalidScore(t *testing.T) {
	results := validPremiseAlignmentResults()
	results[0].Score = intPointer(5)

	_, err := BuildPremiseAlignmentEvaluation(results)
	if err == nil {
		t.Fatal("expected error for invalid element score")
	}
}

func TestBuildPremiseAlignmentEvaluationMissingElement(t *testing.T) {
	results := validPremiseAlignmentResults()
	results = results[:4]

	_, err := BuildPremiseAlignmentEvaluation(results)
	if err == nil {
		t.Fatal("expected error for missing premise alignment element")
	}
}

func TestBuildPremiseAlignmentEvaluationDuplicateElement(t *testing.T) {
	results := validPremiseAlignmentResults()
	results[4].ID = PremiseElementWorkplaceProcess

	_, err := BuildPremiseAlignmentEvaluation(results)
	if err == nil {
		t.Fatal("expected error for duplicate premise alignment element")
	}
}

func TestBuildPremiseAlignmentEvaluationUnknownElement(t *testing.T) {
	results := validPremiseAlignmentResults()
	results[4].ID = PremiseAlignmentElementID("unknown")

	_, err := BuildPremiseAlignmentEvaluation(results)
	if err == nil {
		t.Fatal("expected error for unknown premise alignment element")
	}
}

func TestPremiseAlignmentCategoryFromScoreOutOfRange(t *testing.T) {
	for _, score := range []int{-9, 33} {
		_, err := PremiseAlignmentCategoryFromScore(score)
		if err == nil {
			t.Fatalf(
				"expected error for premise alignment score %d",
				score,
			)
		}
	}
}

func validPremiseAlignmentResults() []PremiseAlignmentElementResult {
	return []PremiseAlignmentElementResult{
		{
			ID:    PremiseElementWorkplaceProcess,
			Score: intPointer(4),
		},
		{
			ID:    PremiseElementWorkplaceRelevance,
			Score: intPointer(4),
		},
		{
			ID:    PremiseElementSituationalAlignment,
			Score: intPointer(4),
		},
		{
			ID:    PremiseElementConsequences,
			Score: intPointer(4),
		},
		{
			ID:    PremiseElementPriorExposure,
			Score: intPointer(4),
		},
	}
}

func intPointer(value int) *int {
	return &value
}

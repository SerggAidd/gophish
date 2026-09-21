package ai

import "testing"

func TestCalculateDetectionDifficulty(t *testing.T) {
	tests := []struct {
		name            string
		cueCategory     CueCategory
		premiseCategory PremiseAlignmentCategory
		expected        DetectionDifficulty
	}{
		{
			name:            "few cues and strong premise",
			cueCategory:     CueCategoryFew,
			premiseCategory: PremiseAlignmentStrong,
			expected:        DifficultyVeryDifficult,
		},
		{
			name:            "few cues and medium premise",
			cueCategory:     CueCategoryFew,
			premiseCategory: PremiseAlignmentMedium,
			expected:        DifficultyVeryDifficult,
		},
		{
			name:            "few cues and weak premise",
			cueCategory:     CueCategoryFew,
			premiseCategory: PremiseAlignmentWeak,
			expected:        DifficultyModeratelyDifficult,
		},
		{
			name:            "some cues and strong premise",
			cueCategory:     CueCategorySome,
			premiseCategory: PremiseAlignmentStrong,
			expected:        DifficultyVeryDifficult,
		},
		{
			name:            "some cues and medium premise",
			cueCategory:     CueCategorySome,
			premiseCategory: PremiseAlignmentMedium,
			expected:        DifficultyModeratelyDifficult,
		},
		{
			name:            "some cues and weak premise",
			cueCategory:     CueCategorySome,
			premiseCategory: PremiseAlignmentWeak,
			expected:        DifficultyModeratelyToLeastDifficult,
		},
		{
			name:            "many cues and strong premise",
			cueCategory:     CueCategoryMany,
			premiseCategory: PremiseAlignmentStrong,
			expected:        DifficultyModeratelyDifficult,
		},
		{
			name:            "many cues and medium premise",
			cueCategory:     CueCategoryMany,
			premiseCategory: PremiseAlignmentMedium,
			expected:        DifficultyModeratelyDifficult,
		},
		{
			name:            "many cues and weak premise",
			cueCategory:     CueCategoryMany,
			premiseCategory: PremiseAlignmentWeak,
			expected:        DifficultyLeastDifficult,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateDetectionDifficulty(
				tt.cueCategory,
				tt.premiseCategory,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Fatalf(
					"expected %q, got %q",
					tt.expected,
					result,
				)
			}
		})
	}
}

func TestCalculateDetectionDifficultyInvalidCategories(t *testing.T) {
	_, err := CalculateDetectionDifficulty(
		CueCategory("invalid"),
		PremiseAlignmentStrong,
	)
	if err == nil {
		t.Fatal("expected error for invalid cue category")
	}

	_, err = CalculateDetectionDifficulty(
		CueCategoryFew,
		PremiseAlignmentCategory("invalid"),
	)
	if err == nil {
		t.Fatal("expected error for invalid premise alignment category")
	}
}

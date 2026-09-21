package ai

import "testing"

func TestCueCategoryFromCount(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		expected CueCategory
	}{
		{
			name:     "one cue",
			count:    1,
			expected: CueCategoryFew,
		},
		{
			name:     "upper boundary of few",
			count:    8,
			expected: CueCategoryFew,
		},
		{
			name:     "lower boundary of some",
			count:    9,
			expected: CueCategorySome,
		},
		{
			name:     "upper boundary of some",
			count:    14,
			expected: CueCategorySome,
		},
		{
			name:     "lower boundary of many",
			count:    15,
			expected: CueCategoryMany,
		},
		{
			name:     "more than fifteen cues",
			count:    20,
			expected: CueCategoryMany,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CueCategoryFromCount(tt.count)
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

func TestCueCategoryFromCountInvalid(t *testing.T) {
	tests := []int{
		0,
		-1,
	}

	for _, count := range tests {
		_, err := CueCategoryFromCount(count)
		if err == nil {
			t.Fatalf("expected error for cue count %d", count)
		}
	}
}

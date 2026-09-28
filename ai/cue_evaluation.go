package ai

import "fmt"

type CueID string

type CueDetectionSource string

const (
	CueSourceDeterministic CueDetectionSource = "deterministic"
	CueSourceLLM           CueDetectionSource = "llm"
	CueSourceHybrid        CueDetectionSource = "hybrid"
)

type CueResult struct {
	ID       CueID              `json:"id"`
	MinCount int                `json:"min_count"`
	MaxCount int                `json:"max_count"`
	Source   CueDetectionSource `json:"source"`
	Evidence []string           `json:"evidence,omitempty"`
}

type CueEvaluation struct {
	Results            []CueResult   `json:"results"`
	MinCount           int           `json:"min_count"`
	MaxCount           int           `json:"max_count"`
	Category           CueCategory   `json:"category,omitempty"`
	MinCategory        CueCategory   `json:"min_category,omitempty"`
	MaxCategory        CueCategory   `json:"max_category,omitempty"`
	PossibleCategories []CueCategory `json:"possible_categories"`
	CategoryResolved   bool          `json:"category_resolved"`
	ZeroCountPossible  bool          `json:"zero_count_possible"`
}

func BuildCueEvaluation(results []CueResult) (CueEvaluation, error) {
	minTotal := 0
	maxTotal := 0
	seen := make(map[CueID]struct{})

	for _, result := range results {
		if result.ID == "" {
			return CueEvaluation{}, fmt.Errorf("cue ID cannot be empty")
		}

		if result.MinCount < 0 || result.MaxCount < 0 {
			return CueEvaluation{}, fmt.Errorf(
				"cue %q has invalid negative count range: %d-%d",
				result.ID,
				result.MinCount,
				result.MaxCount,
			)
		}

		if result.MinCount > result.MaxCount {
			return CueEvaluation{}, fmt.Errorf(
				"cue %q has invalid count range: %d-%d",
				result.ID,
				result.MinCount,
				result.MaxCount,
			)
		}

		if _, exists := seen[result.ID]; exists {
			return CueEvaluation{}, fmt.Errorf(
				"duplicate cue result: %q",
				result.ID,
			)
		}

		seen[result.ID] = struct{}{}
		minTotal += result.MinCount
		maxTotal += result.MaxCount
	}

	possibleCategories := cueCategoriesForRange(minTotal, maxTotal)
	zeroPossible := minTotal == 0

	evaluation := CueEvaluation{
		Results:            results,
		MinCount:           minTotal,
		MaxCount:           maxTotal,
		PossibleCategories: possibleCategories,
		ZeroCountPossible:  zeroPossible,
	}

	if len(possibleCategories) > 0 {
		evaluation.MinCategory = possibleCategories[0]
		evaluation.MaxCategory = possibleCategories[len(possibleCategories)-1]
	}

	evaluation.CategoryResolved = !zeroPossible && len(possibleCategories) == 1
	if evaluation.CategoryResolved {
		evaluation.Category = possibleCategories[0]
	}

	return evaluation, nil
}

func cueCategoriesForRange(minCount, maxCount int) []CueCategory {
	if maxCount < CueCountFewMin || minCount > maxCount {
		return nil
	}

	categories := make([]CueCategory, 0, 3)

	if rangesOverlap(minCount, maxCount, CueCountFewMin, CueCountFewMax) {
		categories = append(categories, CueCategoryFew)
	}

	if rangesOverlap(minCount, maxCount, CueCountSomeMin, CueCountSomeMax) {
		categories = append(categories, CueCategorySome)
	}

	if maxCount >= CueCountManyMin {
		categories = append(categories, CueCategoryMany)
	}

	return categories
}

func rangesOverlap(minA, maxA, minB, maxB int) bool {
	return minA <= maxB && maxA >= minB
}

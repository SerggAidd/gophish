package ai

import "fmt"

func BuildDifficultyEvaluation(
	cues CueEvaluation,
	premise PremiseAlignmentEvaluation,
) (DifficultyEvaluation, error) {
	if !validCueCategory(cues.Category) {
		return DifficultyEvaluation{}, fmt.Errorf(
			"invalid cue category: %q",
			cues.Category,
		)
	}

	premiseCategories, err := possiblePremiseAlignmentCategories(premise)
	if err != nil {
		return DifficultyEvaluation{}, err
	}

	possibleDifficulties := make([]DetectionDifficulty, 0, len(premiseCategories))
	seen := make(map[DetectionDifficulty]struct{})

	for _, premiseCategory := range premiseCategories {
		difficulty, err := CalculateDetectionDifficulty(
			cues.Category,
			premiseCategory,
		)
		if err != nil {
			return DifficultyEvaluation{}, err
		}

		if _, exists := seen[difficulty]; exists {
			continue
		}

		seen[difficulty] = struct{}{}
		possibleDifficulties = append(
			possibleDifficulties,
			difficulty,
		)
	}

	evaluation := DifficultyEvaluation{
		CueCategory:          cues.Category,
		PremiseAlignment:     premise,
		PossibleDifficulties: possibleDifficulties,
		Resolved:             len(possibleDifficulties) == 1,
	}

	if evaluation.Resolved {
		evaluation.DetectionDifficulty = possibleDifficulties[0]
	}

	return evaluation, nil
}

func possiblePremiseAlignmentCategories(
	evaluation PremiseAlignmentEvaluation,
) ([]PremiseAlignmentCategory, error) {
	if evaluation.MinScore < -8 || evaluation.MaxScore > 32 {
		return nil, fmt.Errorf(
			"premise alignment score range out of bounds: %d-%d",
			evaluation.MinScore,
			evaluation.MaxScore,
		)
	}

	if evaluation.MinScore > evaluation.MaxScore {
		return nil, fmt.Errorf(
			"invalid premise alignment score range: %d-%d",
			evaluation.MinScore,
			evaluation.MaxScore,
		)
	}

	categories := make([]PremiseAlignmentCategory, 0, 3)

	if evaluation.MinScore <= 10 {
		categories = append(categories, PremiseAlignmentWeak)
	}

	if evaluation.MinScore <= 17 && evaluation.MaxScore >= 11 {
		categories = append(categories, PremiseAlignmentMedium)
	}

	if evaluation.MaxScore >= 18 {
		categories = append(categories, PremiseAlignmentStrong)
	}

	if len(categories) == 0 {
		return nil, fmt.Errorf(
			"unable to determine premise alignment categories for range %d-%d",
			evaluation.MinScore,
			evaluation.MaxScore,
		)
	}

	return categories, nil
}

func validCueCategory(category CueCategory) bool {
	switch category {
	case CueCategoryFew, CueCategorySome, CueCategoryMany:
		return true
	default:
		return false
	}
}

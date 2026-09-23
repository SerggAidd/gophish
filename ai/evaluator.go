package ai

import "fmt"

func BuildDifficultyEvaluation(
	cues CueEvaluation,
	premise PremiseAlignmentEvaluation,
) (DifficultyEvaluation, error) {
	cueCategories, err := possibleCueCategories(cues)
	if err != nil {
		return DifficultyEvaluation{}, err
	}

	premiseCategories, err := possiblePremiseAlignmentCategories(premise)
	if err != nil {
		return DifficultyEvaluation{}, err
	}

	possibleDifficulties := make([]DetectionDifficulty, 0, len(cueCategories)*len(premiseCategories))
	seen := make(map[DetectionDifficulty]struct{})

	for _, cueCategory := range cueCategories {
		for _, premiseCategory := range premiseCategories {
			difficulty, err := CalculateDetectionDifficulty(
				cueCategory,
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
	}

	evaluation := DifficultyEvaluation{
		PossibleDifficulties:         possibleDifficulties,
		UnclassifiedCueCountPossible: cues.ZeroCountPossible,
		Resolved:                     len(possibleDifficulties) == 1 && !cues.ZeroCountPossible,
	}

	if evaluation.Resolved {
		evaluation.DetectionDifficulty = possibleDifficulties[0]
	}

	return evaluation, nil
}

func possibleCueCategories(evaluation CueEvaluation) ([]CueCategory, error) {
	if evaluation.MinCount < 0 || evaluation.MaxCount < 0 {
		return nil, fmt.Errorf(
			"cue count range out of bounds: %d-%d",
			evaluation.MinCount,
			evaluation.MaxCount,
		)
	}

	if evaluation.MinCount > evaluation.MaxCount {
		return nil, fmt.Errorf(
			"invalid cue count range: %d-%d",
			evaluation.MinCount,
			evaluation.MaxCount,
		)
	}

	categories := cueCategoriesForRange(
		evaluation.MinCount,
		evaluation.MaxCount,
	)

	if len(categories) == 0 {
		// NIST Table 3 starts at one cue. An exact zero-cue result is therefore
		// representable but not classifiable by the scale.
		if evaluation.MinCount == 0 && evaluation.MaxCount == 0 {
			return nil, nil
		}

		return nil, fmt.Errorf(
			"unable to determine cue categories for range %d-%d",
			evaluation.MinCount,
			evaluation.MaxCount,
		)
	}

	return categories, nil
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

func validDetectionDifficulty(difficulty DetectionDifficulty) bool {
	switch difficulty {
	case DifficultyVeryDifficult,
		DifficultyModeratelyDifficult,
		DifficultyModeratelyToLeastDifficult,
		DifficultyLeastDifficult:
		return true
	default:
		return false
	}
}

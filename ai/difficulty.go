package ai

import "fmt"

func CalculateDetectionDifficulty(
	cueCategory CueCategory,
	premiseCategory PremiseAlignmentCategory,
) (DetectionDifficulty, error) {
	switch cueCategory {
	case CueCategoryFew:
		switch premiseCategory {
		case PremiseAlignmentStrong, PremiseAlignmentMedium:
			return DifficultyVeryDifficult, nil
		case PremiseAlignmentWeak:
			return DifficultyModeratelyDifficult, nil
		}

	case CueCategorySome:
		switch premiseCategory {
		case PremiseAlignmentStrong:
			return DifficultyVeryDifficult, nil
		case PremiseAlignmentMedium:
			return DifficultyModeratelyDifficult, nil
		case PremiseAlignmentWeak:
			return DifficultyModeratelyToLeastDifficult, nil
		}

	case CueCategoryMany:
		switch premiseCategory {
		case PremiseAlignmentStrong, PremiseAlignmentMedium:
			return DifficultyModeratelyDifficult, nil
		case PremiseAlignmentWeak:
			return DifficultyLeastDifficult, nil
		}

	default:
		return "", fmt.Errorf("unknown cue category: %q", cueCategory)
	}

	return "", fmt.Errorf("unknown premise alignment category: %q", premiseCategory)
}

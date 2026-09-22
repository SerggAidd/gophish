package ai

import "fmt"

type TrainingExposure string

const (
	TrainingExposureUnknown     TrainingExposure = "unknown"
	TrainingExposureNone        TrainingExposure = "none"
	TrainingExposureLow         TrainingExposure = "low"
	TrainingExposureModerate    TrainingExposure = "moderate"
	TrainingExposureSignificant TrainingExposure = "significant"
	TrainingExposureExtreme     TrainingExposure = "extreme"
)

type PremiseAlignmentElementID string

const (
	PremiseElementWorkplaceProcess     PremiseAlignmentElementID = "workplace_process"
	PremiseElementWorkplaceRelevance   PremiseAlignmentElementID = "workplace_relevance"
	PremiseElementSituationalAlignment PremiseAlignmentElementID = "situational_alignment"
	PremiseElementConsequences         PremiseAlignmentElementID = "consequences_for_not_clicking"
	PremiseElementPriorExposure        PremiseAlignmentElementID = "prior_training_or_exposure"
)

type PremiseAlignmentElementResult struct {
	ID          PremiseAlignmentElementID `json:"id"`
	Score       *int                      `json:"score,omitempty"`
	Explanation string                    `json:"explanation,omitempty"`
}

type PremiseAlignmentEvaluation struct {
	Elements         []PremiseAlignmentElementResult `json:"elements"`
	MinScore         int                             `json:"min_score"`
	MaxScore         int                             `json:"max_score"`
	Category         PremiseAlignmentCategory        `json:"category,omitempty"`
	MinCategory      PremiseAlignmentCategory        `json:"min_category"`
	MaxCategory      PremiseAlignmentCategory        `json:"max_category"`
	CategoryResolved bool                            `json:"category_resolved"`
}

func BuildPremiseAlignmentEvaluation(
	results []PremiseAlignmentElementResult,
) (PremiseAlignmentEvaluation, error) {
	if len(results) != 5 {
		return PremiseAlignmentEvaluation{}, fmt.Errorf(
			"expected 5 premise alignment elements, got %d",
			len(results),
		)
	}

	expected := map[PremiseAlignmentElementID]struct{}{
		PremiseElementWorkplaceProcess:     {},
		PremiseElementWorkplaceRelevance:   {},
		PremiseElementSituationalAlignment: {},
		PremiseElementConsequences:         {},
		PremiseElementPriorExposure:        {},
	}

	scores := make(map[PremiseAlignmentElementID]*int, 5)

	for _, result := range results {
		if _, exists := expected[result.ID]; !exists {
			return PremiseAlignmentEvaluation{}, fmt.Errorf(
				"unknown premise alignment element: %q",
				result.ID,
			)
		}

		if _, exists := scores[result.ID]; exists {
			return PremiseAlignmentEvaluation{}, fmt.Errorf(
				"duplicate premise alignment element: %q",
				result.ID,
			)
		}

		if result.Score != nil && !validPremiseAlignmentScore(*result.Score) {
			return PremiseAlignmentEvaluation{}, fmt.Errorf(
				"invalid score %d for premise alignment element %q",
				*result.Score,
				result.ID,
			)
		}

		scores[result.ID] = result.Score
	}

	minScore := 0
	maxScore := 0

	positiveElements := []PremiseAlignmentElementID{
		PremiseElementWorkplaceProcess,
		PremiseElementWorkplaceRelevance,
		PremiseElementSituationalAlignment,
		PremiseElementConsequences,
	}

	for _, id := range positiveElements {
		score := scores[id]

		if score == nil {
			maxScore += 8
			continue
		}

		minScore += *score
		maxScore += *score
	}

	priorExposure := scores[PremiseElementPriorExposure]

	if priorExposure == nil {
		minScore -= 8
	} else {
		minScore -= *priorExposure
		maxScore -= *priorExposure
	}

	minCategory, err := PremiseAlignmentCategoryFromScore(minScore)
	if err != nil {
		return PremiseAlignmentEvaluation{}, err
	}

	maxCategory, err := PremiseAlignmentCategoryFromScore(maxScore)
	if err != nil {
		return PremiseAlignmentEvaluation{}, err
	}

	evaluation := PremiseAlignmentEvaluation{
		Elements:         results,
		MinScore:         minScore,
		MaxScore:         maxScore,
		MinCategory:      minCategory,
		MaxCategory:      maxCategory,
		CategoryResolved: minCategory == maxCategory,
	}

	if evaluation.CategoryResolved {
		evaluation.Category = minCategory
	}

	return evaluation, nil
}

func PremiseAlignmentCategoryFromScore(
	score int,
) (PremiseAlignmentCategory, error) {
	if score < -8 || score > 32 {
		return "", fmt.Errorf(
			"premise alignment score out of range: %d",
			score,
		)
	}

	switch {
	case score <= 10:
		return PremiseAlignmentWeak, nil
	case score <= 17:
		return PremiseAlignmentMedium, nil
	default:
		return PremiseAlignmentStrong, nil
	}
}

func TrainingExposureScore(
	exposure TrainingExposure,
) (*int, error) {
	if exposure == "" || exposure == TrainingExposureUnknown {
		return nil, nil
	}

	scores := map[TrainingExposure]int{
		TrainingExposureNone:        0,
		TrainingExposureLow:         2,
		TrainingExposureModerate:    4,
		TrainingExposureSignificant: 6,
		TrainingExposureExtreme:     8,
	}

	score, exists := scores[exposure]
	if !exists {
		return nil, fmt.Errorf(
			"unknown training exposure: %q",
			exposure,
		)
	}

	return &score, nil
}

func validPremiseAlignmentScore(score int) bool {
	switch score {
	case 0, 2, 4, 6, 8:
		return true
	default:
		return false
	}
}

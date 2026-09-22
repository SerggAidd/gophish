package ai

type CueCategory string

const (
	CueCategoryFew  CueCategory = "few"
	CueCategorySome CueCategory = "some"
	CueCategoryMany CueCategory = "many"
)

type PremiseAlignmentCategory string

const (
	PremiseAlignmentWeak   PremiseAlignmentCategory = "weak"
	PremiseAlignmentMedium PremiseAlignmentCategory = "medium"
	PremiseAlignmentStrong PremiseAlignmentCategory = "strong"
)

type DetectionDifficulty string

const (
	DifficultyVeryDifficult              DetectionDifficulty = "very_difficult"
	DifficultyModeratelyDifficult        DetectionDifficulty = "moderately_difficult"
	DifficultyModeratelyToLeastDifficult DetectionDifficulty = "moderately_to_least_difficult"
	DifficultyLeastDifficult             DetectionDifficulty = "least_difficult"
)

type DifficultyEvaluation struct {
	CueCategory          CueCategory                `json:"cue_category"`
	PremiseAlignment     PremiseAlignmentEvaluation `json:"premise_alignment"`
	DetectionDifficulty  DetectionDifficulty        `json:"detection_difficulty,omitempty"`
	PossibleDifficulties []DetectionDifficulty      `json:"possible_difficulties"`
	Resolved             bool                       `json:"resolved"`
}

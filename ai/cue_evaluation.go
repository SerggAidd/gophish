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
	Count    int                `json:"count"`
	Source   CueDetectionSource `json:"source"`
	Evidence []string           `json:"evidence,omitempty"`
}

type CueEvaluation struct {
	Results    []CueResult `json:"results"`
	TotalCount int         `json:"total_count"`
	Category   CueCategory `json:"category"`
}

func BuildCueEvaluation(results []CueResult) (CueEvaluation, error) {
	total := 0
	seen := make(map[CueID]struct{})

	for _, result := range results {
		if result.ID == "" {
			return CueEvaluation{}, fmt.Errorf("cue ID cannot be empty")
		}

		if result.Count < 0 {
			return CueEvaluation{}, fmt.Errorf(
				"cue %q has invalid negative count: %d",
				result.ID,
				result.Count,
			)
		}

		if _, exists := seen[result.ID]; exists {
			return CueEvaluation{}, fmt.Errorf(
				"duplicate cue result: %q",
				result.ID,
			)
		}

		seen[result.ID] = struct{}{}
		total += result.Count
	}

	category, err := CueCategoryFromCount(total)
	if err != nil {
		return CueEvaluation{}, fmt.Errorf(
			"unable to determine cue category: %w",
			err,
		)
	}

	return CueEvaluation{
		Results:    results,
		TotalCount: total,
		Category:   category,
	}, nil
}

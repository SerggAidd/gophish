package ai

import "fmt"

// DetectCueCriteria runs the currently available cue detectors for an email.
//
// At this stage only deterministic detectors are enabled. Semantic LLM-based
// detectors will be added later and combined with these results.
func DetectCueCriteria(email Email) ([]CueCriterionResult, error) {
	results := DetectDeterministicCueCriteria(email)

	// Reuse the existing aggregation validation so detector output cannot
	// introduce unknown criteria, invalid values, or duplicate criterion IDs.
	if _, err := BuildCueResults(results); err != nil {
		return nil, fmt.Errorf("validate detected cue criteria: %w", err)
	}

	return results, nil
}

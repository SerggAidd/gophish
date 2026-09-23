package ai

import (
	"context"
	"fmt"
)

type semanticCueEvaluator interface {
	Evaluate(
		ctx context.Context,
		input EvaluationInput,
	) ([]CueCriterionResult, error)
}

// CueDetector combines deterministic and semantic cue detection.
type CueDetector struct {
	semantic semanticCueEvaluator
}

func NewCueDetector(client *Client) *CueDetector {
	return &CueDetector{
		semantic: NewSemanticCueEvaluator(client),
	}
}

func (d *CueDetector) Detect(
	ctx context.Context,
	input EvaluationInput,
) ([]CueCriterionResult, error) {
	if d == nil || d.semantic == nil {
		return nil, fmt.Errorf("semantic cue evaluator is not configured")
	}

	if err := input.EvaluationContext.Validate(); err != nil {
		return nil, fmt.Errorf("validate evaluation context: %w", err)
	}

	deterministicResults := DetectDeterministicCueCriteria(input)

	semanticResults, err := d.semantic.Evaluate(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("detect semantic cues: %w", err)
	}

	results := make(
		[]CueCriterionResult,
		0,
		len(deterministicResults)+len(semanticResults),
	)

	results = append(results, deterministicResults...)
	results = append(results, semanticResults...)

	if _, err := BuildCueResults(results); err != nil {
		return nil, fmt.Errorf("validate detected cue criteria: %w", err)
	}

	return results, nil
}

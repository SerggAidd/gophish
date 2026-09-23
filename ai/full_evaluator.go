package ai

import (
	"context"
	"fmt"
)

type cueEvaluationProvider interface {
	Detect(ctx context.Context, input EvaluationInput) ([]CueCriterionResult, error)
}

type premiseEvaluationProvider interface {
	Evaluate(ctx context.Context, input EvaluationInput) (PremiseAlignmentEvaluation, error)
}

type EmailEvaluation struct {
	Criteria         []CueCriterionResult       `json:"criteria"`
	Cues             CueEvaluation              `json:"cues"`
	PremiseAlignment PremiseAlignmentEvaluation `json:"premise_alignment"`
	Difficulty       DifficultyEvaluation       `json:"difficulty"`
	MissingContext   []string                   `json:"missing_context,omitempty"`
	ContextComplete  bool                       `json:"context_complete"`
}

type EmailEvaluator struct {
	cues    cueEvaluationProvider
	premise premiseEvaluationProvider
}

func NewEmailEvaluator(client *Client) *EmailEvaluator {
	return &EmailEvaluator{
		cues:    NewCueDetector(client),
		premise: NewPremiseAlignmentEvaluator(client),
	}
}

func (e *EmailEvaluator) Evaluate(
	ctx context.Context,
	input EvaluationInput,
) (EmailEvaluation, error) {
	if e == nil || e.cues == nil || e.premise == nil {
		return EmailEvaluation{}, fmt.Errorf("email evaluator is not configured")
	}

	if err := input.EvaluationContext.Validate(); err != nil {
		return EmailEvaluation{}, fmt.Errorf("validate evaluation context: %w", err)
	}

	criteria, err := e.cues.Detect(ctx, input)
	if err != nil {
		return EmailEvaluation{}, fmt.Errorf("evaluate cues: %w", err)
	}

	cueResults, err := BuildCueResults(criteria)
	if err != nil {
		return EmailEvaluation{}, fmt.Errorf("build cue results: %w", err)
	}

	cueEvaluation, err := BuildCueEvaluation(cueResults)
	if err != nil {
		return EmailEvaluation{}, fmt.Errorf("build cue evaluation: %w", err)
	}

	premiseEvaluation, err := e.premise.Evaluate(ctx, input)
	if err != nil {
		return EmailEvaluation{}, fmt.Errorf("evaluate premise alignment: %w", err)
	}

	difficulty, err := BuildDifficultyEvaluation(cueEvaluation, premiseEvaluation)
	if err != nil {
		return EmailEvaluation{}, fmt.Errorf("build difficulty evaluation: %w", err)
	}

	missing := input.EvaluationContext.MissingFields()

	return EmailEvaluation{
		Criteria:         criteria,
		Cues:             cueEvaluation,
		PremiseAlignment: premiseEvaluation,
		Difficulty:       difficulty,
		MissingContext:   missing,
		ContextComplete:  len(missing) == 0,
	}, nil
}

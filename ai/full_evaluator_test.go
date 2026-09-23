package ai

import (
	"context"
	"testing"
)

type fakeCueProvider struct {
	results []CueCriterionResult
	err     error
}

func (f fakeCueProvider) Detect(ctx context.Context, input EvaluationInput) ([]CueCriterionResult, error) {
	return f.results, f.err
}

type fakePremiseProvider struct {
	result PremiseAlignmentEvaluation
	err    error
}

func (f fakePremiseProvider) Evaluate(ctx context.Context, input EvaluationInput) (PremiseAlignmentEvaluation, error) {
	return f.result, f.err
}

func TestEmailEvaluatorBuildsFullEvaluation(t *testing.T) {
	evaluator := &EmailEvaluator{
		cues: fakeCueProvider{results: []CueCriterionResult{
			{ID: CriterionHiddenURLLinks, MinValue: 3, MaxValue: 3, Source: CueSourceDeterministic},
		}},
		premise: fakePremiseProvider{result: PremiseAlignmentEvaluation{
			MinScore: 20, MaxScore: 20,
			Category: PremiseAlignmentStrong, MinCategory: PremiseAlignmentStrong, MaxCategory: PremiseAlignmentStrong,
			CategoryResolved: true,
		}},
	}

	input := EvaluationInput{EvaluationContext: completeEvaluationContext()}
	result, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ContextComplete {
		t.Fatalf("expected complete context, missing: %v", result.MissingContext)
	}
	if result.Cues.Category != CueCategoryFew {
		t.Fatalf("expected Few, got %q", result.Cues.Category)
	}
	if !result.Difficulty.Resolved || result.Difficulty.DetectionDifficulty != DifficultyVeryDifficult {
		t.Fatalf("unexpected difficulty: %#v", result.Difficulty)
	}
}

func TestEmailEvaluatorReportsPartialContext(t *testing.T) {
	evaluator := &EmailEvaluator{
		cues: fakeCueProvider{results: []CueCriterionResult{
			{ID: CriterionSenderDomainSpoofing, MinValue: 0, MaxValue: 1, Source: CueSourceDeterministic},
		}},
		premise: fakePremiseProvider{result: PremiseAlignmentEvaluation{MinScore: 12, MaxScore: 20}},
	}

	result, err := evaluator.Evaluate(context.Background(), EvaluationInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ContextComplete || len(result.MissingContext) == 0 {
		t.Fatal("expected partial evaluation")
	}
}

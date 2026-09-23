package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeSemanticCueEvaluator struct {
	results []CueCriterionResult
	err     error
}

func (f fakeSemanticCueEvaluator) Evaluate(ctx context.Context, input EvaluationInput) ([]CueCriterionResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

func completeEvaluationContext() EvaluationContext {
	return EvaluationContext{
		PriorTrainingExposure: TrainingExposureNone,
		SimulatedSender:       &SenderIdentity{Email: "training@example.test"},
		ExpectedSender:        &SenderIdentity{Email: "training@example.test"},
		SituationContext:      "Routine internal service activity is currently expected by the audience.",
		Link:                  LinkContext{Usage: LinkUsageNone},
		Attachments:           AttachmentContext{Usage: AttachmentUsageNone},
	}
}

func TestCueDetectorCombinesDeterministicAndSemanticResults(t *testing.T) {
	detector := &CueDetector{semantic: fakeSemanticCueEvaluator{results: []CueCriterionResult{{
		ID: CriterionTimePressure, MinValue: 1, MaxValue: 1, Source: CueSourceLLM,
		Evidence: []string{"Immediate action is required."},
	}}}}

	input := EvaluationInput{
		Email: Email{
			Subject: "Document review required",
			Text:    "Your document is ready for review.",
			HTML:    `<p>Your document is ready for review.</p><p><a href="{{.URL}}">Review document</a></p>`,
		},
		EvaluationContext: completeEvaluationContext(),
	}

	results, err := detector.Detect(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 combined criterion results, got %d", len(results))
	}

	byID := make(map[CueCriterionID]CueCriterionResult)
	for _, result := range results {
		byID[result.ID] = result
	}
	if byID[CriterionMissingGreeting].MinValue != 1 || byID[CriterionTimePressure].MinValue != 1 {
		t.Fatalf("expected deterministic and semantic findings, got %#v", byID)
	}
}

func TestCueDetectorSemanticFailure(t *testing.T) {
	detector := &CueDetector{semantic: fakeSemanticCueEvaluator{err: errors.New("semantic evaluation failed")}}
	_, err := detector.Detect(context.Background(), EvaluationInput{EvaluationContext: completeEvaluationContext()})
	if err == nil || !strings.Contains(err.Error(), "detect semantic cues") {
		t.Fatalf("expected wrapped semantic error, got %v", err)
	}
}

func TestCueDetectorRejectsOverlappingCriteria(t *testing.T) {
	detector := &CueDetector{semantic: fakeSemanticCueEvaluator{results: []CueCriterionResult{{
		ID: CriterionMissingGreeting, MinValue: 1, MaxValue: 1, Source: CueSourceLLM,
	}}}}
	_, err := detector.Detect(context.Background(), EvaluationInput{
		Email: Email{Text: "No greeting here."}, EvaluationContext: completeEvaluationContext(),
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate cue criterion result") {
		t.Fatalf("expected duplicate criterion error, got %v", err)
	}
}

func TestCueDetectorRequiresSemanticEvaluator(t *testing.T) {
	_, err := (&CueDetector{}).Detect(context.Background(), EvaluationInput{})
	if err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestCueDetectorRejectsInvalidEvaluationContext(t *testing.T) {
	detector := &CueDetector{semantic: fakeSemanticCueEvaluator{}}
	_, err := detector.Detect(context.Background(), EvaluationInput{EvaluationContext: EvaluationContext{
		Link: LinkContext{Usage: LinkUsageNone, SimulatedURL: "https://example.test"},
	}})
	if err == nil || !strings.Contains(err.Error(), "validate evaluation context") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

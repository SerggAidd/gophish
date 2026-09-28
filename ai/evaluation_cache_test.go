package ai

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

type snapshotCueProvider struct {
	calls     int32
	failFirst bool
	release   <-chan struct{}
	entered   chan struct{}
}

func (p *snapshotCueProvider) Detect(ctx context.Context, input EvaluationInput) ([]CueCriterionResult, error) {
	call := atomic.AddInt32(&p.calls, 1)
	if p.entered != nil && call == 1 {
		close(p.entered)
	}
	if p.release != nil {
		select {
		case <-p.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if p.failFirst && call == 1 {
		return nil, fmt.Errorf("temporary evaluator failure")
	}
	return []CueCriterionResult{{
		ID: CriterionHiddenURLLinks, MinValue: 1, MaxValue: 1, Source: CueSourceDeterministic,
		Evidence: []string{fmt.Sprintf("evaluation %d", call)},
	}}, nil
}

func cachedEvaluatorForTest(provider cueEvaluationProvider) *EmailEvaluator {
	return &EmailEvaluator{
		cues: provider,
		premise: fakePremiseProvider{result: PremiseAlignmentEvaluation{
			MinScore: 20, MaxScore: 20,
			Category: PremiseAlignmentStrong, MinCategory: PremiseAlignmentStrong, MaxCategory: PremiseAlignmentStrong,
			CategoryResolved: true,
		}},
		cache: newEvaluationCache(),
	}
}

func TestEvaluationSnapshotReusedOnlyForUnchangedInput(t *testing.T) {
	provider := &snapshotCueProvider{}
	evaluator := cachedEvaluatorForTest(provider)
	input := EvaluationInput{Email: Email{Subject: "Before"}, EvaluationContext: completeEvaluationContext()}
	first, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	first.Criteria[0].Evidence[0] = "caller mutated the result"
	second, err := evaluator.Evaluate(context.Background(), input)
	if err != nil || second.Criteria[0].Evidence[0] != "evaluation 1" {
		t.Fatalf("identical input must have the same independent snapshot: %#v, %v", second.Criteria, err)
	}
	input.GenerationContext.TargetDifficulty = "least_difficult"
	retargeted, err := evaluator.Evaluate(context.Background(), input)
	if err != nil || retargeted.Criteria[0].Evidence[0] != "evaluation 1" {
		t.Fatalf("target category must not change an unchanged email's score: %#v, %v", retargeted.Criteria, err)
	}
	input.Email.Subject = "After revision"
	changed, err := evaluator.Evaluate(context.Background(), input)
	if err != nil || changed.Criteria[0].Evidence[0] != "evaluation 2" {
		t.Fatalf("changing email must run a fresh evaluation: %#v, %v", changed.Criteria, err)
	}
	input.Email.Subject = "Before"
	input.EvaluationContext.Attachments = AttachmentContext{
		Usage: AttachmentUsageUsed, Files: []AttachmentMetadata{{Name: "policy.txt", Type: "text/plain"}},
	}
	attached, err := evaluator.Evaluate(context.Background(), input)
	if err != nil || attached.Criteria[0].Evidence[0] != "evaluation 3" {
		t.Fatalf("changing attachments must run a fresh evaluation: %#v, %v", attached.Criteria, err)
	}
	input.EvaluationContext.Attachments = AttachmentContext{Usage: AttachmentUsageNone}
	restored, err := evaluator.Evaluate(context.Background(), input)
	if err != nil || restored.Criteria[0].Evidence[0] != "evaluation 1" {
		t.Fatalf("restoring context must recover its snapshot: %#v, %v", restored.Criteria, err)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 3 {
		t.Fatalf("wanted three fresh evaluations, got %d", got)
	}
}

func TestEvaluationFailureIsNeverCached(t *testing.T) {
	provider := &snapshotCueProvider{failFirst: true}
	evaluator := cachedEvaluatorForTest(provider)
	input := EvaluationInput{EvaluationContext: completeEvaluationContext()}
	if _, err := evaluator.Evaluate(context.Background(), input); err == nil {
		t.Fatal("first evaluation should fail")
	}
	for i := 0; i < 2; i++ {
		result, err := evaluator.Evaluate(context.Background(), input)
		if err != nil || result.Criteria[0].Evidence[0] != "evaluation 2" {
			t.Fatalf("successful retry should be reused: %#v, %v", result.Criteria, err)
		}
	}
	if got := atomic.LoadInt32(&provider.calls); got != 2 {
		t.Fatalf("wanted one failure and one successful evaluation, got %d", got)
	}
}

func TestRefreshReplacesSnapshotAndSampleDoesNotTouchIt(t *testing.T) {
	provider := &snapshotCueProvider{}
	evaluator := cachedEvaluatorForTest(provider)
	input := EvaluationInput{Email: Email{Subject: "Same message"}, EvaluationContext: completeEvaluationContext()}
	check := func(label string, evaluate func(context.Context, EvaluationInput) (EmailEvaluation, error), want string) {
		t.Helper()
		result, err := evaluate(context.Background(), input)
		if err != nil || result.Criteria[0].Evidence[0] != want {
			t.Fatalf("%s: result %#v, err %v, expected %s", label, result.Criteria, err, want)
		}
	}
	check("initial", evaluator.Evaluate, "evaluation 1")
	check("independent sample", evaluator.EvaluateSample, "evaluation 2")
	check("sample must not replace", evaluator.Evaluate, "evaluation 1")
	check("manual refresh", evaluator.EvaluateRefresh, "evaluation 3")
	check("reuse new", evaluator.Evaluate, "evaluation 3")
	check("another refresh", evaluator.EvaluateRefresh, "evaluation 4")
	if got := atomic.LoadInt32(&provider.calls); got != 4 {
		t.Fatalf("wanted four real evaluator calls, got %d", got)
	}
	if got := len(evaluator.cache.order); got != 1 {
		t.Fatalf("refresh must replace one cache entry, got %d", got)
	}
}

func TestFailedRefreshPreservesLastSuccessfulSnapshot(t *testing.T) {
	provider := &snapshotCueProvider{}
	evaluator := cachedEvaluatorForTest(provider)
	input := EvaluationInput{EvaluationContext: completeEvaluationContext()}
	if _, err := evaluator.Evaluate(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	// The evaluator below fails on its second call; the cache must retain
	// the original result despite the failed fresh evaluation.
	evaluator.cues = fakeCueProvider{err: fmt.Errorf("model unavailable")}
	if _, err := evaluator.EvaluateRefresh(context.Background(), input); err == nil {
		t.Fatal("failed refresh succeeded")
	}
	evaluator.cues = provider
	result, err := evaluator.Evaluate(context.Background(), input)
	if err != nil || result.Criteria[0].Evidence[0] != "evaluation 1" {
		t.Fatalf("refresh must preserve old snapshot on failure: %#v, %v", result.Criteria, err)
	}
}

func TestConcurrentEvaluationOfSameInputHasOneProducer(t *testing.T) {
	release := make(chan struct{})
	provider := &snapshotCueProvider{release: release, entered: make(chan struct{})}
	evaluator := cachedEvaluatorForTest(provider)
	input := EvaluationInput{EvaluationContext: completeEvaluationContext()}
	var group sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := evaluator.Evaluate(context.Background(), input)
			if err == nil && result.Criteria[0].Evidence[0] != "evaluation 1" {
				err = fmt.Errorf("unexpected snapshot: %#v", result.Criteria)
			}
			errors <- err
		}()
	}
	<-provider.entered
	close(release)
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&provider.calls); got != 1 {
		t.Fatalf("wanted one shared evaluation, got %d", got)
	}
}

func TestDifficultyAgentCanShareEvaluatorWithEvaluateRoute(t *testing.T) {
	shared := cachedEvaluatorForTest(&snapshotCueProvider{})
	if agent := NewDifficultyAgentWithEvaluator(nil, shared); agent.evaluator != shared {
		t.Fatal("difficulty agent must reuse the evaluator configured for the API route")
	}
}

package ai

import (
	"context"
	"strings"
	"testing"
)

type fakeDifficultyEvaluator struct {
	results []EmailEvaluation
	index   int
}

func (f *fakeDifficultyEvaluator) Evaluate(ctx context.Context, input EvaluationInput) (EmailEvaluation, error) {
	if f.index >= len(f.results) {
		return f.results[len(f.results)-1], nil
	}
	result := f.results[f.index]
	f.index++
	return result, nil
}

type fakeDifficultyGenerator struct {
	generated Email
	revisions []Email
	index     int
}

func (f *fakeDifficultyGenerator) GenerateWithContext(ctx context.Context, req GenerationRequest, evaluationContext EvaluationContext) (Email, error) {
	return f.generated, nil
}

func (f *fakeDifficultyGenerator) Revise(ctx context.Context, req RevisionRequest) (Email, error) {
	if f.index >= len(f.revisions) {
		return req.Email, nil
	}
	email := f.revisions[f.index]
	f.index++
	return email, nil
}

func resolvedEvaluation(difficulty DetectionDifficulty) EmailEvaluation {
	return EmailEvaluation{Difficulty: DifficultyEvaluation{
		DetectionDifficulty:  difficulty,
		PossibleDifficulties: []DetectionDifficulty{difficulty},
		Resolved:             true,
	}}
}

func TestDifficultyAgentAdjustsUntilTargetReached(t *testing.T) {
	evaluator := &fakeDifficultyEvaluator{results: []EmailEvaluation{
		resolvedEvaluation(DifficultyModeratelyDifficult),
		resolvedEvaluation(DifficultyVeryDifficult),
	}}
	generator := &fakeDifficultyGenerator{revisions: []Email{{Subject: "Revised", Text: "x", HTML: "<p>x</p>"}}}
	agent := &DifficultyAgent{evaluator: evaluator, generator: generator}

	result, err := agent.Adjust(context.Background(), DifficultyAdjustmentRequest{
		Input:         EvaluationInput{Email: Email{Subject: "Initial", Text: "x", HTML: "<p>x</p>"}},
		Target:        DifficultyVeryDifficult,
		MaxIterations: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != DifficultyAdjustmentReached || result.Iterations != 1 {
		t.Fatalf("unexpected adjustment result: %#v", result)
	}
}

func TestDifficultyAgentStopsWhenTargetAlreadyPossibleButUnconfirmed(t *testing.T) {
	evaluation := EmailEvaluation{Difficulty: DifficultyEvaluation{
		PossibleDifficulties: []DetectionDifficulty{DifficultyModeratelyDifficult, DifficultyVeryDifficult},
		Resolved:             false,
	}}
	agent := &DifficultyAgent{
		evaluator: &fakeDifficultyEvaluator{results: []EmailEvaluation{evaluation}},
		generator: &fakeDifficultyGenerator{},
	}

	result, err := agent.Adjust(context.Background(), DifficultyAdjustmentRequest{
		Input:  EvaluationInput{Email: Email{Subject: "Initial"}},
		Target: DifficultyVeryDifficult,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != DifficultyAdjustmentPossibleUnconfirmed || result.Iterations != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestDifficultyAgentGenerateAndAdjust(t *testing.T) {
	initial := Email{Subject: "Generated", Text: "body", HTML: "<p>body</p>"}
	agent := &DifficultyAgent{
		evaluator: &fakeDifficultyEvaluator{results: []EmailEvaluation{resolvedEvaluation(DifficultyVeryDifficult)}},
		generator: &fakeDifficultyGenerator{generated: initial},
	}

	result, err := agent.GenerateAndAdjust(context.Background(), DifficultyGenerationRequest{
		GenerationContext: GenerationRequest{TargetDifficulty: string(DifficultyVeryDifficult)},
		EvaluationContext: completeEvaluationContext(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != DifficultyAdjustmentReached || result.Email.Subject != "Generated" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestBuildContextChangeSuggestionsIncludesFixedFactors(t *testing.T) {
	evaluation := EmailEvaluation{Criteria: []CueCriterionResult{
		{ID: CriterionSenderDomainSpoofing, MinValue: 1, MaxValue: 1},
		{ID: CriterionAttachments, MinValue: 1, MaxValue: 1},
	}}
	suggestions := buildContextChangeSuggestions(evaluation, EvaluationContext{PriorTrainingExposure: TrainingExposureModerate})
	if len(suggestions) < 3 {
		t.Fatalf("expected fixed-context suggestions, got %v", suggestions)
	}
}

func TestBuildDifficultyRevisionFeedbackMakesVeryDifficultEmailEasierToDetect(t *testing.T) {
	evaluation := resolvedEvaluation(DifficultyVeryDifficult)
	evaluation.Cues = CueEvaluation{
		MinCount:    13,
		MaxCount:    13,
		MinCategory: CueCategorySome,
		MaxCategory: CueCategorySome,
	}
	evaluation.PremiseAlignment = PremiseAlignmentEvaluation{
		MinScore:    24,
		MaxScore:    24,
		MinCategory: PremiseAlignmentStrong,
		MaxCategory: PremiseAlignmentStrong,
	}

	feedback := buildDifficultyRevisionFeedback(
		evaluation,
		DifficultyModeratelyDifficult,
		"",
	)

	if !strings.Contains(feedback, "EASIER") || !strings.Contains(feedback, "Increase email-controlled detection cues") {
		t.Fatalf("unexpected feedback: %s", feedback)
	}
}

func TestBuildDifficultyRevisionFeedbackMakesLeastDifficultEmailHarderToDetect(t *testing.T) {
	evaluation := resolvedEvaluation(DifficultyLeastDifficult)
	evaluation.Criteria = []CueCriterionResult{
		{ID: CriterionTimePressure, MinValue: 1, MaxValue: 1, Evidence: []string{"Urgent."}},
	}

	feedback := buildDifficultyRevisionFeedback(
		evaluation,
		DifficultyVeryDifficult,
		"",
	)

	if !strings.Contains(feedback, "HARDER") || !strings.Contains(feedback, string(CriterionTimePressure)) {
		t.Fatalf("unexpected feedback: %s", feedback)
	}
}

func TestBuildDifficultyRevisionFeedbackIncludesConcreteLeastDifficultRoute(t *testing.T) {
	evaluation := resolvedEvaluation(DifficultyModeratelyDifficult)
	evaluation.Cues = CueEvaluation{
		Category:         CueCategorySome,
		MinCount:         9,
		MaxCount:         9,
		MinCategory:      CueCategorySome,
		MaxCategory:      CueCategorySome,
		CategoryResolved: true,
	}
	evaluation.PremiseAlignment = PremiseAlignmentEvaluation{
		Category:         PremiseAlignmentMedium,
		MinScore:         14,
		MaxScore:         14,
		MinCategory:      PremiseAlignmentMedium,
		MaxCategory:      PremiseAlignmentMedium,
		CategoryResolved: true,
	}

	feedback := buildDifficultyRevisionFeedback(
		evaluation,
		DifficultyLeastDifficult,
		"",
	)

	if !strings.Contains(feedback, "cue category many (at least 15 total cues)") {
		t.Fatalf("expected concrete cue target, got: %s", feedback)
	}
	if !strings.Contains(feedback, "premise alignment weak (score <= 10)") {
		t.Fatalf("expected concrete premise target, got: %s", feedback)
	}
	if !strings.Contains(feedback, "Never disclose that it is phishing") {
		t.Fatalf("expected simulation-realism guardrail, got: %s", feedback)
	}
}

func TestClosestNISTTargetRouteChoosesNearestModerateRoute(t *testing.T) {
	evaluation := EmailEvaluation{
		Cues: CueEvaluation{
			Category:         CueCategorySome,
			MinCategory:      CueCategorySome,
			MaxCategory:      CueCategorySome,
			CategoryResolved: true,
		},
		PremiseAlignment: PremiseAlignmentEvaluation{
			Category:         PremiseAlignmentStrong,
			MinCategory:      PremiseAlignmentStrong,
			MaxCategory:      PremiseAlignmentStrong,
			CategoryResolved: true,
		},
	}

	route, ok := closestNISTTargetRoute(evaluation, DifficultyModeratelyDifficult)
	if !ok {
		t.Fatal("expected target route")
	}

	if route.CueCategory != CueCategorySome || route.PremiseCategory != PremiseAlignmentMedium {
		t.Fatalf("unexpected route: %#v", route)
	}
}

func routeEvaluation(
	difficulty DetectionDifficulty,
	cueCount int,
	cueCategory CueCategory,
	premiseScore int,
	premiseCategory PremiseAlignmentCategory,
) EmailEvaluation {
	return EmailEvaluation{
		Cues: CueEvaluation{
			MinCount:           cueCount,
			MaxCount:           cueCount,
			Category:           cueCategory,
			MinCategory:        cueCategory,
			MaxCategory:        cueCategory,
			PossibleCategories: []CueCategory{cueCategory},
			CategoryResolved:   true,
		},
		PremiseAlignment: PremiseAlignmentEvaluation{
			MinScore:         premiseScore,
			MaxScore:         premiseScore,
			Category:         premiseCategory,
			MinCategory:      premiseCategory,
			MaxCategory:      premiseCategory,
			CategoryResolved: true,
		},
		Difficulty: DifficultyEvaluation{
			DetectionDifficulty:  difficulty,
			PossibleDifficulties: []DetectionDifficulty{difficulty},
			Resolved:             true,
		},
	}
}

func TestDifficultyAgentRejectsWorseCandidateAndAcceptsImprovingRetry(t *testing.T) {
	initial := routeEvaluation(
		DifficultyModeratelyDifficult,
		10,
		CueCategorySome,
		14,
		PremiseAlignmentMedium,
	)
	worse := routeEvaluation(
		DifficultyVeryDifficult,
		12,
		CueCategorySome,
		28,
		PremiseAlignmentStrong,
	)
	target := routeEvaluation(
		DifficultyLeastDifficult,
		16,
		CueCategoryMany,
		8,
		PremiseAlignmentWeak,
	)

	evaluator := &fakeDifficultyEvaluator{results: []EmailEvaluation{
		initial,
		worse,
		target,
	}}
	generator := &fakeDifficultyGenerator{revisions: []Email{
		{Subject: "Worse", Text: "worse", HTML: "<p>worse</p>"},
		{Subject: "Improved", Text: "improved", HTML: "<p>improved</p>"},
	}}
	agent := &DifficultyAgent{evaluator: evaluator, generator: generator}

	result, err := agent.Adjust(context.Background(), DifficultyAdjustmentRequest{
		Input: EvaluationInput{
			Email: Email{Subject: "Initial", Text: "body", HTML: "<p>body</p>"},
		},
		Target:        DifficultyLeastDifficult,
		MaxIterations: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != DifficultyAdjustmentReached {
		t.Fatalf("expected target to be reached, got %#v", result)
	}
	if result.Iterations != 1 {
		t.Fatalf("expected one accepted iteration, got %d", result.Iterations)
	}
	if result.Email.Subject != "Improved" {
		t.Fatalf("expected improving retry to be accepted, got %#v", result.Email)
	}
	if len(result.History) != 2 {
		t.Fatalf("expected history to contain initial + accepted candidate, got %d steps", len(result.History))
	}
}

func TestRevisionCandidatePreservesSatisfiedRouteDimension(t *testing.T) {
	current := routeEvaluation(
		DifficultyModeratelyDifficult,
		16,
		CueCategoryMany,
		20,
		PremiseAlignmentStrong,
	)
	candidate := routeEvaluation(
		DifficultyModeratelyToLeastDifficult,
		14,
		CueCategorySome,
		8,
		PremiseAlignmentWeak,
	)

	route := nistTargetRoute{
		CueCategory:     CueCategoryMany,
		PremiseCategory: PremiseAlignmentWeak,
	}

	if revisionCandidateImprovesRoute(current, candidate, route) {
		t.Fatal("candidate should be rejected because it loses an already-satisfied cue category")
	}
}

func TestValidateRevisionCandidateRejectsEvaluatorContextLeak(t *testing.T) {
	original := Email{
		Subject: "Payroll document requires review",
		Text:    "Please review the document.",
		HTML:    "<p>Please review the document.</p>",
	}
	revised := Email{
		Subject: "Payroll document requires review",
		Text:    "Please review the document. Contact support@microsoft.com if needed.",
		HTML:    "<p>Please review the document. Contact support@microsoft.com if needed.</p>",
	}
	context := EvaluationContext{
		ExpectedSender: &SenderIdentity{
			DisplayName: "Microsoft 365 Support",
			Email:       "support@microsoft.com",
		},
		Link: LinkContext{
			Usage:          LinkUsageUsed,
			ExpectedDomain: "microsoft.com",
		},
	}

	if err := validateRevisionCandidate(original, revised, context); err == nil {
		t.Fatal("expected evaluator-only expected sender/domain leak to be rejected")
	}
}

func TestValidateRevisionCandidateRejectsSimulationDisclosure(t *testing.T) {
	original := Email{
		Subject: "Document review",
		Text:    "Please review the document.",
		HTML:    "<p>Please review the document.</p>",
	}
	revised := Email{
		Subject: "Document review",
		Text:    "This is a phishing simulation. Do not click the link.",
		HTML:    "<p>This is a phishing simulation. Do not click the link.</p>",
	}

	if err := validateRevisionCandidate(original, revised, EvaluationContext{}); err == nil {
		t.Fatal("expected simulation disclosure to be rejected")
	}
}

func TestValidateRevisionCandidateRejectsNewHardCodedExternalURL(t *testing.T) {
	original := Email{
		Subject: "Document review",
		Text:    "Please review the document through the portal.",
		HTML:    `<p>Please <a href="{{.URL}}">review the document</a>.</p>`,
	}
	revised := Email{
		Subject: "Document review",
		Text:    "Please review the document through the portal.",
		HTML:    `<p>Please <a href="{{.URL}}">review the document</a>.</p><img src="https://rnicrosoft.com/logo.png"><a href="https://login.rnicrosoft.com/files/doc.pdf">Download</a>`,
	}

	if err := validateRevisionCandidate(original, revised, EvaluationContext{}); err == nil {
		t.Fatal("expected newly introduced hard-coded external URLs to be rejected")
	}
}

func TestValidateRevisionCandidateAllowsExistingHardCodedURL(t *testing.T) {
	original := Email{
		Subject: "Document review",
		Text:    "Please review the document.",
		HTML:    `<p>Company site: <a href="https://example.com/help">Help</a></p>`,
	}
	revised := Email{
		Subject: "Document review",
		Text:    "Please review the document today.",
		HTML:    `<p>Please review the document today.</p><p>Company site: <a href="https://example.com/help">Help</a></p>`,
	}

	if err := validateRevisionCandidate(original, revised, EvaluationContext{}); err != nil {
		t.Fatalf("expected existing hard-coded URL to remain allowed, got %v", err)
	}
}

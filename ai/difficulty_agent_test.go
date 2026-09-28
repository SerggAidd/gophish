package ai

import (
	"context"
	"fmt"
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
	generated    Email
	revisions    []Email
	revisionErrs []error
	feedbacks    []string
	index        int
}

func (f *fakeDifficultyGenerator) GenerateWithContext(ctx context.Context, req GenerationRequest, evaluationContext EvaluationContext) (Email, error) {
	return f.generated, nil
}

func (f *fakeDifficultyGenerator) Revise(ctx context.Context, req RevisionRequest) (Email, error) {
	f.feedbacks = append(f.feedbacks, req.Feedback)
	idx := f.index
	f.index++
	if idx < len(f.revisionErrs) && f.revisionErrs[idx] != nil {
		return Email{}, f.revisionErrs[idx]
	}
	if idx >= len(f.revisions) {
		return req.Email, nil
	}
	return f.revisions[idx], nil
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

func TestClosestNISTTargetRoutePreservesMediumPremise(t *testing.T) {
	// A plausible policy note has two cues and a medium premise. Both
	// Few+Weak and Some+Medium are one category away, but the latter
	// requires editing only the email instead of changing fixed context.
	evaluation := routeEvaluation(
		DifficultyVeryDifficult,
		2,
		CueCategoryFew,
		16,
		PremiseAlignmentMedium,
	)
	route, ok := closestNISTTargetRoute(evaluation, DifficultyModeratelyDifficult)
	if !ok || route.CueCategory != CueCategorySome || route.PremiseCategory != PremiseAlignmentMedium {
		t.Fatalf("expected Some+Medium route, got %#v (ok=%t)", route, ok)
	}
}

func TestDifficultyAgentMovesFewMediumToSomeMedium(t *testing.T) {
	initial := routeEvaluation(DifficultyVeryDifficult, 2, CueCategoryFew, 16, PremiseAlignmentMedium)
	target := routeEvaluation(DifficultyModeratelyDifficult, 9, CueCategorySome, 16, PremiseAlignmentMedium)
	generator := &fakeDifficultyGenerator{revisions: []Email{{
		Subject: "Revised", Text: "review", HTML: "<p>review</p>",
	}}}
	agent := &DifficultyAgent{
		evaluator: &fakeDifficultyEvaluator{results: []EmailEvaluation{initial, target}},
		generator: generator,
	}
	result, err := agent.Adjust(context.Background(), DifficultyAdjustmentRequest{
		Input: EvaluationInput{Email: Email{Subject: "Initial", Text: "policy", HTML: "<p>policy</p>"}},
		Target: DifficultyModeratelyDifficult,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != DifficultyAdjustmentReached || result.Iterations != 1 {
		t.Fatalf("expected accepted Some+Medium revision, got %#v", result)
	}
	if len(generator.feedbacks) != 1 || !strings.Contains(generator.feedbacks[0], "cue category some") ||
		!strings.Contains(generator.feedbacks[0], "premise alignment medium") {
		t.Fatalf("feedback did not select Some+Medium: %v", generator.feedbacks)
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

func TestValidateInitialGenerationCandidateRejectsExpectedIdentityLeak(t *testing.T) {
	email := Email{
		Subject: "Password expiration",
		Text:    "Contact notifications@innocorp.example for assistance.",
		HTML:    "<p>Contact notifications@innocorp.example for assistance.</p>",
	}
	context := EvaluationContext{
		SimulatedSender: &SenderIdentity{Email: "notifications@inn0corp.test"},
		ExpectedSender:  &SenderIdentity{Email: "notifications@innocorp.example"},
		Link: LinkContext{
			Usage:          LinkUsageUsed,
			SimulatedURL:   "https://login.inn0corp.test/reset",
			ExpectedDomain: "innocorp.example",
		},
	}

	if err := validateInitialGenerationCandidate(email, context); err == nil {
		t.Fatal("expected evaluator-only expected identity leak to be rejected")
	}
}

func TestValidateInitialGenerationCandidateAllowsSimulatedIdentity(t *testing.T) {
	email := Email{
		Subject: "Password expiration",
		Text:    "Contact notifications@inn0corp.test for assistance.",
		HTML:    "<p>Contact notifications@inn0corp.test for assistance.</p>",
	}
	context := EvaluationContext{
		SimulatedSender: &SenderIdentity{Email: "notifications@inn0corp.test"},
		ExpectedSender:  &SenderIdentity{Email: "notifications@innocorp.example"},
		Link: LinkContext{
			Usage:          LinkUsageUsed,
			SimulatedURL:   "https://login.inn0corp.test/reset",
			ExpectedDomain: "innocorp.example",
		},
	}

	if err := validateInitialGenerationCandidate(email, context); err != nil {
		t.Fatalf("expected simulated identity to remain allowed, got %v", err)
	}
}

func TestValidateRevisionRequestCandidateRejectsInventedEmailAddress(t *testing.T) {
	request := RevisionRequest{
		Email: Email{
			Subject: "Finance review",
			Text:    "Please review the assigned finance request.",
			HTML:    "<p>Please review the assigned finance request.</p>",
		},
		OrganizationContext: "InnoCorp finance employees use an internal portal.",
		SenderContext:       "The message appears to come from Finance Operations.",
		EvaluationContext: &EvaluationContext{
			SimulatedSender: &SenderIdentity{Email: "notifications@innocorp.example"},
		},
	}
	revised := Email{
		Subject: "Finance review",
		Text:    "Please review the request. Questions: finance@innocorp.example",
		HTML:    "<p>Please review the request. Questions: finance@innocorp.example</p>",
	}

	if err := validateRevisionRequestCandidate(request, revised); err == nil {
		t.Fatal("expected invented contact email to be rejected")
	}
}

func TestValidateRevisionRequestCandidateAllowsEmailFromOriginalOrContext(t *testing.T) {
	request := RevisionRequest{
		Email: Email{
			Subject: "Finance review",
			Text:    "Contact helpdesk@innocorp.example if needed.",
			HTML:    "<p>Contact helpdesk@innocorp.example if needed.</p>",
		},
		Feedback: "Keep helpdesk@innocorp.example as the support address.",
		EvaluationContext: &EvaluationContext{
			SimulatedSender: &SenderIdentity{Email: "notifications@innocorp.example"},
		},
	}
	revised := Email{
		Subject: "Finance review",
		Text:    "Contact helpdesk@innocorp.example or notifications@innocorp.example if needed.",
		HTML:    "<p>Contact helpdesk@innocorp.example or notifications@innocorp.example if needed.</p>",
	}

	if err := validateRevisionRequestCandidate(request, revised); err != nil {
		t.Fatalf("expected original/simulated sender addresses to remain allowed, got %v", err)
	}
}

func TestValidateRevisionRequestCandidateRejectsInventedPhoneNumber(t *testing.T) {
	request := RevisionRequest{
		Email: Email{
			Subject: "Finance review",
			Text:    "Please review the assigned finance request.",
			HTML:    "<p>Please review the assigned finance request.</p>",
		},
		OrganizationContext: "InnoCorp finance employees use an internal portal.",
		SenderContext:       "The message appears to come from Finance Operations.",
	}
	revised := Email{
		Subject: "Finance review",
		Text:    "Please review the request. Call 555-123-4567 if you have questions.",
		HTML:    "<p>Please review the request. Call 555-123-4567 if you have questions.</p>",
	}

	if err := validateRevisionRequestCandidate(request, revised); err == nil {
		t.Fatal("expected invented phone number to be rejected")
	}
}

func TestValidateRevisionRequestCandidateAllowsPhoneNumberFromOriginalOrFeedback(t *testing.T) {
	request := RevisionRequest{
		Email: Email{
			Subject: "Finance review",
			Text:    "Call +1 (555) 123-4567 if needed.",
			HTML:    "<p>Call +1 (555) 123-4567 if needed.</p>",
		},
		Feedback: "Keep +1 (555) 123-4567 as the support number.",
	}
	revised := Email{
		Subject: "Finance review",
		Text:    "Questions? Call +1 555 123 4567.",
		HTML:    "<p>Questions? Call +1 555 123 4567.</p>",
	}

	if err := validateRevisionRequestCandidate(request, revised); err != nil {
		t.Fatalf("expected original/user-supplied phone number to remain allowed, got %v", err)
	}
}

func TestPhoneNumberSetIgnoresDateLikeShortNumericStrings(t *testing.T) {
	values := phoneNumberSet("Deadline: 2026-09-24. Ticket 123-45-67.")
	if len(values) != 0 {
		t.Fatalf("expected date/short numeric strings not to be treated as full phone numbers, got %v", values)
	}
}

func TestDifficultyAgentRejectsGroundingFailureAndTriesAnotherCandidate(t *testing.T) {
	evaluator := &fakeDifficultyEvaluator{results: []EmailEvaluation{
		resolvedEvaluation(DifficultyModeratelyDifficult),
		resolvedEvaluation(DifficultyVeryDifficult),
	}}
	groundingErr := &RevisionGroundingError{
		Attempts: 2,
		Err:      fmt.Errorf("revision introduced simulation or defensive disclosure %q", "do not click"),
	}
	generator := &fakeDifficultyGenerator{
		revisionErrs: []error{groundingErr, nil},
		revisions: []Email{
			{},
			{Subject: "Revised", Text: "x", HTML: "<p>x</p>"},
		},
	}
	agent := &DifficultyAgent{evaluator: evaluator, generator: generator}

	result, err := agent.Adjust(context.Background(), DifficultyAdjustmentRequest{
		Input:         EvaluationInput{Email: Email{Subject: "Initial", Text: "x", HTML: "<p>x</p>"}},
		Target:        DifficultyVeryDifficult,
		MaxIterations: 1,
	})
	if err != nil {
		t.Fatalf("expected grounding failure to reject only the candidate, got %v", err)
	}
	if result.Status != DifficultyAdjustmentReached || result.Iterations != 1 {
		t.Fatalf("expected second candidate to reach target, got %#v", result)
	}
}

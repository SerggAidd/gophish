package ai

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const DefaultDifficultyRevisionIterations = 3
const DefaultDifficultyRevisionCandidateAttempts = 2
const DefaultInitialGenerationAttempts = 2

var absoluteURLPattern = regexp.MustCompile(`(?i)https?://[^\s\"'<>]+`)
var emailAddressPattern = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
var possiblePhoneNumberPattern = regexp.MustCompile(`(?:\+?\d[\d\s().\-]{7,}\d)`)

type difficultyEmailEvaluator interface {
	Evaluate(ctx context.Context, input EvaluationInput) (EmailEvaluation, error)
}

type difficultyEmailGenerator interface {
	GenerateWithContext(ctx context.Context, req GenerationRequest, evaluationContext EvaluationContext) (Email, error)
	Revise(ctx context.Context, req RevisionRequest) (Email, error)
}

type DifficultyAdjustmentStatus string

const (
	DifficultyAdjustmentReached             DifficultyAdjustmentStatus = "reached"
	DifficultyAdjustmentPossibleUnconfirmed DifficultyAdjustmentStatus = "possible_unconfirmed"
	DifficultyAdjustmentNotReached          DifficultyAdjustmentStatus = "not_reached"
)

type DifficultyAdjustmentRequest struct {
	Input         EvaluationInput     `json:"input"`
	Target        DetectionDifficulty `json:"target"`
	UserFeedback  string              `json:"user_feedback,omitempty"`
	MaxIterations int                 `json:"max_iterations,omitempty"`
}

type DifficultyAdjustmentStep struct {
	Iteration  int             `json:"iteration"`
	Email      Email           `json:"email"`
	Evaluation EmailEvaluation `json:"evaluation"`
}

type DifficultyAdjustmentResult struct {
	Email                    Email                      `json:"email"`
	Evaluation               EmailEvaluation            `json:"evaluation"`
	Status                   DifficultyAdjustmentStatus `json:"status"`
	Iterations               int                        `json:"iterations"`
	History                  []DifficultyAdjustmentStep `json:"history"`
	ContextChangeSuggestions []string                   `json:"context_change_suggestions,omitempty"`
}

type DifficultyAgent struct {
	evaluator difficultyEmailEvaluator
	generator difficultyEmailGenerator
}

func NewDifficultyAgent(client *Client) *DifficultyAgent {
	return NewDifficultyAgentWithEvaluator(client, NewEmailEvaluator(client))
}

// NewDifficultyAgentWithEvaluator shares the same validated evaluation
// snapshots with the separate Evaluate API route.
func NewDifficultyAgentWithEvaluator(client *Client, evaluator *EmailEvaluator) *DifficultyAgent {
	return &DifficultyAgent{
		evaluator: evaluator,
		generator: NewGenerator(client),
	}
}

type DifficultyGenerationRequest struct {
	GenerationContext GenerationRequest `json:"generation_context"`
	EvaluationContext EvaluationContext `json:"evaluation_context"`
	MaxIterations     int               `json:"max_iterations,omitempty"`
}

// GenerateAndAdjust performs the complete AI generation flow: initial
// generation with fixed campaign context, evaluation, and feedback-directed
// revision toward the requested target difficulty.
func (a *DifficultyAgent) GenerateAndAdjust(
	ctx context.Context,
	request DifficultyGenerationRequest,
) (DifficultyAdjustmentResult, error) {
	if a == nil || a.evaluator == nil || a.generator == nil {
		return DifficultyAdjustmentResult{}, fmt.Errorf("difficulty agent is not configured")
	}

	target := DetectionDifficulty(request.GenerationContext.TargetDifficulty)
	if !validDetectionDifficulty(target) {
		return DifficultyAdjustmentResult{}, fmt.Errorf(
			"invalid target difficulty: %q",
			request.GenerationContext.TargetDifficulty,
		)
	}

	var email Email
	var generationErr error

	for attempt := 1; attempt <= DefaultInitialGenerationAttempts; attempt++ {
		email, generationErr = a.generator.GenerateWithContext(
			ctx,
			request.GenerationContext,
			request.EvaluationContext,
		)
		if generationErr != nil {
			return DifficultyAdjustmentResult{}, fmt.Errorf("generate initial email: %w", generationErr)
		}

		if generationErr = validateInitialGenerationCandidate(email, request.EvaluationContext); generationErr == nil {
			break
		}
	}

	if generationErr != nil {
		return DifficultyAdjustmentResult{}, fmt.Errorf(
			"generated email violated evaluator-context isolation after %d attempts: %w",
			DefaultInitialGenerationAttempts,
			generationErr,
		)
	}

	return a.Adjust(ctx, DifficultyAdjustmentRequest{
		Input: EvaluationInput{
			Email:             email,
			GenerationContext: request.GenerationContext,
			EvaluationContext: request.EvaluationContext,
		},
		Target:        target,
		MaxIterations: request.MaxIterations,
	})
}

func (a *DifficultyAgent) Adjust(
	ctx context.Context,
	request DifficultyAdjustmentRequest,
) (DifficultyAdjustmentResult, error) {
	if a == nil || a.evaluator == nil || a.generator == nil {
		return DifficultyAdjustmentResult{}, fmt.Errorf("difficulty agent is not configured")
	}

	if !validDetectionDifficulty(request.Target) {
		return DifficultyAdjustmentResult{}, fmt.Errorf("invalid target difficulty: %q", request.Target)
	}

	maxIterations := request.MaxIterations
	if maxIterations <= 0 {
		maxIterations = DefaultDifficultyRevisionIterations
	}

	currentInput := request.Input
	currentEvaluation, err := a.evaluator.Evaluate(ctx, currentInput)
	if err != nil {
		return DifficultyAdjustmentResult{}, fmt.Errorf("evaluate initial email: %w", err)
	}

	history := []DifficultyAdjustmentStep{{
		Iteration:  0,
		Email:      currentInput.Email,
		Evaluation: currentEvaluation,
	}}

	if status := difficultyStatus(currentEvaluation.Difficulty, request.Target); status != DifficultyAdjustmentNotReached {
		return buildDifficultyAdjustmentResult(currentInput.Email, currentEvaluation, status, 0, history), nil
	}

	var targetRoute *nistTargetRoute
	if route, ok := closestNISTTargetRoute(currentEvaluation, request.Target); ok {
		targetRoute = &route
	}

	acceptedIterations := 0

	for acceptedIterations < maxIterations {
		feedback := buildDifficultyRevisionFeedbackForRoute(
			currentEvaluation,
			request.Target,
			request.UserFeedback,
			targetRoute,
		)

		accepted := false
		for attempt := 1; attempt <= DefaultDifficultyRevisionCandidateAttempts; attempt++ {
			attemptFeedback := feedback
			if attempt > 1 {
				attemptFeedback += "\nThe previous revision candidate was rejected because it did not improve the selected NIST route or violated revision constraints. Produce a different revision that fixes those issues."
			}

			revisionRequest := revisionRequestFromInput(currentInput, request.Target, attemptFeedback)
			revisedEmail, err := a.generator.Revise(ctx, revisionRequest)
			if err != nil {
				var groundingErr *RevisionGroundingError
				if errors.As(err, &groundingErr) {
					// A grounded revision candidate is part of the normal search process.
					// Reject it and let the agent use the remaining candidate budget.
					continue
				}
				return DifficultyAdjustmentResult{}, fmt.Errorf(
					"revise email at iteration %d attempt %d: %w",
					acceptedIterations+1,
					attempt,
					err,
				)
			}

			if err := validateRevisionRequestCandidate(revisionRequest, revisedEmail); err != nil {
				continue
			}

			candidateInput := currentInput
			candidateInput.Email = revisedEmail
			candidateEvaluation, err := a.evaluator.Evaluate(ctx, candidateInput)
			if err != nil {
				return DifficultyAdjustmentResult{}, fmt.Errorf(
					"evaluate revised email at iteration %d attempt %d: %w",
					acceptedIterations+1,
					attempt,
					err,
				)
			}

			status := difficultyStatus(candidateEvaluation.Difficulty, request.Target)
			if status != DifficultyAdjustmentNotReached {
				acceptedIterations++
				history = append(history, DifficultyAdjustmentStep{
					Iteration:  acceptedIterations,
					Email:      revisedEmail,
					Evaluation: candidateEvaluation,
				})
				return buildDifficultyAdjustmentResult(
					revisedEmail,
					candidateEvaluation,
					status,
					acceptedIterations,
					history,
				), nil
			}

			if targetRoute != nil &&
				!revisionCandidateImprovesRoute(currentEvaluation, candidateEvaluation, *targetRoute) {
				continue
			}

			currentInput = candidateInput
			currentEvaluation = candidateEvaluation
			acceptedIterations++
			history = append(history, DifficultyAdjustmentStep{
				Iteration:  acceptedIterations,
				Email:      revisedEmail,
				Evaluation: candidateEvaluation,
			})
			accepted = true
			break
		}

		if !accepted {
			break
		}
	}

	result := buildDifficultyAdjustmentResult(
		currentInput.Email,
		currentEvaluation,
		DifficultyAdjustmentNotReached,
		acceptedIterations,
		history,
	)
	result.ContextChangeSuggestions = buildTargetedContextChangeSuggestions(
		currentEvaluation,
		request.Input.EvaluationContext,
		request.Target,
	)

	return result, nil
}

func difficultyStatus(
	evaluation DifficultyEvaluation,
	target DetectionDifficulty,
) DifficultyAdjustmentStatus {
	if evaluation.Resolved && evaluation.DetectionDifficulty == target {
		return DifficultyAdjustmentReached
	}

	if !evaluation.Resolved && containsDifficulty(evaluation.PossibleDifficulties, target) {
		return DifficultyAdjustmentPossibleUnconfirmed
	}

	return DifficultyAdjustmentNotReached
}

func buildDifficultyAdjustmentResult(
	email Email,
	evaluation EmailEvaluation,
	status DifficultyAdjustmentStatus,
	iterations int,
	history []DifficultyAdjustmentStep,
) DifficultyAdjustmentResult {
	return DifficultyAdjustmentResult{
		Email:      email,
		Evaluation: evaluation,
		Status:     status,
		Iterations: iterations,
		History:    history,
	}
}

func revisionRequestFromInput(input EvaluationInput, target DetectionDifficulty, feedback string) RevisionRequest {
	generation := input.GenerationContext

	return RevisionRequest{
		Email:               input.Email,
		Feedback:            feedback,
		TargetAudience:      generation.TargetAudience,
		RecipientRole:       generation.RecipientRole,
		OrganizationContext: generation.OrganizationContext,
		SenderContext:       generation.SenderContext,
		Scenario:            generation.Scenario,
		CustomScenario:      generation.CustomScenario,
		Language:            generation.Language,
		TargetDifficulty:    string(target),
		EvaluationContext:   &input.EvaluationContext,
	}
}

func buildDifficultyRevisionFeedback(
	evaluation EmailEvaluation,
	target DetectionDifficulty,
	userFeedback string,
) string {
	var route *nistTargetRoute
	if selected, ok := closestNISTTargetRoute(evaluation, target); ok {
		route = &selected
	}

	return buildDifficultyRevisionFeedbackForRoute(
		evaluation,
		target,
		userFeedback,
		route,
	)
}

func buildDifficultyRevisionFeedbackForRoute(
	evaluation EmailEvaluation,
	target DetectionDifficulty,
	userFeedback string,
	route *nistTargetRoute,
) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "Target detection difficulty: %s.\n", target)

	current := DetectionDifficulty("")
	if evaluation.Difficulty.Resolved {
		current = evaluation.Difficulty.DetectionDifficulty
		fmt.Fprintf(
			&builder,
			"Current detection difficulty: %s.\n",
			current,
		)
	} else {
		fmt.Fprintf(
			&builder,
			"Current possible detection difficulties: %s.\n",
			joinDifficulties(evaluation.Difficulty.PossibleDifficulties),
		)
	}

	fmt.Fprintf(
		&builder,
		"Current NIST cue count range: %d..%d (%s..%s).\n",
		evaluation.Cues.MinCount,
		evaluation.Cues.MaxCount,
		evaluation.Cues.MinCategory,
		evaluation.Cues.MaxCategory,
	)
	fmt.Fprintf(
		&builder,
		"Current premise-alignment score range: %d..%d (%s..%s).\n",
		evaluation.PremiseAlignment.MinScore,
		evaluation.PremiseAlignment.MaxScore,
		evaluation.PremiseAlignment.MinCategory,
		evaluation.PremiseAlignment.MaxCategory,
	)

	builder.WriteString("Revise the existing email content toward the target difficulty. Do not regenerate an unrelated email.\n")
	builder.WriteString("Do not change the configured sending profile, expected sender, phishing domain, attachment set, situation/event context, or prior-training context.\n")
	builder.WriteString("Keep the result as a realistic simulated phishing email. Never disclose that it is phishing, a simulation, a training exercise, or a security-awareness test, and never add defensive warnings such as 'do not click' or 'verify the sender' merely to increase detectability.\n")

	if route != nil {
		fmt.Fprintf(
			&builder,
			"Concrete NIST target route: cue category %s (%s) with premise alignment %s (%s).\n",
			route.CueCategory,
			cueCategoryTargetText(route.CueCategory),
			route.PremiseCategory,
			premiseCategoryTargetText(route.PremiseCategory),
		)
		appendRouteGapFeedback(&builder, evaluation, *route)
	}

	direction := difficultyRevisionDirection(evaluation.Difficulty, target)
	switch direction {
	case revisionMakeEasierToDetect:
		builder.WriteString("Direction: make the email EASIER for a recipient to detect as phishing. Increase email-controlled detection cues and/or reduce premise alignment. Do not remove existing detection cues unless the replacement changes add more cues overall. Do not invent facts or alter fixed campaign context.\n")
		builder.WriteString("Useful email-controlled changes may include making wording, presentation, urgency, signer details, branding consistency, or visible link presentation more detectably suspicious when this can be done without changing fixed context.\n")
	case revisionMakeHarderToDetect:
		builder.WriteString("Direction: make the email HARDER for a recipient to detect as phishing. Reduce or remove email-controlled detection cues and preserve or strengthen legitimate premise alignment without inventing facts.\n")
		builder.WriteString("Prioritize correcting the currently detected editable cues listed below.\n")
	default:
		builder.WriteString("Direction cannot be determined exactly from the unresolved current difficulty. Make a conservative change toward the requested target while preserving fixed context.\n")
	}

	if direction == revisionMakeHarderToDetect {
		for _, result := range evaluation.Criteria {
			if !criterionEditableByEmail(result.ID) || result.MaxValue == 0 {
				continue
			}

			fmt.Fprintf(
				&builder,
				"- Detected editable cue %s: %d..%d",
				result.ID,
				result.MinValue,
				result.MaxValue,
			)
			if len(result.Evidence) > 0 {
				fmt.Fprintf(&builder, " (%s)", strings.Join(result.Evidence, "; "))
			}
			builder.WriteString("\n")
		}
	}

	if strings.TrimSpace(userFeedback) != "" {
		fmt.Fprintf(&builder, "User revision request: %s\n", strings.TrimSpace(userFeedback))
	}

	builder.WriteString("Preserve the scenario and supplied facts. Modify only email-controlled content and presentation features.")
	return builder.String()
}

func appendRouteGapFeedback(
	builder *strings.Builder,
	evaluation EmailEvaluation,
	route nistTargetRoute,
) {
	cueGap := cueRouteGap(evaluation.Cues, route.CueCategory)
	premiseGap := premiseRouteGap(evaluation.PremiseAlignment, route.PremiseCategory)

	if cueGap == 0 {
		fmt.Fprintf(
			builder,
			"The cue-category part of the target route is already satisfied. Preserve the current %s cue category while changing other factors.\n",
			route.CueCategory,
		)
	} else {
		fmt.Fprintf(
			builder,
			"The cue-category part of the route is not yet satisfied; the current evaluation is %d cue-count step(s) outside the target range.\n",
			cueGap,
		)
	}

	if premiseGap == 0 {
		fmt.Fprintf(
			builder,
			"The premise-alignment part of the target route is already satisfied. Preserve the current %s premise category.\n",
			route.PremiseCategory,
		)
	} else {
		fmt.Fprintf(
			builder,
			"The premise-alignment part of the route is not yet satisfied; the current score is %d point(s) outside the target range.\n",
			premiseGap,
		)
	}

	if route.PremiseCategory == PremiseAlignmentWeak && premiseGap > 0 {
		builder.WriteString(
			"To lower premise alignment, remove unsupported harmful consequences and reduce unnecessary role-specific, workplace-process, or situational claims where this can be done without changing the fixed scenario or campaign context. Do not strengthen urgency or consequences merely to add cues if doing so raises premise alignment.\n",
		)
	}

	if route.CueCategory == CueCategoryMany && cueGap == 0 {
		builder.WriteString(
			"The email already has enough cues for the Many category. Do not sacrifice that category while trying to lower premise alignment.\n",
		)
	}
}

func revisionCandidateImprovesRoute(
	current EmailEvaluation,
	candidate EmailEvaluation,
	route nistTargetRoute,
) bool {
	currentCueSatisfied := cueRouteGap(current.Cues, route.CueCategory) == 0
	currentPremiseSatisfied := premiseRouteGap(current.PremiseAlignment, route.PremiseCategory) == 0

	candidateCueSatisfied := cueRouteGap(candidate.Cues, route.CueCategory) == 0
	candidatePremiseSatisfied := premiseRouteGap(candidate.PremiseAlignment, route.PremiseCategory) == 0

	if currentCueSatisfied && !candidateCueSatisfied {
		return false
	}
	if currentPremiseSatisfied && !candidatePremiseSatisfied {
		return false
	}

	return nistRouteDistance(candidate, route) < nistRouteDistance(current, route)
}

func nistRouteDistance(
	evaluation EmailEvaluation,
	route nistTargetRoute,
) int {
	unsatisfied := 0

	cueGap := cueRouteGap(evaluation.Cues, route.CueCategory)
	if cueGap > 0 {
		unsatisfied++
	}

	premiseGap := premiseRouteGap(evaluation.PremiseAlignment, route.PremiseCategory)
	if premiseGap > 0 {
		unsatisfied++
	}

	// The large category penalty makes reaching one required NIST dimension
	// more valuable than a small numeric change that leaves both dimensions
	// unsatisfied.
	return unsatisfied*100 + cueGap + premiseGap
}

func cueRouteGap(evaluation CueEvaluation, target CueCategory) int {
	targetMin, targetMax, ok := cueCategoryBounds(target)
	if !ok {
		return 1 << 20
	}
	return intervalDistance(evaluation.MinCount, evaluation.MaxCount, targetMin, targetMax)
}

func premiseRouteGap(
	evaluation PremiseAlignmentEvaluation,
	target PremiseAlignmentCategory,
) int {
	targetMin, targetMax, ok := premiseCategoryBounds(target)
	if !ok {
		return 1 << 20
	}
	return intervalDistance(evaluation.MinScore, evaluation.MaxScore, targetMin, targetMax)
}

func cueCategoryBounds(category CueCategory) (int, int, bool) {
	switch category {
	case CueCategoryFew:
		return 1, 8, true
	case CueCategorySome:
		return 9, 14, true
	case CueCategoryMany:
		return 15, 1 << 20, true
	default:
		return 0, 0, false
	}
}

func premiseCategoryBounds(category PremiseAlignmentCategory) (int, int, bool) {
	switch category {
	case PremiseAlignmentWeak:
		return -8, 10, true
	case PremiseAlignmentMedium:
		return 11, 17, true
	case PremiseAlignmentStrong:
		return 18, 32, true
	default:
		return 0, 0, false
	}
}

func intervalDistance(
	actualMin int,
	actualMax int,
	targetMin int,
	targetMax int,
) int {
	if actualMax < targetMin {
		return targetMin - actualMax
	}
	if actualMin > targetMax {
		return actualMin - targetMax
	}
	return 0
}

func validateInitialGenerationCandidate(
	email Email,
	evaluationContext EvaluationContext,
) error {
	if err := validateEmailHTMLContent(email.HTML); err != nil {
		return err
	}
	content := strings.ToLower(strings.Join(
		[]string{email.Subject, email.Text, email.HTML},
		"\n",
	))

	for _, value := range evaluatorOnlyProtectedValues(evaluationContext) {
		if value != "" && strings.Contains(content, value) {
			return fmt.Errorf("generated email exposed evaluator-only expected identity or domain %q", value)
		}
	}

	return nil
}

func evaluatorOnlyProtectedValues(evaluationContext EvaluationContext) []string {
	protected := make([]string, 0, 4)
	simulatedEmail := ""
	simulatedDomain := ""
	if evaluationContext.SimulatedSender != nil {
		simulatedEmail = strings.TrimSpace(strings.ToLower(evaluationContext.SimulatedSender.Email))
		if domain, err := mailboxDomain(simulatedEmail); err == nil {
			simulatedDomain = domain
		}
	}

	if evaluationContext.ExpectedSender != nil {
		expectedEmail := strings.TrimSpace(strings.ToLower(evaluationContext.ExpectedSender.Email))
		if expectedEmail != "" && expectedEmail != simulatedEmail {
			protected = append(protected, expectedEmail)
		}
		if expectedDomain, err := mailboxDomain(expectedEmail); err == nil &&
			expectedDomain != "" && expectedDomain != simulatedDomain {
			protected = append(protected, expectedDomain)
		}
	}

	expectedLinkDomain := strings.TrimSpace(strings.ToLower(evaluationContext.Link.ExpectedDomain))
	if expectedLinkDomain != "" {
		simulatedLinkDomain := ""
		if domain, err := normalizedURLHost(evaluationContext.Link.SimulatedURL); err == nil {
			simulatedLinkDomain = domain
		}
		if normalizedExpected, err := normalizedDomain(expectedLinkDomain); err == nil &&
			normalizedExpected != "" && normalizedExpected != simulatedLinkDomain &&
			normalizedExpected != simulatedDomain {
			protected = append(protected, normalizedExpected)
		}
	}

	return uniqueStrings(protected)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func validateRevisionCandidate(
	original Email,
	revised Email,
	evaluationContext EvaluationContext,
) error {
	originalContent := strings.ToLower(strings.Join(
		[]string{original.Subject, original.Text, original.HTML},
		"\n",
	))
	revisedContent := strings.ToLower(strings.Join(
		[]string{revised.Subject, revised.Text, revised.HTML},
		"\n",
	))

	for _, phrase := range []string{
		"phishing simulation",
		"phishing exercise",
		"training exercise",
		"security-awareness",
		"security awareness",
		"this is phishing",
		"do not click",
		"verify the sender",
		"unverified domain",
	} {
		if !strings.Contains(originalContent, phrase) && strings.Contains(revisedContent, phrase) {
			return fmt.Errorf("revision introduced simulation or defensive disclosure %q", phrase)
		}
	}

	for _, value := range evaluatorOnlyProtectedValues(evaluationContext) {
		if value == "" {
			continue
		}
		if !strings.Contains(originalContent, value) && strings.Contains(revisedContent, value) {
			return fmt.Errorf("revision exposed evaluator-only expected identity or domain %q", value)
		}
	}

	originalURLs := absoluteURLSet(originalContent)
	for revisedURL := range absoluteURLSet(revisedContent) {
		if _, existed := originalURLs[revisedURL]; !existed {
			return fmt.Errorf("revision introduced new hard-coded external URL %q; use GoPhish template variables such as {{.URL}} for campaign links", revisedURL)
		}
	}

	return nil
}

func validateRevisionRequestCandidate(
	request RevisionRequest,
	revised Email,
) error {
	if err := validateEmailHTMLContent(revised.HTML); err != nil {
		return err
	}
	evaluationContext := EvaluationContext{}
	if request.EvaluationContext != nil {
		evaluationContext = *request.EvaluationContext
	}

	if err := validateRevisionCandidate(request.Email, revised, evaluationContext); err != nil {
		return err
	}

	allowedContext := strings.Join([]string{
		request.TargetAudience,
		request.RecipientRole,
		request.OrganizationContext,
		request.SenderContext,
		request.Scenario,
		request.CustomScenario,
		request.Feedback,
	}, "\n")

	if err := validateNewEmailAddresses(
		request.Email,
		revised,
		evaluationContext,
		allowedContext,
	); err != nil {
		return err
	}

	return validateNewPhoneNumbers(request.Email, revised, allowedContext)
}

func validateNewEmailAddresses(
	original Email,
	revised Email,
	evaluationContext EvaluationContext,
	allowedContext string,
) error {
	allowed := emailAddressSet(strings.Join([]string{
		original.Subject,
		original.Text,
		original.HTML,
		allowedContext,
	}, "\n"))

	if evaluationContext.SimulatedSender != nil {
		for address := range emailAddressSet(evaluationContext.SimulatedSender.Email) {
			allowed[address] = struct{}{}
		}
	}

	revisedAddresses := emailAddressSet(strings.Join([]string{
		revised.Subject,
		revised.Text,
		revised.HTML,
	}, "\n"))

	for address := range revisedAddresses {
		if _, ok := allowed[address]; ok {
			continue
		}
		return fmt.Errorf(
			"revision introduced unsupported email address %q; use only addresses already present in the email or supplied user/simulated-sender context",
			address,
		)
	}

	return nil
}

func validateNewPhoneNumbers(
	original Email,
	revised Email,
	allowedContext string,
) error {
	allowed := phoneNumberSet(strings.Join([]string{
		original.Subject,
		original.Text,
		original.HTML,
		allowedContext,
	}, "\n"))

	revisedNumbers := phoneNumberSet(strings.Join([]string{
		revised.Subject,
		revised.Text,
		revised.HTML,
	}, "\n"))

	for number := range revisedNumbers {
		if _, ok := allowed[number]; ok {
			continue
		}
		return fmt.Errorf(
			"revision introduced unsupported phone number %q; use only phone numbers already present in the email or supplied user context",
			number,
		)
	}

	return nil
}

func phoneNumberSet(content string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, match := range possiblePhoneNumberPattern.FindAllString(content, -1) {
		var digits strings.Builder
		for _, r := range match {
			if r >= '0' && r <= '9' {
				digits.WriteRune(r)
			}
		}
		normalized := digits.String()
		// Keep this deliberately conservative: full phone numbers are usually
		// 10-15 digits. Shorter numeric strings are more likely dates, IDs, or
		// extensions and remain prompt-grounded rather than hard-blocked.
		if len(normalized) < 10 || len(normalized) > 15 {
			continue
		}
		result[normalized] = struct{}{}
	}
	return result
}

func emailAddressSet(content string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, match := range emailAddressPattern.FindAllString(content, -1) {
		normalized := strings.ToLower(strings.TrimSpace(match))
		if normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}

func absoluteURLSet(content string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, match := range absoluteURLPattern.FindAllString(content, -1) {
		normalized := strings.TrimRight(strings.ToLower(match), ".,;:!?)]}")
		if normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}

func emailDomain(address string) string {
	at := strings.LastIndex(address, "@")
	if at < 0 || at == len(address)-1 {
		return ""
	}
	return strings.TrimSpace(address[at+1:])
}

type revisionDirection int

const (
	revisionDirectionUnknown revisionDirection = iota
	revisionMakeHarderToDetect
	revisionMakeEasierToDetect
)

func difficultyRevisionDirection(
	evaluation DifficultyEvaluation,
	target DetectionDifficulty,
) revisionDirection {
	if !evaluation.Resolved {
		return revisionDirectionUnknown
	}

	currentRank, ok := detectionDifficultyRank(evaluation.DetectionDifficulty)
	if !ok {
		return revisionDirectionUnknown
	}
	targetRank, ok := detectionDifficultyRank(target)
	if !ok || currentRank == targetRank {
		return revisionDirectionUnknown
	}

	// Lower rank means harder to detect. Moving to a larger rank therefore
	// means making the phishing email easier for the recipient to detect.
	if targetRank > currentRank {
		return revisionMakeEasierToDetect
	}
	return revisionMakeHarderToDetect
}

func detectionDifficultyRank(value DetectionDifficulty) (int, bool) {
	switch value {
	case DifficultyVeryDifficult:
		return 0, true
	case DifficultyModeratelyDifficult:
		return 1, true
	case DifficultyModeratelyToLeastDifficult:
		return 2, true
	case DifficultyLeastDifficult:
		return 3, true
	default:
		return 0, false
	}
}

type nistTargetRoute struct {
	CueCategory     CueCategory
	PremiseCategory PremiseAlignmentCategory
}

func closestNISTTargetRoute(
	evaluation EmailEvaluation,
	target DetectionDifficulty,
) (nistTargetRoute, bool) {
	if !evaluation.Cues.CategoryResolved || !evaluation.PremiseAlignment.CategoryResolved {
		return nistTargetRoute{}, false
	}

	candidates := nistTargetRoutes(target)
	if len(candidates) == 0 {
		return nistTargetRoute{}, false
	}

	currentCueRank, cueOK := cueCategoryRank(evaluation.Cues.Category)
	currentPremiseRank, premiseOK := premiseCategoryRank(evaluation.PremiseAlignment.Category)
	if !cueOK || !premiseOK {
		return nistTargetRoute{}, false
	}

	best := candidates[0]
	bestDistance := 1 << 30
	bestGap := 1 << 30
	// Partial or synthetic evaluations may not carry numeric ranges. Preserve
	// category-only ordering for those inputs.
	hasNumericRanges := evaluation.Cues.MinCount >= CueCountFewMin &&
		evaluation.Cues.MaxCount >= evaluation.Cues.MinCount &&
		evaluation.PremiseAlignment.MaxScore >= evaluation.PremiseAlignment.MinScore &&
		evaluation.PremiseAlignment.MinScore >= -8

	for _, candidate := range candidates {
		candidateCueRank, cueOK := cueCategoryRank(candidate.CueCategory)
		candidatePremiseRank, premiseOK := premiseCategoryRank(candidate.PremiseCategory)
		if !cueOK || !premiseOK {
			continue
		}

		distance := absInt(currentCueRank-candidateCueRank) +
			absInt(currentPremiseRank-candidatePremiseRank)
		gap := 0
		if hasNumericRanges {
			// Prefer edits to email-controlled cues over large changes to
			// premise alignment, which also depends on fixed campaign context.
			gap = cueRouteGap(evaluation.Cues, candidate.CueCategory) +
				2*premiseRouteGap(evaluation.PremiseAlignment, candidate.PremiseCategory)
		}
		if distance < bestDistance || (distance == bestDistance && gap < bestGap) {
			best = candidate
			bestDistance = distance
			bestGap = gap
		}
	}

	return best, true
}

func nistTargetRoutes(target DetectionDifficulty) []nistTargetRoute {
	switch target {
	case DifficultyVeryDifficult:
		return []nistTargetRoute{
			{CueCategory: CueCategoryFew, PremiseCategory: PremiseAlignmentStrong},
			{CueCategory: CueCategoryFew, PremiseCategory: PremiseAlignmentMedium},
			{CueCategory: CueCategorySome, PremiseCategory: PremiseAlignmentStrong},
		}
	case DifficultyModeratelyDifficult:
		return []nistTargetRoute{
			{CueCategory: CueCategoryFew, PremiseCategory: PremiseAlignmentWeak},
			{CueCategory: CueCategorySome, PremiseCategory: PremiseAlignmentMedium},
			{CueCategory: CueCategoryMany, PremiseCategory: PremiseAlignmentStrong},
			{CueCategory: CueCategoryMany, PremiseCategory: PremiseAlignmentMedium},
		}
	case DifficultyModeratelyToLeastDifficult:
		return []nistTargetRoute{
			{CueCategory: CueCategorySome, PremiseCategory: PremiseAlignmentWeak},
		}
	case DifficultyLeastDifficult:
		return []nistTargetRoute{
			{CueCategory: CueCategoryMany, PremiseCategory: PremiseAlignmentWeak},
		}
	default:
		return nil
	}
}

func cueCategoryTargetText(category CueCategory) string {
	switch category {
	case CueCategoryFew:
		return "1-8 total cues"
	case CueCategorySome:
		return "9-14 total cues"
	case CueCategoryMany:
		return "at least 15 total cues"
	default:
		return "unknown cue range"
	}
}

func premiseCategoryTargetText(category PremiseAlignmentCategory) string {
	switch category {
	case PremiseAlignmentWeak:
		return "score <= 10"
	case PremiseAlignmentMedium:
		return "score 11-17"
	case PremiseAlignmentStrong:
		return "score >= 18"
	default:
		return "unknown score range"
	}
}

func cueCategoryRank(category CueCategory) (int, bool) {
	switch category {
	case CueCategoryFew:
		return 0, true
	case CueCategorySome:
		return 1, true
	case CueCategoryMany:
		return 2, true
	default:
		return 0, false
	}
}

func premiseCategoryRank(category PremiseAlignmentCategory) (int, bool) {
	switch category {
	case PremiseAlignmentWeak:
		return 0, true
	case PremiseAlignmentMedium:
		return 1, true
	case PremiseAlignmentStrong:
		return 2, true
	default:
		return 0, false
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func criterionEditableByEmail(id CueCriterionID) bool {
	switch id {
	case CriterionSenderNameMismatch,
		CriterionSenderDomainSpoofing,
		CriterionAttachments,
		CriterionSpoofedLinkDomains:
		return false
	default:
		return true
	}
}

func buildTargetedContextChangeSuggestions(
	evaluation EmailEvaluation,
	context EvaluationContext,
	target DetectionDifficulty,
) []string {
	suggestions := buildContextChangeSuggestions(evaluation, context)

	route, ok := closestNISTTargetRoute(evaluation, target)
	if !ok {
		return suggestions
	}

	if premiseRouteGap(evaluation.PremiseAlignment, route.PremiseCategory) > 0 {
		suggestions = append(
			suggestions,
			"Premise alignment is outside the selected NIST target route. Review the target audience, recipient role, organization context, or situation/event context if changing the email content alone cannot reach the requested premise-alignment category.",
		)
	}

	if cueRouteGap(evaluation.Cues, route.CueCategory) > 0 {
		suggestions = append(
			suggestions,
			"The cue category is outside the selected NIST target route. Review fixed sender, domain, link, or attachment choices if content-only revision cannot reach the required cue range.",
		)
	}

	return suggestions
}

func buildContextChangeSuggestions(
	evaluation EmailEvaluation,
	context EvaluationContext,
) []string {
	suggestions := make([]string, 0, 4)

	if criterionMayContribute(evaluation.Criteria, CriterionSenderNameMismatch) ||
		criterionMayContribute(evaluation.Criteria, CriterionSenderDomainSpoofing) {
		suggestions = append(suggestions, "Review the selected sending profile and expected legitimate sender.")
	}

	if criterionMayContribute(evaluation.Criteria, CriterionSpoofedLinkDomains) {
		suggestions = append(suggestions, "Review the simulated phishing URL and expected legitimate domain.")
	}

	if criterionMayContribute(evaluation.Criteria, CriterionAttachments) {
		suggestions = append(suggestions, "Remove or replace attachments if the campaign scenario permits it.")
	}

	if context.PriorTrainingExposure != "" && context.PriorTrainingExposure != TrainingExposureUnknown {
		suggestions = append(suggestions, "The audience's prior phishing exposure is fixed context; consider a different target difficulty or audience if needed.")
	}

	return suggestions
}

func criterionMayContribute(results []CueCriterionResult, id CueCriterionID) bool {
	for _, result := range results {
		if result.ID == id {
			return result.MaxValue > 0
		}
	}
	return false
}

func containsDifficulty(values []DetectionDifficulty, target DetectionDifficulty) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func joinDifficulties(values []DetectionDifficulty) string {
	if len(values) == 0 {
		return "unclassified"
	}

	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, string(value))
	}
	return strings.Join(parts, ", ")
}

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const premiseAlignmentMaxAttempts = 2

type premiseModelResult struct {
	ID          PremiseAlignmentElementID `json:"id"`
	Resolved    bool                      `json:"resolved"`
	Score       int                       `json:"score"`
	Explanation string                    `json:"explanation"`
}

type premiseModelResponse struct {
	Results []premiseModelResult `json:"results"`
}

type PremiseAlignmentEvaluator struct {
	client *Client
}

func NewPremiseAlignmentEvaluator(client *Client) *PremiseAlignmentEvaluator {
	return &PremiseAlignmentEvaluator{client: client}
}

func (e *PremiseAlignmentEvaluator) Evaluate(
	ctx context.Context,
	input EvaluationInput,
) (PremiseAlignmentEvaluation, error) {
	messages := []Message{
		{Role: "system", Content: premiseAlignmentSystemPrompt},
		{Role: "user", Content: buildPremiseAlignmentPrompt(input)},
	}

	var semanticResults []PremiseAlignmentElementResult
	var lastParseErr error

	for attempt := 1; attempt <= premiseAlignmentMaxAttempts; attempt++ {
		response, err := e.client.ChatWithThinking(
			ctx,
			messages,
			premiseAlignmentResponseSchema,
			evaluatorModelOptions(),
			evaluatorThinkLevel,
		)
		if err != nil {
			return PremiseAlignmentEvaluation{}, fmt.Errorf("evaluate premise alignment: %w", err)
		}

		semanticResults, err = parsePremiseAlignmentResponse(response)
		if err == nil {
			lastParseErr = nil
			break
		}

		lastParseErr = err
		if attempt == premiseAlignmentMaxAttempts {
			break
		}

		messages = append(
			messages,
			Message{Role: "assistant", Content: response},
			Message{
				Role: "user",
				Content: fmt.Sprintf(
					"Your previous JSON failed application validation: %s\nReturn a corrected complete JSON object with exactly the four requested elements. Every element must contain a non-empty explanation. If resolved is false, the explanation must state which context is missing or insufficient.",
					err,
				),
			},
		)
	}

	if lastParseErr != nil {
		return PremiseAlignmentEvaluation{}, fmt.Errorf(
			"parse premise alignment after repair attempt: %w",
			lastParseErr,
		)
	}

	semanticResults = applyPremiseContextRules(semanticResults, input)

	priorExposureScore, err := TrainingExposureScore(input.EvaluationContext.PriorTrainingExposure)
	if err != nil {
		return PremiseAlignmentEvaluation{}, err
	}

	semanticResults = append(semanticResults, PremiseAlignmentElementResult{
		ID:          PremiseElementPriorExposure,
		Score:       priorExposureScore,
		Explanation: priorExposureExplanation(input.EvaluationContext.PriorTrainingExposure),
	})

	evaluation, err := BuildPremiseAlignmentEvaluation(semanticResults)
	if err != nil {
		return PremiseAlignmentEvaluation{}, fmt.Errorf("build premise alignment evaluation: %w", err)
	}

	return evaluation, nil
}

func applyPremiseContextRules(
	results []PremiseAlignmentElementResult,
	input EvaluationInput,
) []PremiseAlignmentElementResult {
	situationProvided := strings.TrimSpace(input.EvaluationContext.SituationContext) != ""

	for i := range results {
		switch results[i].ID {
		case PremiseElementSituationalAlignment:
			if !situationProvided {
				// NIST element 3 is alignment with another situation or event. A
				// generation scenario label describes what the email is about, but it is
				// not independent evidence that such a situation/event exists.
				results[i].Score = nil
				results[i].Explanation = "No concrete situation or event context was supplied; situational alignment is unresolved."
				continue
			}

			if results[i].Score == nil || premiseExplanationShowsExplicitMismatch(results[i].Explanation) {
				// Once explicit situation/event context is available, lack of alignment
				// or an explicit contradiction is a resolvable result. This also guards
				// against self-contradictory model outputs such as score=2 together with
				// an explanation that says no matching event is expected.
				score := 0
				results[i].Score = &score
				if strings.TrimSpace(results[i].Explanation) == "" {
					results[i].Explanation = "Situation/event context was supplied, but no supported situational alignment could be established; scored as not applicable."
				}
			}

		case PremiseElementConsequences:
			if results[i].Score == nil && premiseExplanationShowsNoConsequences(results[i].Explanation) {
				// Explicit absence of a harmful consequence is evidence for score 0, not
				// missing context. Keep Unknown only for genuinely unavailable or
				// ambiguous information.
				score := 0
				results[i].Score = &score
			}
		}
	}

	return results
}

func premiseExplanationShowsExplicitMismatch(explanation string) bool {
	value := strings.ToLower(strings.TrimSpace(explanation))
	if value == "" {
		return false
	}

	patterns := []string{
		"does not align",
		"doesn't align",
		"not align",
		"contradict",
		"not expected",
		"no matching event",
		"no supported situational alignment",
	}
	for _, pattern := range patterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

func premiseExplanationShowsNoConsequences(explanation string) bool {
	value := strings.ToLower(strings.TrimSpace(explanation))
	if value == "" {
		return false
	}

	absencePatterns := []string{
		"no harmful consequence",
		"no harmful ramification",
		"no explicit consequence",
		"no explicit or clearly implied negative consequence",
		"does not mention any harmful",
		"does not state any harmful",
		"does not explicitly state any harmful",
		"no consequence is present",
		"no consequences are present",
	}
	hasAbsence := false
	for _, pattern := range absencePatterns {
		if strings.Contains(value, pattern) {
			hasAbsence = true
			break
		}
	}
	if !hasAbsence {
		return false
	}

	// Do not clamp a genuinely implied harmful outcome to zero merely because
	// the explanation first notes that the consequence is not stated verbatim.
	positivePatterns := []string{
		"clearly implied",
		"implies",
		"implied consequence",
		"account lock",
		"account block",
		"account disable",
		"access loss",
		"loss of access",
		"service disruption",
		"payroll delay",
		"financial penalty",
	}
	for _, pattern := range positivePatterns {
		if strings.Contains(value, pattern) {
			return false
		}
	}

	return true
}

func parsePremiseAlignmentResponse(response string) ([]PremiseAlignmentElementResult, error) {
	var modelResponse premiseModelResponse
	if err := json.Unmarshal([]byte(response), &modelResponse); err != nil {
		return nil, fmt.Errorf("invalid JSON returned by model: %w", err)
	}

	if len(modelResponse.Results) != len(semanticPremiseElementIDs) {
		return nil, fmt.Errorf(
			"expected %d premise alignment results, got %d",
			len(semanticPremiseElementIDs),
			len(modelResponse.Results),
		)
	}

	byID := make(map[PremiseAlignmentElementID]PremiseAlignmentElementResult, len(modelResponse.Results))

	for _, modelResult := range modelResponse.Results {
		if !isSemanticPremiseElementID(modelResult.ID) {
			return nil, fmt.Errorf("unexpected premise alignment element: %q", modelResult.ID)
		}
		if _, exists := byID[modelResult.ID]; exists {
			return nil, fmt.Errorf("duplicate premise alignment element: %q", modelResult.ID)
		}
		if !validPremiseAlignmentScore(modelResult.Score) {
			return nil, fmt.Errorf("invalid score %d for premise alignment element %q", modelResult.Score, modelResult.ID)
		}

		explanation := strings.TrimSpace(modelResult.Explanation)
		if explanation == "" {
			return nil, fmt.Errorf("premise alignment element %q has no explanation", modelResult.ID)
		}

		var score *int
		if modelResult.Resolved {
			value := modelResult.Score
			score = &value
		}

		byID[modelResult.ID] = PremiseAlignmentElementResult{
			ID:          modelResult.ID,
			Score:       score,
			Explanation: explanation,
		}
	}

	results := make([]PremiseAlignmentElementResult, 0, len(semanticPremiseElementIDs))
	for _, id := range semanticPremiseElementIDs {
		result, exists := byID[id]
		if !exists {
			return nil, fmt.Errorf("missing premise alignment element: %q", id)
		}
		results = append(results, result)
	}

	return results, nil
}

func priorExposureExplanation(exposure TrainingExposure) string {
	if exposure == "" || exposure == TrainingExposureUnknown {
		return "Relevance of prior phishing training or warnings to this scenario is unknown."
	}
	return fmt.Sprintf("User-rated relevance of prior phishing training or warnings to this scenario: %s.", exposure)
}

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
		response, err := e.client.ChatWithOptions(
			ctx,
			messages,
			premiseAlignmentResponseSchema,
			evaluatorModelOptions(),
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
	if strings.TrimSpace(input.EvaluationContext.SituationContext) != "" {
		return results
	}

	for i := range results {
		if results[i].ID != PremiseElementSituationalAlignment {
			continue
		}

		// NIST element 3 is alignment with another situation or event. A
		// generation scenario label describes what the email is about, but it is
		// not independent evidence that such a situation/event exists.
		results[i].Score = nil
		results[i].Explanation = "No concrete situation or event context was supplied; situational alignment is unresolved."
		break
	}

	return results
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
		return "Prior phishing training or exposure is unknown."
	}
	return fmt.Sprintf("Prior phishing training or exposure: %s.", exposure)
}

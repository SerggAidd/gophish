package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const semanticCueMaxAttempts = 2

type semanticCueModelResult struct {
	MinValue int      `json:"min_value"`
	MaxValue int      `json:"max_value"`
	Evidence []string `json:"evidence"`
}

type semanticCueModelResponse struct {
	Results map[CueCriterionID]semanticCueModelResult `json:"results"`
}

type SemanticCueEvaluator struct {
	client *Client
}

func NewSemanticCueEvaluator(client *Client) *SemanticCueEvaluator {
	return &SemanticCueEvaluator{client: client}
}

func (e *SemanticCueEvaluator) Evaluate(
	ctx context.Context,
	input EvaluationInput,
) ([]CueCriterionResult, error) {
	messages := []Message{
		{Role: "system", Content: semanticCueSystemPrompt},
		{Role: "user", Content: buildSemanticCuePrompt(input)},
	}

	var lastParseErr error

	for attempt := 1; attempt <= semanticCueMaxAttempts; attempt++ {
		response, err := e.client.ChatWithOptions(
			ctx,
			messages,
			semanticCueResponseSchema,
			evaluatorModelOptions(),
		)
		if err != nil {
			return nil, fmt.Errorf("evaluate semantic cues: %w", err)
		}

		results, err := parseSemanticCueResponse(response)
		if err == nil {
			return results, nil
		}

		lastParseErr = err
		if attempt == semanticCueMaxAttempts {
			break
		}

		messages = append(
			messages,
			Message{Role: "assistant", Content: response},
			Message{
				Role: "user",
				Content: fmt.Sprintf(
					"Your previous JSON failed application validation: %s\nReturn a corrected complete JSON object. Keep every required criterion key exactly once. Every exact positive finding must include at least one non-empty evidence string. For unresolved ranges, include evidence describing what context is missing when possible. Exact 0..0 results must use an empty evidence array.",
					err,
				),
			},
		)
	}

	return nil, fmt.Errorf("parse semantic cue evaluation after repair attempt: %w", lastParseErr)
}

func parseSemanticCueResponse(response string) ([]CueCriterionResult, error) {
	var modelResponse semanticCueModelResponse

	if err := json.Unmarshal([]byte(response), &modelResponse); err != nil {
		return nil, fmt.Errorf("invalid JSON returned by model: %w", err)
	}

	if len(modelResponse.Results) != len(semanticCueCriterionIDs) {
		return nil, fmt.Errorf(
			"expected %d semantic cue results, got %d",
			len(semanticCueCriterionIDs),
			len(modelResponse.Results),
		)
	}

	for id := range modelResponse.Results {
		if !isSemanticCueCriterionID(id) {
			return nil, fmt.Errorf("unexpected semantic cue criterion: %q", id)
		}
	}

	results := make([]CueCriterionResult, 0, len(semanticCueCriterionIDs))
	for _, id := range semanticCueCriterionIDs {
		modelResult, exists := modelResponse.Results[id]
		if !exists {
			return nil, fmt.Errorf("missing semantic cue criterion: %q", id)
		}

		evidence := cleanSemanticEvidence(modelResult.Evidence)
		unresolved := modelResult.MinValue != modelResult.MaxValue
		exactPositive := !unresolved && modelResult.MaxValue > 0

		if exactPositive && len(evidence) == 0 {
			return nil, fmt.Errorf(
				"criterion %q has exact positive value %d but no evidence",
				id,
				modelResult.MaxValue,
			)
		}

		// Unknown must never be converted to zero. If Ollama returns a valid
		// unresolved range but omits explanatory evidence, preserve the range
		// and add a neutral fallback instead of failing the entire evaluation.
		if unresolved && len(evidence) == 0 {
			evidence = []string{fmt.Sprintf(
				"Available email or campaign context is insufficient to resolve criterion %s.",
				id,
			)}
		}

		if modelResult.MinValue == 0 && modelResult.MaxValue == 0 {
			evidence = nil
		}

		source := CueSourceLLM
		if isHybridDomainCriterion(id) {
			source = CueSourceHybrid
		}

		results = append(results, CueCriterionResult{
			ID:       id,
			MinValue: modelResult.MinValue,
			MaxValue: modelResult.MaxValue,
			Source:   source,
			Evidence: evidence,
		})
	}

	if _, err := BuildCueResults(results); err != nil {
		return nil, fmt.Errorf("validate semantic cue results: %w", err)
	}

	return results, nil
}

func cleanSemanticEvidence(values []string) []string {
	evidence := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		evidence = append(evidence, value)
	}
	return evidence
}

func isHybridDomainCriterion(id CueCriterionID) bool {
	switch id {
	case CriterionSenderDomainSpoofing, CriterionSpoofedLinkDomains:
		return true
	default:
		return false
	}
}

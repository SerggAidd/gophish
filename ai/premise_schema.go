package ai

var semanticPremiseElementIDs = []PremiseAlignmentElementID{
	PremiseElementWorkplaceProcess,
	PremiseElementWorkplaceRelevance,
	PremiseElementSituationalAlignment,
	PremiseElementConsequences,
}

var premiseAlignmentResponseSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"results": map[string]interface{}{
			"type":     "array",
			"minItems": len(semanticPremiseElementIDs),
			"maxItems": len(semanticPremiseElementIDs),
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type": "string",
						"enum": premiseElementStrings(),
					},
					"resolved": map[string]interface{}{
						"type": "boolean",
					},
					"score": map[string]interface{}{
						"type": "integer",
						"enum": []int{0, 2, 4, 6, 8},
					},
					"explanation": map[string]interface{}{
						"type":      "string",
						"minLength": 1,
					},
				},
				"required":             []string{"id", "resolved", "score", "explanation"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"results"},
	"additionalProperties": false,
}

func premiseElementStrings() []string {
	values := make([]string, 0, len(semanticPremiseElementIDs))
	for _, id := range semanticPremiseElementIDs {
		values = append(values, string(id))
	}
	return values
}

func isSemanticPremiseElementID(id PremiseAlignmentElementID) bool {
	for _, expected := range semanticPremiseElementIDs {
		if id == expected {
			return true
		}
	}
	return false
}

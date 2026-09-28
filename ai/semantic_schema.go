package ai

var semanticCueCriterionIDs = []CueCriterionID{
	CriterionSenderNameMismatch,
	CriterionSenderDomainSpoofing,
	CriterionSpoofedLinkDomains,
	CriterionMissingBranding,
	CriterionUnprofessionalDesign,
	CriterionMissingSignerDetails,
	CriterionMimicsBusinessProcess,
	CriterionPosesAsAuthority,
	CriterionSpellingErrors,
	CriterionGrammarErrors,
	CriterionInconsistencies,
	CriterionImitatedBranding,
	CriterionOutdatedBranding,
	CriterionSecurityIndicators,
	CriterionLegalLanguage,
	CriterionDistractingDetails,
	CriterionSensitiveInfoRequests,
	CriterionTimePressure,
	CriterionThreats,
	CriterionHumanitarianAppeals,
	CriterionTooGoodToBeTrueOffers,
	CriterionPersonalizedUnexpectedOffer,
	CriterionLimitedTimeOffers,
}

var semanticCueResponseSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"results": map[string]interface{}{
			"type":                 "object",
			"properties":           semanticCueResultProperties(),
			"required":             semanticCueCriterionStrings(),
			"additionalProperties": false,
		},
	},
	"required":             []string{"results"},
	"additionalProperties": false,
}

func semanticCueResultProperties() map[string]interface{} {
	properties := make(map[string]interface{}, len(semanticCueCriterionIDs))
	for _, id := range semanticCueCriterionIDs {
		properties[string(id)] = map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"min_value": map[string]interface{}{
					"type":    "integer",
					"minimum": 0,
				},
				"max_value": map[string]interface{}{
					"type":    "integer",
					"minimum": 0,
				},
				"evidence": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type":      "string",
						"minLength": 1,
					},
				},
			},
			"required":             []string{"min_value", "max_value", "evidence"},
			"additionalProperties": false,
		}
	}
	return properties
}

func semanticCueCriterionStrings() []string {
	values := make([]string, 0, len(semanticCueCriterionIDs))

	for _, id := range semanticCueCriterionIDs {
		values = append(values, string(id))
	}

	return values
}

func isSemanticCueCriterionID(id CueCriterionID) bool {
	for _, expected := range semanticCueCriterionIDs {
		if id == expected {
			return true
		}
	}

	return false
}

package ai

import "fmt"

type CueCriterionID string

type CueCriterionKind string

const (
	CueCriterionBinary  CueCriterionKind = "binary"
	CueCriterionCounted CueCriterionKind = "counted"
)

type CueCriterionDefinition struct {
	ID    CueCriterionID   `json:"id"`
	CueID CueID            `json:"cue_id"`
	Name  string           `json:"name"`
	Kind  CueCriterionKind `json:"kind"`
}

type CueCriterionResult struct {
	ID       CueCriterionID     `json:"id"`
	Value    int                `json:"value"`
	Source   CueDetectionSource `json:"source"`
	Evidence []string           `json:"evidence,omitempty"`
}

const (
	CriterionSenderNameMismatch   CueCriterionID = "sender_name_address_mismatch"
	CriterionSenderDomainSpoofing CueCriterionID = "sender_domain_spoofing"

	CriterionMissingBranding      CueCriterionID = "missing_branding"
	CriterionUnprofessionalDesign CueCriterionID = "unprofessional_design"

	CriterionMissingGreeting        CueCriterionID = "missing_greeting"
	CriterionMissingPersonalization CueCriterionID = "missing_personalization"
	CriterionMissingSignerDetails   CueCriterionID = "missing_signer_details"

	CriterionMimicsBusinessProcess CueCriterionID = "mimics_business_process"
	CriterionPosesAsAuthority      CueCriterionID = "poses_as_authority"

	CriterionSpellingErrors  CueCriterionID = "spelling_errors"
	CriterionGrammarErrors   CueCriterionID = "grammar_errors"
	CriterionInconsistencies CueCriterionID = "inconsistencies"

	CriterionAttachments        CueCriterionID = "attachments"
	CriterionHiddenURLLinks     CueCriterionID = "hidden_url_links"
	CriterionSpoofedLinkDomains CueCriterionID = "spoofed_link_domains"

	CriterionImitatedBranding   CueCriterionID = "imitated_branding"
	CriterionOutdatedBranding   CueCriterionID = "outdated_branding"
	CriterionSecurityIndicators CueCriterionID = "security_indicators"

	CriterionLegalLanguage         CueCriterionID = "legal_language"
	CriterionDistractingDetails    CueCriterionID = "distracting_details"
	CriterionSensitiveInfoRequests CueCriterionID = "sensitive_information_requests"
	CriterionTimePressure          CueCriterionID = "time_pressure"
	CriterionThreats               CueCriterionID = "threats"

	CriterionHumanitarianAppeals         CueCriterionID = "humanitarian_appeals"
	CriterionTooGoodToBeTrueOffers       CueCriterionID = "too_good_to_be_true_offers"
	CriterionPersonalizedUnexpectedOffer CueCriterionID = "personalized_unexpected_offer"
	CriterionLimitedTimeOffers           CueCriterionID = "limited_time_offers"
)

var NISTCueCriteria = []CueCriterionDefinition{
	{
		ID:    CriterionSenderNameMismatch,
		CueID: CueSenderDisplayName,
		Name:  "Sender name does not match sender or reply-to address",
		Kind:  CueCriterionBinary,
	},
	{
		ID:    CriterionSenderDomainSpoofing,
		CueID: CueDomainSpoofing,
		Name:  "Sender domain resembles a recognizable entity",
		Kind:  CueCriterionBinary,
	},
	{
		ID:    CriterionMissingBranding,
		CueID: CueMissingBranding,
		Name:  "Expected branding elements are missing",
		Kind:  CueCriterionBinary,
	},
	{
		ID:    CriterionUnprofessionalDesign,
		CueID: CueUnprofessionalDesign,
		Name:  "Email design or formatting appears unprofessional",
		Kind:  CueCriterionBinary,
	},
	{
		ID:    CriterionMissingGreeting,
		CueID: CueGenericGreeting,
		Name:  "Email is missing a greeting",
		Kind:  CueCriterionBinary,
	},
	{
		ID:    CriterionMissingPersonalization,
		CueID: CueGenericGreeting,
		Name:  "Email is missing personalization",
		Kind:  CueCriterionBinary,
	},
	{
		ID:    CriterionMissingSignerDetails,
		CueID: CueLackOfSignerDetails,
		Name:  "Email is missing sender or contact details",
		Kind:  CueCriterionBinary,
	},
	{
		ID:    CriterionMimicsBusinessProcess,
		CueID: CueMimicsBusinessProcess,
		Name:  "Email appears related to a work or business process",
		Kind:  CueCriterionBinary,
	},
	{
		ID:    CriterionPosesAsAuthority,
		CueID: CuePosesAsAuthority,
		Name:  "Email appears to come from a trusted person or authority",
		Kind:  CueCriterionBinary,
	},
	{
		ID:    CriterionSpellingErrors,
		CueID: CueSpellingGrammar,
		Name:  "Spelling errors",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionGrammarErrors,
		CueID: CueSpellingGrammar,
		Name:  "Grammar errors",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionInconsistencies,
		CueID: CueInconsistency,
		Name:  "Inconsistencies",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionAttachments,
		CueID: CueAttachmentType,
		Name:  "Attachments counted for the attachment type cue",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionHiddenURLLinks,
		CueID: CueURLHyperlinking,
		Name:  "Hyperlinks whose text hides the true URL",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionSpoofedLinkDomains,
		CueID: CueDomainSpoofing,
		Name:  "Links with domains resembling recognizable entities",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionImitatedBranding,
		CueID: CueLogoImitation,
		Name:  "Imitated branding elements",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionOutdatedBranding,
		CueID: CueLogoImitation,
		Name:  "Out-of-date branding elements",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionSecurityIndicators,
		CueID: CueSecurityIndicators,
		Name:  "Inappropriate security indicators or icons",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionLegalLanguage,
		CueID: CueLegalLanguage,
		Name:  "Uses of legal language or disclaimers",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionDistractingDetails,
		CueID: CueDistractingDetail,
		Name:  "Details not central to the message",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionSensitiveInfoRequests,
		CueID: CueSensitiveInformation,
		Name:  "Requests for sensitive information",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionTimePressure,
		CueID: CueSenseOfUrgency,
		Name:  "Expressions of time pressure",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionThreats,
		CueID: CueThreateningLanguage,
		Name:  "Threats or implied threats",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionHumanitarianAppeals,
		CueID: CueHumanitarianAppeals,
		Name:  "Appeals to help others",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionTooGoodToBeTrueOffers,
		CueID: CueTooGoodToBeTrue,
		Name:  "Offers that appear too good to be true",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionPersonalizedUnexpectedOffer,
		CueID: CueYouAreSpecial,
		Name:  "Unexpected personalized offers",
		Kind:  CueCriterionCounted,
	},
	{
		ID:    CriterionLimitedTimeOffers,
		CueID: CueLimitedTimeOffer,
		Name:  "Limited-time offers",
		Kind:  CueCriterionCounted,
	},
}

func BuildCueResults(results []CueCriterionResult) ([]CueResult, error) {
	definitions := make(map[CueCriterionID]CueCriterionDefinition)

	for _, definition := range NISTCueCriteria {
		definitions[definition.ID] = definition
	}

	seen := make(map[CueCriterionID]struct{})

	type cueAggregate struct {
		count    int
		source   CueDetectionSource
		evidence []string
	}

	aggregates := make(map[CueID]*cueAggregate)

	for _, result := range results {
		definition, exists := definitions[result.ID]
		if !exists {
			return nil, fmt.Errorf("unknown cue criterion: %q", result.ID)
		}

		if _, exists := seen[result.ID]; exists {
			return nil, fmt.Errorf("duplicate cue criterion result: %q", result.ID)
		}
		seen[result.ID] = struct{}{}

		if result.Value < 0 {
			return nil, fmt.Errorf(
				"criterion %q has invalid negative value: %d",
				result.ID,
				result.Value,
			)
		}

		if definition.Kind == CueCriterionBinary && result.Value > 1 {
			return nil, fmt.Errorf(
				"binary criterion %q must be 0 or 1",
				result.ID,
			)
		}

		if !validCueDetectionSource(result.Source) {
			return nil, fmt.Errorf(
				"criterion %q has invalid detection source: %q",
				result.ID,
				result.Source,
			)
		}

		if result.Value == 0 {
			continue
		}

		aggregate, exists := aggregates[definition.CueID]
		if !exists {
			aggregates[definition.CueID] = &cueAggregate{
				count:    result.Value,
				source:   result.Source,
				evidence: append([]string(nil), result.Evidence...),
			}
			continue
		}

		aggregate.count += result.Value
		aggregate.source = mergeCueDetectionSources(
			aggregate.source,
			result.Source,
		)
		aggregate.evidence = append(
			aggregate.evidence,
			result.Evidence...,
		)
	}

	cueResults := make([]CueResult, 0, len(aggregates))

	for _, definition := range NISTCueDefinitions {
		aggregate, exists := aggregates[definition.ID]
		if !exists {
			continue
		}

		cueResults = append(cueResults, CueResult{
			ID:       definition.ID,
			Count:    aggregate.count,
			Source:   aggregate.source,
			Evidence: aggregate.evidence,
		})
	}

	return cueResults, nil
}

func validCueDetectionSource(source CueDetectionSource) bool {
	switch source {
	case CueSourceDeterministic, CueSourceLLM, CueSourceHybrid:
		return true
	default:
		return false
	}
}

func mergeCueDetectionSources(
	current CueDetectionSource,
	next CueDetectionSource,
) CueDetectionSource {
	if current == next {
		return current
	}

	return CueSourceHybrid
}

package ai

type CueType string

const (
	CueTypeError              CueType = "error"
	CueTypeTechnicalIndicator CueType = "technical_indicator"
	CueTypeVisualPresentation CueType = "visual_presentation_indicator"
	CueTypeLanguageContent    CueType = "language_and_content"
	CueTypeCommonTactic       CueType = "common_tactic"
)

const (
	CueSpellingGrammar       CueID = "spelling_and_grammar_irregularities"
	CueInconsistency         CueID = "inconsistency"
	CueAttachmentType        CueID = "attachment_type"
	CueSenderDisplayName     CueID = "sender_display_name_and_email_address"
	CueURLHyperlinking       CueID = "url_hyperlinking"
	CueDomainSpoofing        CueID = "domain_spoofing"
	CueMissingBranding       CueID = "no_or_minimal_branding_and_logos"
	CueLogoImitation         CueID = "logo_imitation_or_outdated_branding"
	CueUnprofessionalDesign  CueID = "unprofessional_design_or_formatting"
	CueSecurityIndicators    CueID = "security_indicators_and_icons"
	CueLegalLanguage         CueID = "legal_language_and_disclaimers"
	CueDistractingDetail     CueID = "distracting_detail"
	CueSensitiveInformation  CueID = "requests_for_sensitive_information"
	CueSenseOfUrgency        CueID = "sense_of_urgency"
	CueThreateningLanguage   CueID = "threatening_language"
	CueGenericGreeting       CueID = "generic_greeting"
	CueLackOfSignerDetails   CueID = "lack_of_signer_details"
	CueHumanitarianAppeals   CueID = "humanitarian_appeals"
	CueTooGoodToBeTrue       CueID = "too_good_to_be_true_offers"
	CueYouAreSpecial         CueID = "you_are_special"
	CueLimitedTimeOffer      CueID = "limited_time_offer"
	CueMimicsBusinessProcess CueID = "mimics_work_or_business_process"
	CuePosesAsAuthority      CueID = "poses_as_friend_colleague_or_authority"
)

type CueDefinition struct {
	ID   CueID   `json:"id"`
	Name string  `json:"name"`
	Type CueType `json:"type"`
}

var NISTCueDefinitions = []CueDefinition{
	{
		ID:   CueSpellingGrammar,
		Name: "Spelling and grammar irregularities",
		Type: CueTypeError,
	},
	{
		ID:   CueInconsistency,
		Name: "Inconsistency",
		Type: CueTypeError,
	},
	{
		ID:   CueAttachmentType,
		Name: "Attachment type",
		Type: CueTypeTechnicalIndicator,
	},
	{
		ID:   CueSenderDisplayName,
		Name: "Sender display name and email address",
		Type: CueTypeTechnicalIndicator,
	},
	{
		ID:   CueURLHyperlinking,
		Name: "URL hyperlinking",
		Type: CueTypeTechnicalIndicator,
	},
	{
		ID:   CueDomainSpoofing,
		Name: "Domain spoofing",
		Type: CueTypeTechnicalIndicator,
	},
	{
		ID:   CueMissingBranding,
		Name: "No/minimal branding and logos",
		Type: CueTypeVisualPresentation,
	},
	{
		ID:   CueLogoImitation,
		Name: "Logo imitation or out-of-date branding/logos",
		Type: CueTypeVisualPresentation,
	},
	{
		ID:   CueUnprofessionalDesign,
		Name: "Unprofessional looking design or formatting",
		Type: CueTypeVisualPresentation,
	},
	{
		ID:   CueSecurityIndicators,
		Name: "Security indicators and icons",
		Type: CueTypeVisualPresentation,
	},
	{
		ID:   CueLegalLanguage,
		Name: "Legal language/copyright info/disclaimers",
		Type: CueTypeLanguageContent,
	},
	{
		ID:   CueDistractingDetail,
		Name: "Distracting detail",
		Type: CueTypeLanguageContent,
	},
	{
		ID:   CueSensitiveInformation,
		Name: "Requests for sensitive information",
		Type: CueTypeLanguageContent,
	},
	{
		ID:   CueSenseOfUrgency,
		Name: "Sense of urgency",
		Type: CueTypeLanguageContent,
	},
	{
		ID:   CueThreateningLanguage,
		Name: "Threatening language",
		Type: CueTypeLanguageContent,
	},
	{
		ID:   CueGenericGreeting,
		Name: "Generic greeting",
		Type: CueTypeLanguageContent,
	},
	{
		ID:   CueLackOfSignerDetails,
		Name: "Lack of signer details",
		Type: CueTypeLanguageContent,
	},
	{
		ID:   CueHumanitarianAppeals,
		Name: "Humanitarian appeals",
		Type: CueTypeCommonTactic,
	},
	{
		ID:   CueTooGoodToBeTrue,
		Name: "Too good to be true offers",
		Type: CueTypeCommonTactic,
	},
	{
		ID:   CueYouAreSpecial,
		Name: "You're special",
		Type: CueTypeCommonTactic,
	},
	{
		ID:   CueLimitedTimeOffer,
		Name: "Limited time offer",
		Type: CueTypeCommonTactic,
	},
	{
		ID:   CueMimicsBusinessProcess,
		Name: "Mimics a work or business process",
		Type: CueTypeCommonTactic,
	},
	{
		ID:   CuePosesAsAuthority,
		Name: "Poses as friend, colleague, supervisor, authority figure",
		Type: CueTypeCommonTactic,
	},
}

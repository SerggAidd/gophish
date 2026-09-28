package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const semanticCueMaxAttempts = 2
const maxFocusedEvidenceRepairs = 3

const focusedCueEvidenceSystemPrompt = `Recheck exactly one semantic phishing cue. The previous result was positive but had no supporting evidence. Do not assume it is correct. Return only JSON for min_value, max_value, and evidence. Confirm a positive value only with a concrete explanation grounded in the supplied email or campaign context. If it cannot be supported, use an unresolved range and explain why. Never invent sender relationships, policies, deadlines, or visible text.`

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
		response, err := e.client.ChatWithThinking(
			ctx,
			messages,
			semanticCueResponseSchema,
			evaluatorModelOptions(),
			evaluatorThinkLevel,
		)
		if err != nil {
			return nil, fmt.Errorf("evaluate semantic cues: %w", err)
		}

		results, err := parseSemanticCueResponse(response)
		if err != nil {
			// Repair a well-formed response with only unsupported positives by
			// asking about each affected criterion alone. Requesting all 23 cues
			// again repeatedly failed to supply evidence in independent runs.
			fallback, fallbackErr := parseSemanticCueResponseWithEvidenceFallback(response)
			if fallbackErr == nil {
				results, err = e.repairMissingCueEvidence(ctx, input, response, fallback)
				if err != nil {
					return nil, fmt.Errorf("repair semantic cue evidence: %w", err)
				}
			} else if attempt == semanticCueMaxAttempts {
				err = fallbackErr
			}
		}
		if err == nil {
			results = applySemanticCueContextRules(results, input)
			if _, validateErr := BuildCueResults(results); validateErr != nil {
				return nil, fmt.Errorf("validate semantic cue results after context rules: %w", validateErr)
			}
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

// A focused follow-up can provide the missing evidence without asking the
// model to regenerate the entire cue table. A conflicting count or a second
// unsupported claim keeps the original finding unresolved. The cap prevents
// one response with many omissions from triggering unbounded model calls.
func (e *SemanticCueEvaluator) repairMissingCueEvidence(
	ctx context.Context,
	input EvaluationInput,
	response string,
	results []CueCriterionResult,
) ([]CueCriterionResult, error) {
	var modelResponse semanticCueModelResponse
	if err := json.Unmarshal([]byte(response), &modelResponse); err != nil {
		return nil, err
	}
	repairs := 0
	for i := range results {
		prior := modelResponse.Results[results[i].ID]
		if prior.MinValue != prior.MaxValue || prior.MaxValue <= 0 ||
			len(cleanSemanticEvidence(prior.Evidence)) != 0 {
			continue
		}
		if repairs == maxFocusedEvidenceRepairs {
			break
		}
		repairs++

		id := results[i].ID
		focused, err := e.client.ChatWithThinking(
			ctx,
			[]Message{
				{Role: "system", Content: focusedCueEvidenceSystemPrompt},
				{Role: "user", Content: buildFocusedCueEvidencePrompt(input, id, prior.MaxValue)},
			},
			semanticCueResultProperties()[string(id)],
			evaluatorModelOptions(),
			evaluatorThinkLevel,
		)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue // Keep the explicitly unresolved cue on a failed follow-up.
		}
		repaired, err := parseFocusedCueEvidence(focused, id)
		if err != nil || repaired.MinValue != prior.MinValue || repaired.MaxValue != prior.MaxValue {
			continue
		}
		repaired.EvaluationRepair = "focused_response"
		results[i] = repaired
	}
	return results, nil
}

func parseFocusedCueEvidence(response string, id CueCriterionID) (CueCriterionResult, error) {
	var modelResult semanticCueModelResult
	if err := json.Unmarshal([]byte(response), &modelResult); err != nil {
		return CueCriterionResult{}, fmt.Errorf("parse focused cue evidence: %w", err)
	}
	evidence := cleanSemanticEvidence(modelResult.Evidence)
	if modelResult.MinValue == modelResult.MaxValue && modelResult.MaxValue > 0 && len(evidence) == 0 {
		return CueCriterionResult{}, fmt.Errorf("focused criterion %q remains positive without evidence", id)
	}
	if modelResult.MinValue != modelResult.MaxValue && len(evidence) == 0 {
		return CueCriterionResult{}, fmt.Errorf("focused criterion %q is unresolved without explanation", id)
	}
	source := CueSourceLLM
	if isHybridDomainCriterion(id) {
		source = CueSourceHybrid
	}
	result := CueCriterionResult{
		ID: id, MinValue: modelResult.MinValue, MaxValue: modelResult.MaxValue,
		Source: source, Evidence: evidence,
	}
	if _, err := BuildCueResults([]CueCriterionResult{result}); err != nil {
		return CueCriterionResult{}, err
	}
	return result, nil
}

func buildFocusedCueEvidencePrompt(input EvaluationInput, id CueCriterionID, originalValue int) string {
	definition := CueCriterionDefinition{ID: id}
	for _, candidate := range NISTCueCriteria {
		if candidate.ID == id {
			definition = candidate
			break
		}
	}
	unknownRange := "0..15"
	if definition.Kind == CueCriterionBinary {
		unknownRange = "0..1"
	}
	return fmt.Sprintf(`Recheck only criterion %s (%s). The previous model claimed %d..%d but gave no evidence. Confirm that exact value only if you can substantiate it with a specific detail from the supplied email or explicit sender/campaign context. Do not count unsupported claims. If a positive cannot be substantiated, return %s with a reason; a 0..0 answer cannot confirm that the original positive was wrong, so it will remain unresolved. Return only the single criterion result.

Organization context: %s
Sender context: %s
Audience: %s
Recipient role: %s
Simulated sender: %s
Expected sender: %s
Situation: %s
Link: %s
Attachments: %s
Domain comparison: %s
Subject: %s
Plain text: %s
HTML: %s`,
		id, definition.Name, originalValue, originalValue, unknownRange,
		valueOrDefault(input.GenerationContext.OrganizationContext, "Not specified"),
		valueOrDefault(input.GenerationContext.SenderContext, "Not specified"),
		valueOrDefault(input.GenerationContext.TargetAudience, "Not specified"),
		valueOrDefault(input.GenerationContext.RecipientRole, "Not specified"),
		formatSenderIdentity(input.EvaluationContext.SimulatedSender),
		formatSenderIdentity(input.EvaluationContext.ExpectedSender),
		valueOrDefault(input.EvaluationContext.SituationContext, "Not specified"),
		formatLinkContext(input.EvaluationContext.Link),
		formatAttachmentContext(input.EvaluationContext.Attachments),
		formatDomainComparisonContext(input.EvaluationContext),
		input.Email.Subject, input.Email.Text, input.Email.HTML,
	)
}

func parseSemanticCueResponse(response string) ([]CueCriterionResult, error) {
	return parseSemanticCueResponseWithPolicy(response, false)
}

// Use this when preparing a focused repair or after a failed full-table
// repair. An unsupported positive is neither demonstrated nor disproved.
func parseSemanticCueResponseWithEvidenceFallback(response string) ([]CueCriterionResult, error) {
	return parseSemanticCueResponseWithPolicy(response, true)
}

func parseSemanticCueResponseWithPolicy(response string, allowEvidenceFallback bool) ([]CueCriterionResult, error) {
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

		missingPositiveEvidence := exactPositive && len(evidence) == 0
		if missingPositiveEvidence {
			if !allowEvidenceFallback {
				return nil, fmt.Errorf(
					"criterion %q has exact positive value %d but no evidence",
					id,
					modelResult.MaxValue,
				)
			}
			maxUnresolved := 15 // Classification ceiling for a counted criterion.
			for _, definition := range NISTCueCriteria {
				if definition.ID == id && definition.Kind == CueCriterionBinary {
					maxUnresolved = 1
					break
				}
			}
			if modelResult.MaxValue > maxUnresolved && maxUnresolved == 1 {
				return nil, fmt.Errorf("binary criterion %q exceeds 1", id)
			}
			modelResult.MinValue = 0
			modelResult.MaxValue = maxUnresolved
			evidence = []string{fmt.Sprintf(
				"The model reported a positive value for %s but omitted supporting evidence; this criterion remains unresolved after evidence validation.",
				id,
			)}
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
		if isHybridDomainCriterion(id) || missingPositiveEvidence {
			source = CueSourceHybrid
		}

		repair := ""
		if missingPositiveEvidence {
			repair = "unsupported_positive"
		}
		results = append(results, CueCriterionResult{
			ID:               id,
			MinValue:         modelResult.MinValue,
			MaxValue:         modelResult.MaxValue,
			Source:           source,
			Evidence:         evidence,
			EvaluationRepair: repair,
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

var attachmentTypeClaimPatterns = map[string]*regexp.Regexp{
	"pdf":        regexp.MustCompile(`(?i)(?:\b(?:attached|attachment|file|document|open|download)[^\n.!?]{0,50}\bpdf\b|\bpdf\b[^\n.!?]{0,50}\b(?:attached|attachment|file|document)\b)`),
	"zip":        regexp.MustCompile(`(?i)(?:\b(?:attached|attachment|file|archive|open|download)[^\n.!?]{0,50}\bzip\b|\bzip\b[^\n.!?]{0,50}\b(?:attached|attachment|file|archive)\b)`),
	"csv":        regexp.MustCompile(`(?i)(?:\b(?:attached|attachment|file|spreadsheet|open|download)[^\n.!?]{0,50}\bcsv\b|\bcsv\b[^\n.!?]{0,50}\b(?:attached|attachment|file|spreadsheet)\b)`),
	"word":       regexp.MustCompile(`(?i)(?:\b(?:attached|attachment|file|document|open|download)[^\n.!?]{0,50}\b(?:doc|docx|word)\b|\b(?:doc|docx|word)\b[^\n.!?]{0,50}\b(?:attached|attachment|file|document)\b)`),
	"excel":      regexp.MustCompile(`(?i)(?:\b(?:attached|attachment|file|spreadsheet|open|download)[^\n.!?]{0,50}\b(?:xls|xlsx|excel)\b|\b(?:xls|xlsx|excel)\b[^\n.!?]{0,50}\b(?:attached|attachment|file|spreadsheet)\b)`),
	"powerpoint": regexp.MustCompile(`(?i)(?:\b(?:attached|attachment|file|presentation|open|download)[^\n.!?]{0,50}\b(?:ppt|pptx|powerpoint)\b|\b(?:ppt|pptx|powerpoint)\b[^\n.!?]{0,50}\b(?:attached|attachment|file|presentation)\b)`),
	"text":       regexp.MustCompile(`(?i)(?:\b(?:attached|attachment|file|document|open|download)[^\n.!?]{0,50}\b(?:txt|text file)\b|\b(?:txt|text file)\b[^\n.!?]{0,50}\b(?:attached|attachment|file|document)\b)`),
}

func applySemanticCueContextRules(
	results []CueCriterionResult,
	input EvaluationInput,
) []CueCriterionResult {
	results = suppressLocalPartOnlySenderMismatch(results)
	results = stabilizeContextGroundedCues(results, input)
	results = suppressUnsubstantiatedLanguageErrors(results)
	results = suppressHiddenImplementationEvidence(results)
	results = suppressDomainOnlyInconsistency(results)
	results = suppressCompletedReviewFalseConflict(results, input)
	results = enforceClearlyMissingSignerDetails(results, input.Email)
	results = addAttachmentTypeInconsistency(results, input)
	return results
}

// A message with no plausible signer or contact hint in either visible body
// cannot acquire signer details from the From header or campaign context.
// Keep the LLM result when a closing might be present: matching a person's
// name and job title without a conventional sign-off requires interpretation.
var signerClosingHint = regexp.MustCompile(`(?i)\b(?:regards|sincerely|respectfully|cheers|thanks|thank you|best wishes|best regards|kind regards|yours truly|с уважением|с наилучшими пожеланиями|телефон|контакт|должность)\b`)
var signerNameHint = regexp.MustCompile(`\p{Lu}[\p{L}.'-]+[ \t]+\p{Lu}[\p{L}.'-]+`)
var signerPhoneHint = regexp.MustCompile(`(?:\+?\d[\d ()-]{6,}\d)`)

func enforceClearlyMissingSignerDetails(results []CueCriterionResult, email Email) []CueCriterionResult {
	text := email.Text
	htmlText := plainTextFromHTML(recipientVisibleHTML(email.HTML))
	for _, body := range []string{text, htmlText} {
		if strings.Contains(body, "@") || signerClosingHint.MatchString(body) ||
			signerNameHint.MatchString(body) || signerPhoneHint.MatchString(body) {
			return results
		}
	}

	for i := range results {
		if results[i].ID != CriterionMissingSignerDetails || results[i].MinValue != 0 ||
			results[i].MaxValue != 0 {
			continue
		}
		results[i].MinValue = 1
		results[i].MaxValue = 1
		results[i].Source = CueSourceHybrid
		results[i].Evidence = []string{"No individual signature or contact details appear in the visible email body."}
	}
	return results
}

// A completed review satisfies an action-required review request. Limit this
// correction to the precise, single conflict reported by the model so that
// independent contradictions still count.
var completedReviewNoFurtherAction = regexp.MustCompile(`(?i)\bafter\s+completing\s+the\s+review\s*,?\s+no\s+further\s+action\s+is\s+needed\b`)
var unqualifiedNoFurtherAction = regexp.MustCompile(`(?i)\bno\s+further\s+action\s+(?:is\s+)?(?:required|needed)\b`)

func suppressCompletedReviewFalseConflict(results []CueCriterionResult, input EvaluationInput) []CueCriterionResult {
	subject := strings.ToLower(input.Email.Subject)
	message := input.Email.Text + "\n" + input.Email.HTML
	if !strings.Contains(subject, "action required") || !strings.Contains(subject, "review") ||
		!strings.Contains(strings.ToLower(message), "please review") ||
		!completedReviewNoFurtherAction.MatchString(message) {
		return results
	}
	// A separate, unqualified cancellation is still a real contradiction.
	if unqualifiedNoFurtherAction.MatchString(completedReviewNoFurtherAction.ReplaceAllString(message, "")) {
		return results
	}

	for i := range results {
		result := &results[i]
		if result.ID != CriterionInconsistencies || result.MinValue != 1 ||
			result.MaxValue != 1 || len(result.Evidence) != 1 {
			continue
		}

		evidence := strings.ToLower(result.Evidence[0])
		if strings.Contains(evidence, "subject") && strings.Contains(evidence, "action required") &&
			(strings.Contains(evidence, "conflict") || strings.Contains(evidence, "contradic")) &&
			completedReviewNoFurtherAction.MatchString(evidence) {
			result.MinValue = 0
			result.MaxValue = 0
			result.Evidence = nil
		}
	}
	return results
}

func suppressDomainOnlyInconsistency(results []CueCriterionResult) []CueCriterionResult {
	domainSpoofingDetected := false
	for _, result := range results {
		if (result.ID == CriterionSenderDomainSpoofing || result.ID == CriterionSpoofedLinkDomains) && result.MaxValue > 0 {
			domainSpoofingDetected = true
			break
		}
	}
	if !domainSpoofingDetected {
		return results
	}

	for i := range results {
		if results[i].ID != CriterionInconsistencies || results[i].MaxValue == 0 || len(results[i].Evidence) == 0 {
			continue
		}

		allDomainOnly := true
		for _, evidence := range results[i].Evidence {
			if !isDomainOnlyInconsistencyEvidence(evidence) {
				allDomainOnly = false
				break
			}
		}
		if allDomainOnly {
			results[i].MinValue = 0
			results[i].MaxValue = 0
			results[i].Evidence = nil
		}
	}

	return results
}

func isDomainOnlyInconsistencyEvidence(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || !strings.Contains(value, "domain") {
		return false
	}

	if !(strings.Contains(value, "sender") ||
		strings.Contains(value, "link") ||
		strings.Contains(value, "expected") ||
		strings.Contains(value, "legitimate")) {
		return false
	}

	for _, marker := range []string{
		"attachment",
		"subject",
		"body",
		"wording",
		"date",
		"deadline",
		"display name",
		"signature",
		"branding",
		"logo",
		"role",
		"department",
	} {
		if strings.Contains(value, marker) {
			return false
		}
	}

	return true
}

func suppressUnsubstantiatedLanguageErrors(results []CueCriterionResult) []CueCriterionResult {
	for i := range results {
		if results[i].ID != CriterionSpellingErrors && results[i].ID != CriterionGrammarErrors {
			continue
		}
		if results[i].MaxValue == 0 || len(results[i].Evidence) == 0 {
			continue
		}

		allBare := true
		for _, evidence := range results[i].Evidence {
			if !isBareLanguageEvidence(evidence) {
				allBare = false
				break
			}
		}
		if allBare {
			// A bare word does not establish that a spelling or grammar error exists.
			// Require the semantic evaluator to explain the actual visible error.
			results[i].MinValue = 0
			results[i].MaxValue = 0
			results[i].Evidence = nil
		}
	}
	return results
}

func isBareLanguageEvidence(value string) bool {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'`.,;:()[]{}")
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return false
	}

	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		// Non-ASCII letters are valid bare evidence too; punctuation/symbols are not.
		if r > 127 {
			continue
		}
		return false
	}
	return true
}

func suppressHiddenImplementationEvidence(results []CueCriterionResult) []CueCriterionResult {
	visibleOnlyCriteria := map[CueCriterionID]bool{
		CriterionSpellingErrors:     true,
		CriterionGrammarErrors:      true,
		CriterionDistractingDetails: true,
		CriterionLegalLanguage:      true,
		CriterionTimePressure:       true,
		CriterionThreats:            true,
	}

	for i := range results {
		if !visibleOnlyCriteria[results[i].ID] || results[i].MaxValue == 0 || len(results[i].Evidence) == 0 {
			continue
		}

		allHidden := true
		for _, evidence := range results[i].Evidence {
			if !isHiddenImplementationEvidence(evidence) {
				allHidden = false
				break
			}
		}
		if allHidden {
			results[i].MinValue = 0
			results[i].MaxValue = 0
			results[i].Evidence = nil
		}
	}

	return results
}

func isHiddenImplementationEvidence(value string) bool {
	value = strings.ToLower(value)
	patterns := []string{
		"display:none",
		"display: none",
		"visibility:hidden",
		"visibility: hidden",
		"hidden html",
		"html comment",
		"hidden comment",
		"hidden element",
		"not visible to the recipient",
		"invisible to the recipient",
	}
	for _, pattern := range patterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

func suppressLocalPartOnlySenderMismatch(results []CueCriterionResult) []CueCriterionResult {
	for i := range results {
		if results[i].ID != CriterionSenderNameMismatch || results[i].MaxValue == 0 {
			continue
		}

		evidence := strings.ToLower(strings.Join(results[i].Evidence, " "))
		mentionsLocalPart := strings.Contains(evidence, "local part") ||
			strings.Contains(evidence, "local-part") ||
			strings.Contains(evidence, "localpart")
		mentionsClearConflict := strings.Contains(evidence, "reply-to") ||
			strings.Contains(evidence, "reply to") ||
			strings.Contains(evidence, "different organization") ||
			strings.Contains(evidence, "different entity") ||
			strings.Contains(evidence, "unrelated organization") ||
			strings.Contains(evidence, "unrelated entity")

		if mentionsLocalPart && !mentionsClearConflict {
			results[i].MinValue = 0
			results[i].MaxValue = 0
			results[i].Evidence = nil
		}
	}

	return results
}

func addAttachmentTypeInconsistency(
	results []CueCriterionResult,
	input EvaluationInput,
) []CueCriterionResult {
	attachments := input.EvaluationContext.Attachments
	if normalizedAttachmentUsage(attachments.Usage) != AttachmentUsageUsed || len(attachments.Files) != 1 {
		return results
	}

	actualType := normalizedAttachmentType(attachments.Files[0])
	if actualType == "" {
		return results
	}

	content := strings.Join([]string{input.Email.Subject, input.Email.Text, input.Email.HTML}, "\n")
	claimedTypes := explicitAttachmentTypeClaims(content)
	if len(claimedTypes) == 0 {
		return results
	}
	if _, matches := claimedTypes[actualType]; matches {
		return results
	}

	claimed := make([]string, 0, len(claimedTypes))
	for value := range claimedTypes {
		claimed = append(claimed, value)
	}
	sort.Strings(claimed)

	for i := range results {
		if results[i].ID != CriterionInconsistencies {
			continue
		}

		evidenceText := strings.ToLower(strings.Join(results[i].Evidence, " "))
		if strings.Contains(evidenceText, strings.ToLower(attachments.Files[0].Name)) &&
			strings.Contains(evidenceText, "attachment") {
			return results
		}

		results[i].MinValue++
		results[i].MaxValue++
		results[i].Source = CueSourceHybrid
		results[i].Evidence = append(results[i].Evidence, fmt.Sprintf(
			"Email describes the attachment as %s, but the attached file %q is %s.",
			strings.Join(claimed, "/"),
			attachments.Files[0].Name,
			actualType,
		))
		return results
	}

	return results
}

func explicitAttachmentTypeClaims(content string) map[string]struct{} {
	claims := make(map[string]struct{})
	for name, pattern := range attachmentTypeClaimPatterns {
		if pattern.MatchString(content) {
			claims[name] = struct{}{}
		}
	}
	return claims
}

func normalizedAttachmentType(file AttachmentMetadata) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(file.Name)))
	switch ext {
	case ".pdf":
		return "pdf"
	case ".zip":
		return "zip"
	case ".csv":
		return "csv"
	case ".doc", ".docx":
		return "word"
	case ".xls", ".xlsx":
		return "excel"
	case ".ppt", ".pptx":
		return "powerpoint"
	case ".txt":
		return "text"
	}

	mime := strings.ToLower(strings.TrimSpace(file.Type))
	switch {
	case strings.Contains(mime, "pdf"):
		return "pdf"
	case strings.Contains(mime, "zip") || strings.Contains(mime, "compressed"):
		return "zip"
	case strings.Contains(mime, "csv"):
		return "csv"
	case strings.Contains(mime, "word"):
		return "word"
	case strings.Contains(mime, "excel") || strings.Contains(mime, "spreadsheet"):
		return "excel"
	case strings.Contains(mime, "powerpoint") || strings.Contains(mime, "presentation"):
		return "powerpoint"
	case strings.HasPrefix(mime, "text/plain"):
		return "text"
	default:
		return ""
	}
}

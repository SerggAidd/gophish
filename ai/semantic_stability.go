package ai

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Stabilize only findings that can be checked against visible content or
// explicit campaign context. The remaining semantic findings still come
// from the evaluator; this is deliberately not an evaluation cache.
func stabilizeContextGroundedCues(results []CueCriterionResult, input EvaluationInput) []CueCriterionResult {
	for i := range results {
		result := &results[i]
		switch result.ID {
		case CriterionSenderNameMismatch:
			stabilizeSenderNameMismatch(result, input.EvaluationContext)
		case CriterionMissingBranding:
			stabilizeBrandingExpectation(result, input)
		case CriterionMissingSignerDetails:
			stabilizeGroupOnlySignature(result, input.Email)
		case CriterionSpellingErrors:
			stabilizeDuplicateFormatSpelling(result, input.Email)
		case CriterionTimePressure:
			applyVisibleCountFloor(result, explicitUrgencyExpressions(input.Email))
		case CriterionThreats:
			applyVisibleCountFloor(result, explicitThreatExpressions(input.Email))
		case CriterionDistractingDetails:
			suppressRoutineFooterDistractions(result, input.Email)
			suppressSameTargetExtraLink(result, input.Email)
		}
	}
	return results
}

func setExactContextCue(result *CueCriterionResult, count int, evidence ...string) {
	result.MinValue = count
	result.MaxValue = count
	result.Source = CueSourceHybrid
	result.Evidence = evidence
}

func stabilizeSenderNameMismatch(result *CueCriterionResult, context EvaluationContext) {
	sender := context.SimulatedSender
	if sender == nil || strings.TrimSpace(sender.Email) == "" {
		return // Unknown From identity is still genuinely unresolved.
	}
	if strings.TrimSpace(sender.DisplayName) == "" {
		setExactContextCue(result, 0) // No displayed name exists to contradict the address.
		return
	}
	if strings.TrimSpace(sender.ReplyTo) != "" {
		return // The Reply-To can introduce a real identity conflict.
	}
	if context.ExpectedSender != nil && strings.EqualFold(
		strings.TrimSpace(sender.DisplayName), strings.TrimSpace(context.ExpectedSender.DisplayName),
	) {
		senderDomain, senderErr := mailboxDomain(sender.Email)
		expectedDomain, expectedErr := mailboxDomain(context.ExpectedSender.Email)
		if senderErr == nil && expectedErr == nil && sameDomainOrSubdomain(senderDomain, expectedDomain) {
			setExactContextCue(result, 0)
		}
	}
}

func stabilizeBrandingExpectation(result *CueCriterionResult, input EvaluationInput) {
	senderContext := strings.ToLower(input.GenerationContext.SenderContext)
	orgContext := strings.ToLower(input.GenerationContext.OrganizationContext)
	context := senderContext + "\n" + orgContext
	if strings.Contains(context, "do not use branded") || strings.Contains(context, "no branding expected") ||
		strings.Contains(context, "branding is not expected") {
		setExactContextCue(result, 0)
		return
	}

	sender := input.EvaluationContext.SimulatedSender
	expected := input.EvaluationContext.ExpectedSender
	if sender != nil && expected != nil && sender.DisplayName != "" &&
		strings.EqualFold(sender.DisplayName, expected.DisplayName) &&
		(strings.Contains(context, "familiar colleague") || strings.Contains(context, "familiar coworker")) {
		setExactContextCue(result, 0)
		return
	}
	if strings.Contains(context, "normally use a distinctive") ||
		strings.Contains(context, "normally carry a") ||
		strings.Contains(context, "expected brand") ||
		strings.Contains(context, "branding is required") {
		return // The documented expectation can be evaluated by the model.
	}
	if strings.Contains(strings.ToLower(recipientVisibleHTML(input.Email.HTML)), "<img ") {
		return // A visible image needs semantic inspection before deciding.
	}
	// No description of the organization's usual branding was supplied.
	// Neither an unbranded appearance nor the mere lack of a logo proves
	// that appropriate branding is missing or present.
	result.MinValue = 0
	result.MaxValue = 1
	result.Source = CueSourceHybrid
	result.Evidence = []string{"Expected branding for this sender and message type is not specified in the campaign context."}
}

var signerSignoff = regexp.MustCompile(`(?im)^\s*(?:regards|thanks|thank you|sincerely|best(?: regards)?|с уважением)\s*[,.:!]?\s*$`)
var groupSignature = regexp.MustCompile(`(?i)\b(?:IT service desk|service desk|support team|security team|human resources)\b|отдел кадров|служба безопасности`)
var signerAddress = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
var htmlBlockBreak = regexp.MustCompile(`(?i)</?(?:p|div|tr|td|table|section|h[1-6]|li)[^>]*>|<br\s*/?>`)

func signerFooter(body string) string {
	matches := signerSignoff.FindAllStringIndex(body, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimSpace(body[matches[len(matches)-1][1]:])
}

func stabilizeGroupOnlySignature(result *CueCriterionResult, email Email) {
	footer := signerFooter(email.Text)
	if footer == "" || !groupSignature.MatchString(footer) {
		return
	}
	withoutGroup := groupSignature.ReplaceAllString(signerAddress.ReplaceAllString(footer, ""), "")
	if signerNameHint.MatchString(withoutGroup) {
		return // A person might also have signed this message.
	}
	// When the HTML view has a different individual signer, neither
	// representation should silently win over the other.
	visibleHTML := recipientVisibleHTML(email.HTML)
	visibleHTML = htmlBlockBreak.ReplaceAllString(visibleHTML, "\n")
	htmlFooter := signerFooter(html.UnescapeString(htmlTagPattern.ReplaceAllString(visibleHTML, " ")))
	if htmlFooter != "" && signerNameHint.MatchString(groupSignature.ReplaceAllString(signerAddress.ReplaceAllString(htmlFooter, ""), "")) {
		result.MinValue = 0
		result.MaxValue = 1
		result.Source = CueSourceHybrid
		result.Evidence = []string{"Plain text has a group-only closing; HTML may contain an individual signer."}
		return
	}
	setExactContextCue(result, 1, "The closing names a group rather than an individual signer.")
}

var quotedTypo = regexp.MustCompile(`(?:"|“)([^"”]{2,80})(?:"|”)`)

func stabilizeDuplicateFormatSpelling(result *CueCriterionResult, email Email) {
	if result.MinValue != result.MaxValue || result.MinValue != 2 ||
		(len(result.Evidence) != 1 && len(result.Evidence) != 2) {
		return
	}
	if len(result.Evidence) == 1 && !strings.Contains(strings.ToLower(result.Evidence[0]), "twice") {
		return
	}
	match := quotedTypo.FindStringSubmatch(result.Evidence[0])
	if len(match) != 2 {
		return
	}
	if len(result.Evidence) == 2 {
		second := quotedTypo.FindStringSubmatch(result.Evidence[1])
		if len(second) != 2 || !strings.EqualFold(match[1], second[1]) {
			return
		}
	}
	phrase := strings.ToLower(match[1])
	if strings.Count(strings.ToLower(email.Text), phrase) != 1 ||
		strings.Count(strings.ToLower(plainTextFromHTML(recipientVisibleHTML(email.HTML))), phrase) != 1 {
		return
	}
	setExactContextCue(result, 1, fmt.Sprintf("The same misspelling %q appears once in each alternative email format.", match[1]))
}

var urgencyMarker = regexp.MustCompile(`(?i)\b(?:urgent|immediate(?:ly)?|right\s+now|as soon as possible|end of day|within\s+\d+\s+(?:hours?|days?)|next\s+\d+\s+(?:hours?|days?)|expires?\s+in\s+\d+\s+(?:hours?|days?)|expir(?:e|es|ing)\s+soon|before\s+\d{1,2}:\d{2}|by\s+\d{1,2}:\d{2}|(?:reset|review|confirm|verify|submit|update|act|respond|open|click|read|pay)\b[^.!?\n]{0,80}\bnow)\b|срочн\p{L}*|немедлен\p{L}*`)
// A dot inside {{.URL}}, an address, or a domain is not a sentence boundary.
var sentenceBoundary = regexp.MustCompile(`[!?]+|\.\s+|\.$|\n+`)
var nowCTALabel = regexp.MustCompile(`(?i)^([\p{L}][\p{L}\s-]{3,70}\bnow)(?:\s*:\s*\{\{\.URL\}\})?[!.\s]*$`)
var nowWord = regexp.MustCompile(`(?i)\bnow\b`)
var quickAccessReassurance = regexp.MustCompile(`(?i)\bafter\b.{0,100}\bresetting\b.{0,100}\breactivated\s+immediately\b`)
var unrelatedSecurityReporting = regexp.MustCompile(`(?i)\bif\s+you\s+(?:notice|suspect|see)\b.{0,100}\b(?:unusual|suspicious)\b.{0,100}\breport\b.{0,40}\bimmediately\b`)

func explicitUrgencyExpressions(email Email) []string {
	expressions := uniqueVisibleSentences(email, urgencyMarker, true)
	seen := make(map[string]bool)
	for _, sentence := range expressions {
		seen[urgencySentenceKey(sentence)] = true
	}
	addCTA := func(label string) {
		label = strings.TrimSpace(spacePattern.ReplaceAllString(label, " "))
		match := nowCTALabel.FindStringSubmatch(label)
		if len(match) != 2 || urgencyMarker.MatchString(label) {
			return
		}
		key := strings.ToLower(match[1])
		if !seen[key] {
			seen[key] = true
			expressions = append(expressions, "Action link: "+match[1])
		}
	}
	for _, line := range strings.Split(email.Text, "\n") {
		addCTA(line)
	}
	// A button label can itself be a distinct urgent instruction, even when
	// the surrounding sentence simply says to follow a link.
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(recipientVisibleHTML(email.HTML)))
	if err == nil {
		doc.Find("a, button").Each(func(_ int, link *goquery.Selection) {
			addCTA(link.Text())
		})
	}
	return expressions
}

var conditionalThreat = regexp.MustCompile(`(?i)\bif\s+you\s+(?:do\s+not|don't|fail\s+to)\b.*\b(?:disabled|locked|lockout|suspended|lose\s+access|consequences\s+will\s+follow)\b`)

func explicitThreatExpressions(email Email) []string {
	return uniqueVisibleSentences(email, conditionalThreat, false)
}

func urgencySentenceKey(sentence string) string {
	key := strings.ToLower(strings.TrimSpace(sentence))
	key = strings.TrimPrefix(key, "subject: ")
	if now := nowWord.FindStringIndex(key); now != nil {
		// Text/HTML may render the same action as a URL, button, or link
		// label. The action through "now" is the same visible instruction.
		key = key[:now[1]]
	}
	return strings.TrimSpace(strings.TrimRight(key, " .,!?:"))
}

func uniqueVisibleSentences(email Email, pattern *regexp.Regexp, timePressure bool) []string {
	seen := make(map[string]bool)
	var evidence []string
	for _, source := range []string{
		"Subject: " + email.Subject,
		email.Text,
		visibleHTMLSentenceText(email.HTML),
	} {
		for _, sentence := range sentenceBoundary.Split(source, -1) {
			sentence = strings.TrimSpace(spacePattern.ReplaceAllString(sentence, " "))
			match := pattern.FindStringIndex(sentence)
			if sentence == "" || match == nil {
				continue
			}
			if timePressure && (quickAccessReassurance.MatchString(sentence) ||
				unrelatedSecurityReporting.MatchString(sentence)) {
				continue // A post-action benefit or separate security advice is not pressure to follow this email's CTA.
			}
			// Text and HTML can carry identical wording with a layout label or
			// button prefix in one rendering. Compare from the actual cue onward.
			key := strings.ToLower(strings.TrimSpace(sentence[match[0]:]))
			if timePressure {
				// "Immediately" alone cannot identify a cue: two different
				// sentences may end in the same adverb. Keep their full meaning.
				key = urgencySentenceKey(sentence)
			}
			if !seen[key] {
				seen[key] = true
				evidence = append(evidence, sentence)
			}
		}
	}
	return evidence
}

func visibleHTMLSentenceText(value string) string {
	// Preserve block boundaries: stripping all tags to spaces joins the
	// heading, greeting and next paragraph into a fictitious single sentence.
	blocks := htmlBlockBreak.ReplaceAllString(recipientVisibleHTML(value), "\n")
	return html.UnescapeString(htmlTagPattern.ReplaceAllString(blocks, " "))
}

func applyVisibleCountFloor(result *CueCriterionResult, expressions []string) {
	if len(expressions) <= result.MinValue {
		if len(expressions) > 0 && len(expressions) == result.MinValue &&
			result.MinValue == result.MaxValue {
			for _, evidence := range result.Evidence {
				if quickAccessReassurance.MatchString(evidence) || unrelatedSecurityReporting.MatchString(evidence) {
					// The count is independently supported, but the model cited
					// a reassurance or safety instruction instead of an urgent CTA.
					result.Source = CueSourceHybrid
					result.Evidence = expressions
					break
				}
			}
		}
		return
	}
	// Unlike model evidence, these are independently present in the subject
	// or visible body. Do not shrink an unresolved upper bound.
	result.MinValue = len(expressions)
	if result.MaxValue < result.MinValue {
		result.MaxValue = result.MinValue
	}
	result.Source = CueSourceHybrid
	result.Evidence = expressions
}

var extraInformationLink = regexp.MustCompile(`(?is)<a\b[^>]*href\s*=\s*["']?\{\{\.URL\}\}["']?[^>]*>\s*Click here for additional information\s*</a>`)

var routineFooterDetails = map[string]bool{
	"приносим извинения за неудобства": true,
	"данный запрос отправлен автоматически": true,
	"this is an automated message": true,
	"this is an automated notification": true,
	"we apologize for the inconvenience": true,
	"we apologize for any inconvenience": true,
}

func suppressRoutineFooterDistractions(result *CueCriterionResult, email Email) {
	if result.MinValue != result.MaxValue || result.MinValue == 0 ||
		len(result.Evidence) != result.MinValue {
		return
	}
	visible := strings.ToLower(email.Text + "\n" + plainTextFromHTML(recipientVisibleHTML(email.HTML)))
	kept := make([]string, 0, len(result.Evidence))
	for _, evidence := range result.Evidence {
		phrase := strings.ToLower(strings.Trim(strings.TrimSpace(evidence), ` "'“”«».,!`))
		if routineFooterDetails[phrase] && strings.Contains(visible, phrase) {
			continue // Standard delivery notice or apology alone is not a diversion.
		}
		kept = append(kept, evidence)
	}
	if len(kept) != len(result.Evidence) {
		result.MinValue = len(kept)
		result.MaxValue = len(kept)
		result.Evidence = kept
		result.Source = CueSourceHybrid
	}
}

func suppressSameTargetExtraLink(result *CueCriterionResult, email Email) {
	if result.MinValue != result.MaxValue || result.MinValue == 0 ||
		len(result.Evidence) != result.MinValue ||
		!extraInformationLink.MatchString(recipientVisibleHTML(email.HTML)) ||
		strings.Count(email.Text, "{{.URL}}") < 2 {
		return
	}
	for i, value := range result.Evidence {
		if !strings.Contains(strings.ToLower(value), "additional information") {
			continue
		}
		result.Evidence = append(result.Evidence[:i], result.Evidence[i+1:]...)
		result.MinValue--
		result.MaxValue--
		result.Source = CueSourceHybrid
		return // Same policy URL: no unrelated extra destination was introduced.
	}
}

package ai

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	anchorPattern  = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	hrefPattern    = regexp.MustCompile(`(?is)\bhref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	htmlTagPattern = regexp.MustCompile(`(?is)<[^>]+>`)
	spacePattern   = regexp.MustCompile(`\s+`)
)

var greetingPrefixes = []string{
	"hello",
	"hi ",
	"hi,",
	"dear",
	"good morning",
	"good afternoon",
	"good evening",
	"здравствуйте",
	"добрый день",
	"доброе утро",
	"добрый вечер",
	"привет",
	"уважаемый",
	"уважаемая",
	"уважаемые",
}

var personalizationVariables = []string{
	"{{.FirstName}}",
	"{{.LastName}}",
	"{{.Email}}",
	"{{.Position}}",
}

// DetectDeterministicCueCriteria evaluates cue criteria that can be derived
// from the email and campaign context without semantic interpretation.
func DetectDeterministicCueCriteria(input EvaluationInput) []CueCriterionResult {
	return []CueCriterionResult{
		detectAttachments(input.EvaluationContext),
		detectMissingGreeting(input.Email),
		detectMissingPersonalization(input.Email),
		detectHiddenURLLinks(input.Email),
	}
}

func detectAttachments(context EvaluationContext) CueCriterionResult {
	result := CueCriterionResult{
		ID:     CriterionAttachments,
		Source: CueSourceDeterministic,
	}

	switch normalizedAttachmentUsage(context.Attachments.Usage) {
	case AttachmentUsageNone:
		return result
	case AttachmentUsageUsed:
		result.MinValue = len(context.Attachments.Files)
		result.MaxValue = len(context.Attachments.Files)
		for _, file := range context.Attachments.Files {
			result.Evidence = append(
				result.Evidence,
				fmt.Sprintf("Attachment: %s", file.Name),
			)
		}
		return result
	default:
		// The exact number is unknown. Fifteen is sufficient as the upper bound
		// for NIST cue-category classification because all values >=15 are Many.
		result.MaxValue = CueCountManyMin
		result.Evidence = []string{
			"Attachment configuration is unknown; the upper bound is capped at the NIST Many threshold for classification.",
		}
		return result
	}
}

func detectMissingGreeting(email Email) CueCriterionResult {
	body := strings.TrimSpace(email.Text)
	if body == "" {
		body = plainTextFromHTML(email.HTML)
	}

	result := CueCriterionResult{
		ID:     CriterionMissingGreeting,
		Source: CueSourceDeterministic,
	}

	if hasGreeting(body) {
		return result
	}

	result.MinValue = 1
	result.MaxValue = 1
	result.Evidence = []string{
		"No greeting was detected near the beginning of the email.",
	}

	return result
}

func detectMissingPersonalization(email Email) CueCriterionResult {
	content := email.Subject + "\n" + email.Text + "\n" + email.HTML

	result := CueCriterionResult{
		ID:     CriterionMissingPersonalization,
		Source: CueSourceDeterministic,
	}

	for _, variable := range personalizationVariables {
		if strings.Contains(content, variable) {
			return result
		}
	}

	result.MinValue = 1
	result.MaxValue = 1
	result.Evidence = []string{
		"No recipient-specific GoPhish template variable was detected.",
	}

	return result
}

func detectHiddenURLLinks(email Email) CueCriterionResult {
	result := CueCriterionResult{
		ID:     CriterionHiddenURLLinks,
		Source: CueSourceDeterministic,
	}

	for _, match := range anchorPattern.FindAllStringSubmatch(email.HTML, -1) {
		if len(match) < 3 {
			continue
		}

		href := extractHref(match[1])
		if !isCountableHref(href) {
			continue
		}

		displayText := normalizeAnchorText(match[2])
		if displayText == "" {
			continue
		}

		if linkTextMatchesHref(displayText, href) {
			continue
		}

		result.MinValue++
		result.MaxValue++
		result.Evidence = append(
			result.Evidence,
			fmt.Sprintf(
				"Link text %q hides target %q.",
				displayText,
				href,
			),
		)
	}

	return result
}

func hasGreeting(body string) bool {
	body = strings.TrimSpace(body)
	if body == "" {
		return false
	}

	if len(body) > 300 {
		body = body[:300]
	}

	lower := strings.ToLower(body)

	for _, prefix := range greetingPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	for _, variable := range []string{"{{.FirstName}}", "{{.LastName}}"} {
		if strings.HasPrefix(body, variable) {
			return true
		}
	}

	return false
}

func plainTextFromHTML(value string) string {
	text := htmlTagPattern.ReplaceAllString(value, " ")
	text = html.UnescapeString(text)
	text = spacePattern.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

func extractHref(attributes string) string {
	match := hrefPattern.FindStringSubmatch(attributes)
	if len(match) == 0 {
		return ""
	}

	for i := 1; i < len(match); i++ {
		if match[i] != "" {
			return strings.TrimSpace(html.UnescapeString(match[i]))
		}
	}

	return ""
}

func normalizeAnchorText(value string) string {
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	value = spacePattern.ReplaceAllString(value, " ")

	return strings.TrimSpace(value)
}

func isCountableHref(href string) bool {
	href = strings.TrimSpace(href)
	if href == "" || href == "#" {
		return false
	}

	lower := strings.ToLower(href)

	return !strings.HasPrefix(lower, "mailto:") &&
		!strings.HasPrefix(lower, "tel:") &&
		!strings.HasPrefix(lower, "javascript:")
}

func linkTextMatchesHref(displayText, href string) bool {
	displayText = normalizeURLText(displayText)
	href = normalizeURLText(href)

	return displayText != "" && displayText == href
}

func normalizeURLText(value string) string {
	value = strings.TrimSpace(html.UnescapeString(value))
	value = strings.Trim(value, `"'<>`)
	value = strings.ToLower(value)

	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "www.")
	value = strings.TrimSuffix(value, "/")

	return value
}

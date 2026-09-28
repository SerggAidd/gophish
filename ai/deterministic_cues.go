package ai

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	htmlTagPattern     = regexp.MustCompile(`(?is)<[^>]+>`)
	spacePattern       = regexp.MustCompile(`\s+`)
	hiddenStylePattern = regexp.MustCompile(`(?i)(?:^|;)\s*(?:display\s*:\s*none|visibility\s*:\s*(?:hidden|collapse))\s*(?:!important)?\s*(?:;|$)`)
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
		body = plainTextFromHTML(recipientVisibleHTML(email.HTML))
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
	content := email.Subject + "\n" + email.Text + "\n" + recipientVisibleHTML(email.HTML)

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

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(recipientVisibleHTML(email.HTML)))
	if err != nil {
		return result
	}
	doc.Find("a").Each(func(_ int, anchor *goquery.Selection) {
		href, _ := anchor.Attr("href")
		href = strings.TrimSpace(html.UnescapeString(href))
		if !isCountableHref(href) {
			return
		}

		displayText := strings.TrimSpace(spacePattern.ReplaceAllString(anchor.Text(), " "))
		if displayText == "" {
			return
		}

		if linkTextMatchesHref(displayText, href) {
			return
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
	})

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

// recipientVisibleHTML removes explicitly hidden nodes and their descendants.
// External CSS and browser layout are outside the scope of this parser.
func recipientVisibleHTML(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(value))
	if err != nil {
		return ""
	}
	doc.Find("script, style, template, noscript, head, [hidden]").Remove()
	doc.Find("*").Each(func(_ int, node *goquery.Selection) {
		style, _ := node.Attr("style")
		ariaHidden, _ := node.Attr("aria-hidden")
		if hiddenStylePattern.MatchString(style) || strings.EqualFold(strings.TrimSpace(ariaHidden), "true") {
			node.Remove()
		}
	})
	visible, err := doc.Find("body").Html()
	if err != nil {
		return ""
	}
	return visible
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

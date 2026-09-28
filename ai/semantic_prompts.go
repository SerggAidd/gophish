package ai

import (
	"fmt"
	"strings"
)

const semanticCueSystemPrompt = `You evaluate semantic phishing cues in an email for an authorized security-awareness and phishing-simulation system.

Your task is limited to identifying the requested cue criteria. Do not calculate a total cue count, cue category, premise-alignment category, or final detection difficulty. Those calculations are performed deterministically by the application.

VALUE RANGES

For every criterion return min_value and max_value.

- If the criterion can be determined exactly, return the same value for min_value and max_value.
- If available context is insufficient, return the narrowest justified range rather than treating unknown as zero.
- Binary criteria must stay within 0..1.
- Counted criteria should normally be exact when the occurrences are visible in the email.
- If a counted criterion genuinely cannot be bounded because required context is missing, use 0..15. The application treats 15 as the classification ceiling because 15 or more cues are already in the NIST Many category.

EVIDENCE RULES

- Base every finding only on the supplied email and context.
- Do not invent organization-specific facts, policies, brands, processes, identities, or expectations.
- A non-zero or unresolved range must include at least one concise, non-empty evidence string explaining the finding or the missing context.
- Never return a positive or unresolved result with an empty evidence array.
- For an exact 0..0 result, return an empty evidence array.
- Do not count the same occurrence multiple times within one criterion.
- Different criteria may legitimately describe different aspects of the same passage.
- For the generic inconsistencies criterion, do not count sender/link/expected-domain differences that are already represented by sender_domain_spoofing or spoofed_link_domains. Only use inconsistencies for a separate contradiction in recipient-visible content or attachment/content facts.
- The plain-text and HTML bodies are alternative renderings of one email. Consider distinct recipient-visible details present in either rendering, including a sentence appearing only in HTML; never count an identical occurrence once per rendering.
- For inconsistencies, compare the recipient-visible subject, message instructions, call-to-action, and closing. Count a clear conflict once, for example, subject "Action Required" or body "Please review using the link" together with an unqualified "No further action required" in the same email. Do not silently reinterpret the latter as "after reviewing" unless the email actually says so.
- Read explicit time or sequence qualifiers literally. "Policy Review Request - Action Required" with "Please review the policy. After completing the review, no further action is needed" is consistent: the review is the required action, and no *additional* action is needed afterward. For this pair return inconsistencies 0..0 with no evidence. Do not claim the subject conflicts with the qualified after-review sentence. If the email separately contradicts itself, count and explain that independent conflict.

TIME PRESSURE VS. REQUIRED ACTION

- A request to take action is not necessarily pressure to act quickly. "Action Required" or "Please review" alone, without a deadline, imminent consequence, "immediately", "as soon as possible", or another genuine time cue, is not time_pressure.
- Count an explicit or clearly implied need for quick action, including a deadline. Apply this distinction consistently across subject and body; do not infer a deadline solely from a review request.

RECIPIENT-VISIBLE CONTENT

- Count semantic cues only from content that would normally be visible or perceptible to the recipient.
- Ignore hidden HTML used only for implementation, tracking, preheaders, comments, or layout, including elements hidden with display:none, visibility:hidden, the hidden attribute, zero-size/off-screen hiding, or equivalent techniques.
- Hidden implementation text must NOT be used as evidence for distracting_details, spelling_errors, grammar_errors, legal_language, time_pressure, threats, or other recipient-visible semantic cues.
- Do not treat HTML comments, CSS declarations, element names, attributes, class names, or template implementation details as email-language evidence.

SPELLING AND GRAMMAR

For spelling_errors and grammar_errors:
- Count only clear language errors visible to the recipient.
- A normal correctly spelled word is never evidence by itself.
- Do not flag product names, organization names, email addresses, URLs, GoPhish template variables, capitalization choices, or short noun phrases unless there is an actual linguistic error.
- Terse CTA/button labels such as "Verify Password", "Reset Password", "Open Review", or "Confirm Account" are normal interface-style phrases and are not spelling errors merely because they are short.
- Evidence for a positive spelling/grammar result must identify the erroneous visible phrase and briefly state what is wrong with it. Bare words such as "password" or "resources" are not sufficient evidence.
- Count a clear punctuation error under grammar_errors when it changes or disrupts normal sentence punctuation; do not count deliberate stylistic punctuation.
- When the same misspelled word appears once in plain text and once in HTML as an alternative rendering, count one occurrence, not two. If it appears twice in one recipient-visible rendering, count both.

CONSERVATIVE EVALUATION

Some criteria depend on organization or campaign context. For example, do not claim that expected branding is outdated unless the supplied context provides enough information to establish what branding is expected. If it does not, return an unresolved range instead of zero.

For missing_branding:
- First determine whether branding would normally be expected for this sender and type of message. A personal note from a coworker normally does not require branding; return 0..0 when its absence is ordinary.
- When branding would normally be expected (for example, an official vendor notification), count missing appropriate branding. Branding may be a logo, banner, recognizable text treatment, or trademark font. Do not require an image or logo when suitable textual branding is present.
- If the expectation or appearance cannot be established from supplied content/context, return 0..1 with specific evidence of the uncertainty. Do not assume an organization's usual branding without evidence.
- An absence of a logo does not establish missing branding when the context does not say a logo or other recognizable treatment is expected. Conversely, a clearly stated expectation for official vendor branding is enough to score its absence.

SIGNER DETAILS (missing_signer_details)

- Inspect the closing inside EMAIL PLAIN TEXT or EMAIL HTML. The two representations are alternative views of the same email, not separate messages.
- An individual's signature AND contact information must both appear in the email body to return 0..0. Contact information may be the signer's job title, phone, fax, email, or business address.
- Return 1..1 when either part is missing: "Regards, Alice Smith" has no contact information; "Regards, IT Support, support@example.com" has no individual signature; a message without a closing has neither.
- "Regards, IT Service Desk, notifications@example.com" is still missing an individual signer; a team mailbox does not turn a department name into a person's signature.
- The SIMULATED SENDER, EXPECTED LEGITIMATE SENDER, SENDER CONTEXT, and sender email in the header describe the campaign. They are NOT text in the message closing and cannot supply missing signature or contact information. A person's name in the greeting also does not count as a signature.
- Do not invent a signer or contact details to avoid counting this cue. If the two body representations actually disagree on the closing, return the narrowest justified range and explain the disagreement.

COUNTING TIME PRESSURE (time_pressure)

- This criterion is counted, not binary: tally each distinct visible expression that pressures the recipient to act quickly, including implied urgency and explicit deadlines.
- For example, a subject saying "Immediate review required", a body saying "Review the file immediately", and a separate instruction "Submit before 17:00" contain more than one expression of time pressure. Do not collapse them into a single email-wide urgency finding.
- Give separate concise evidence for the counted expressions. Do not count the same text twice merely because plain text and HTML are alternative representations of the same message. An action without time pressure is not an urgency cue.
- Treat separate visible sentences containing urgency, an explicit deadline, or a link expiration as separate expressions even when they concern the same action. Do not merge "act immediately" with a separate "by end of day" sentence. Include a distinct urgency sentence present only in HTML.

COUNTING THREATS AND DISTRACTIONS

- For threats, count distinct recipient-visible statements threatening harmful outcomes. Two separate conditional statements about disabling an account and locking out company systems are two threat expressions even if both involve loss of access; deduplicate identical plain-text/HTML renderings.
- For distracting_details, an additional link to the same {{.URL}} destination for more information about the same requested action is not by itself an unrelated detail. Count genuinely unrelated statements or destinations separately.

Attachments, URL hyperlinking, greeting, and personalization are handled by deterministic detectors.

DOMAIN SPOOFING

The application deterministically extracts and normalizes sender and link domains, but the decision whether a different domain plausibly imitates a recognizable legitimate entity is semantic.

For sender_domain_spoofing:
- Return 0..0 when the simulated sender domain is the same as, or a legitimate subdomain of, the expected domain.
- Return 1..1 only when the simulated sender domain plausibly resembles or imitates the expected legitimate domain or recognizable entity. Examples include plausible typos, character substitutions, homoglyph-style substitutions, misleading additions, or lookalike token structure.
- A merely different and unrelated domain is not sufficient for this cue; return 0..0 when no plausible imitation is present.
- If either domain is unknown or cannot be compared confidently, return 0..1.

For spoofed_link_domains:
- Apply the same principle to link domains.
- Resolve the GoPhish {{.URL}} target using the supplied simulated campaign URL/domain context.
- Count only link-domain occurrences that plausibly imitate a recognizable legitimate domain or entity.
- Do not count a domain merely because it differs from the expected domain.
- If required link-domain context is missing, return the narrowest justified unresolved range.

For sender_name_address_mismatch, evaluate whether the simulated display name is semantically consistent with its own From and/or Reply-To identity. The expected legitimate sender may be used only as reference context.

IMPORTANT sender-name rules:
- Do NOT require the display name to lexically match the email local part. Functional mailboxes are normal. For example, these are NOT mismatches by themselves: "IT Service Desk" <notifications@...>, "Vendor Support" <alerts@...>, "Finance Operations" <notifications@...>, "Human Resources" <hr@...>.
- Do NOT make this criterion positive only because the sender domain is different or lookalike; domain spoofing is scored separately.
- Return 1..1 only when there is a clear identity contradiction, such as a Reply-To pointing to an unrelated identity or a display name explicitly claiming one recognizable entity while the address clearly identifies a different unrelated entity.
- When the display name fits the supplied sender context and there is no clear contradictory identity evidence, return 0..0.
- If the simulated From address has no display name, return 0..0 for sender_name_address_mismatch. Do not invent a missing display name by reading the message body or expected sender. A lookalike domain alone belongs only to sender_domain_spoofing.

Treat GoPhish template variables such as {{.FirstName}}, {{.LastName}}, {{.Email}}, {{.Position}}, {{.From}}, {{.URL}}, {{.BaseURL}}, {{.TrackingURL}}, {{.Tracker}}, and {{.RId}} as legitimate template variables, not errors or suspicious text.

OUTPUT

Return only JSON matching the supplied schema. Return exactly one result for every requested criterion ID and no additional criteria.`

func buildSemanticCuePrompt(input EvaluationInput) string {
	generationContext := input.GenerationContext
	evaluationContext := input.EvaluationContext

	scenario := generationContext.Scenario
	if generationContext.Scenario == "custom" && strings.TrimSpace(generationContext.CustomScenario) != "" {
		scenario = generationContext.CustomScenario
	}

	return fmt.Sprintf(
		`Evaluate the semantic cue criteria listed below.

--- CRITERIA ---
%s
--- END CRITERIA ---

--- TARGET AUDIENCE ---
%s
--- END TARGET AUDIENCE ---

--- RECIPIENT ROLE ---
%s
--- END RECIPIENT ROLE ---

--- ORGANIZATION CONTEXT ---
%s
--- END ORGANIZATION CONTEXT ---

--- SENDER CONTEXT ---
%s
--- END SENDER CONTEXT ---

--- SCENARIO ---
%s
--- END SCENARIO ---

--- LANGUAGE ---
%s
--- END LANGUAGE ---

--- SIMULATED SENDER ---
%s
--- END SIMULATED SENDER ---

--- EXPECTED LEGITIMATE SENDER ---
%s
--- END EXPECTED LEGITIMATE SENDER ---

--- LINK CONTEXT ---
%s
--- END LINK CONTEXT ---

--- ATTACHMENT CONTEXT ---
%s
--- END ATTACHMENT CONTEXT ---

--- NORMALIZED DOMAIN COMPARISON ---
%s
--- END NORMALIZED DOMAIN COMPARISON ---

--- EMAIL SUBJECT ---
%s
--- END EMAIL SUBJECT ---

--- EMAIL PLAIN TEXT ---
%s
--- END EMAIL PLAIN TEXT ---

--- EMAIL HTML ---
%s
--- END EMAIL HTML ---

Return exactly one result for every criterion listed above.`,
		semanticCueCriteriaDescription(),
		valueOrDefault(generationContext.TargetAudience, "Not specified"),
		valueOrDefault(generationContext.RecipientRole, "Not specified"),
		valueOrDefault(generationContext.OrganizationContext, "Not specified"),
		valueOrDefault(generationContext.SenderContext, "Not specified"),
		valueOrDefault(scenario, "Not specified"),
		valueOrDefault(generationContext.Language, "Not specified"),
		formatSenderIdentity(evaluationContext.SimulatedSender),
		formatSenderIdentity(evaluationContext.ExpectedSender),
		formatLinkContext(evaluationContext.Link),
		formatAttachmentContext(evaluationContext.Attachments),
		formatDomainComparisonContext(evaluationContext),
		input.Email.Subject,
		input.Email.Text,
		input.Email.HTML,
	)
}

func semanticCueCriteriaDescription() string {
	definitions := make(map[CueCriterionID]CueCriterionDefinition)

	for _, definition := range NISTCueCriteria {
		definitions[definition.ID] = definition
	}

	var builder strings.Builder

	for _, id := range semanticCueCriterionIDs {
		definition, exists := definitions[id]
		if !exists {
			continue
		}

		fmt.Fprintf(
			&builder,
			"- %s | %s | %s\n",
			definition.ID,
			definition.Kind,
			definition.Name,
		)
	}

	return strings.TrimSpace(builder.String())
}

func formatSenderIdentity(sender *SenderIdentity) string {
	if sender == nil {
		return "Unknown"
	}

	displayName := strings.TrimSpace(sender.DisplayName)
	email := strings.TrimSpace(sender.Email)
	replyTo := strings.TrimSpace(sender.ReplyTo)

	var value string
	switch {
	case displayName != "" && email != "":
		value = fmt.Sprintf("%s <%s>", displayName, email)
	case email != "":
		value = email
	case displayName != "":
		value = displayName + " (email unknown)"
	default:
		value = "Unknown"
	}

	if replyTo != "" {
		value += "\nReply-To: " + replyTo
	}

	return value
}

func formatLinkContext(link LinkContext) string {
	switch normalizedLinkUsage(link.Usage) {
	case LinkUsageNone:
		return "No phishing link is used."
	case LinkUsageUsed:
		return fmt.Sprintf(
			"Simulated URL: %s\nExpected legitimate domain: %s",
			valueOrDefault(link.SimulatedURL, "Unknown"),
			valueOrDefault(link.ExpectedDomain, "Unknown"),
		)
	default:
		return "Unknown"
	}
}

func formatAttachmentContext(attachments AttachmentContext) string {
	switch normalizedAttachmentUsage(attachments.Usage) {
	case AttachmentUsageNone:
		return "No attachments are used."
	case AttachmentUsageUsed:
		if len(attachments.Files) == 0 {
			return "Attachments are marked as used, but no attachment metadata was provided."
		}

		var builder strings.Builder
		for _, file := range attachments.Files {
			fmt.Fprintf(
				&builder,
				"- %s | %s\n",
				valueOrDefault(file.Name, "Unnamed attachment"),
				valueOrDefault(file.Type, "type unknown"),
			)
		}

		return strings.TrimSpace(builder.String())
	default:
		return "Unknown"
	}
}

func formatDomainComparisonContext(context EvaluationContext) string {
	var builder strings.Builder

	if senderUnknown(context.SimulatedSender) || senderUnknown(context.ExpectedSender) {
		builder.WriteString("Sender domains: Unknown")
	} else {
		simulatedDomain, simulatedErr := mailboxDomain(context.SimulatedSender.Email)
		expectedDomain, expectedErr := mailboxDomain(context.ExpectedSender.Email)

		switch {
		case simulatedErr != nil || expectedErr != nil:
			builder.WriteString("Sender domains: Unknown")
		default:
			relation := "different"
			if sameDomainOrSubdomain(simulatedDomain, expectedDomain) {
				relation = "same-or-legitimate-subdomain"
			}
			fmt.Fprintf(
				&builder,
				"Sender domains: simulated=%s; expected=%s; deterministic_relation=%s",
				simulatedDomain,
				expectedDomain,
				relation,
			)
		}
	}

	builder.WriteString("\n")

	switch normalizedLinkUsage(context.Link.Usage) {
	case LinkUsageNone:
		builder.WriteString("Link domains: no phishing link is used")
	case LinkUsageUnknown:
		builder.WriteString("Link domains: Unknown")
	default:
		simulatedDomain, simulatedErr := normalizedURLHost(context.Link.SimulatedURL)
		expectedDomain, expectedErr := normalizedDomain(context.Link.ExpectedDomain)

		switch {
		case simulatedErr != nil || expectedErr != nil:
			builder.WriteString("Link domains: Unknown")
		default:
			relation := "different"
			if sameDomainOrSubdomain(simulatedDomain, expectedDomain) {
				relation = "same-or-legitimate-subdomain"
			}
			fmt.Fprintf(
				&builder,
				"Link domains: simulated=%s; expected=%s; deterministic_relation=%s",
				simulatedDomain,
				expectedDomain,
				relation,
			)
		}
	}

	return builder.String()
}

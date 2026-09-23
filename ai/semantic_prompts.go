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

CONSERVATIVE EVALUATION

Some criteria depend on organization or campaign context. For example, do not claim that expected branding is outdated unless the supplied context provides enough information to establish what branding is expected. If it does not, return an unresolved range instead of zero.

For missing_branding, apply a stricter observable rule:
- A textual organization or sender name alone is not a branding element.
- If the supplied HTML contains no recognizable logo, branded image, branded header/footer, or comparable visual brand element, return 1..1.
- Return 0..0 when recognizable visual branding is clearly present.
- Use 0..1 only when the supplied HTML references branding that cannot actually be inspected from the provided content.
This rule is intentionally narrow so that the same email is evaluated consistently across repeated runs.

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

For sender_name_address_mismatch, evaluate whether the simulated display name is consistent with its own From and/or Reply-To identity. Compare the visible name with the identity implied by the local part, From address, and Reply-To address. The expected legitimate sender may be used only as reference context. A different or spoofed domain by itself MUST NOT make this criterion positive because domain spoofing is scored separately. For example, a display name such as "Microsoft 365 Support" with a From local part such as "support" can remain internally consistent even when its domain is a lookalike domain.

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

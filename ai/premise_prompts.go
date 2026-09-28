package ai

import "fmt"

const premiseAlignmentSystemPrompt = `You evaluate premise alignment for an authorized security-awareness phishing simulation using the NIST Phish Scale method.

Evaluate only the four semantic premise-alignment elements requested by the application. The fifth element, prior phishing training or exposure, is supplied directly by the user and is not scored by you.

SCORING

Use only these applicability scores:
- 0 = Not applicable
- 2 = Low
- 4 = Moderate
- 6 = Significant
- 8 = Extreme

ELEMENTS

workplace_process:
How strongly the premise relates to a plausible workplace process or practice for the stated audience.

workplace_relevance:
How pertinent the premise is to the actual roles and responsibilities of the target audience.
Consider the supplied simulated sender name and domain and the expected sender identity when judging relevance: a familiar person or organizational domain may make a message more relevant to this audience. Do not assume an identity is familiar unless the supplied audience/organization/sender context supports it. A familiar display name with a public email domain can still increase relevance; score sender-domain spoofing separately as a cue.

situational_alignment:
How well the premise aligns with a concrete situation or event that gives the message additional familiarity or plausibility, including situations external to the workplace.

IMPORTANT: score this element only against the explicitly supplied SITUATION / EVENT CONTEXT. A generation scenario name, the fact that the email matches its own topic, or a generic workplace process is not evidence for this element. If no concrete situation/event context is supplied, this element is unresolved and the application will ignore your score. If concrete situation/event context IS supplied, do not return unresolved merely because the email does not align with it. Explicit contradiction with the supplied situation/event context is a resolvable Not applicable result and MUST receive 0. Use 2 only when the supplied situation/event context provides weak but genuine supporting alignment. Do not award 2 merely because the email is generally plausible or matches its own claimed event.

consequences_for_not_clicking:
How strongly the supplied email and context create concern about harmful ramifications of NOT taking the requested action.

Use the full applicability scale. Missing an informational notice or an unread message can evoke a weak fear of missing out and justify 2 when the message/context supports it. A concrete threat of account suspension, data loss, or other serious harm can justify 6 or 8 depending on the audience and context. Do not infer an unstated serious consequence from a routine topic alone: "requires review", "please review", "today", or a payroll/document subject does not by itself imply a payroll delay or blocked account. If there is no supported concern about inaction, return resolved=true with score=0. Do NOT return unresolved merely because consequences are absent. Use unresolved only when required source content/context is genuinely unavailable or ambiguous.

EVIDENCE AND EXPLANATIONS

- Every result must contain a non-empty explanation.
- The explanation must identify the supplied email/context evidence that justifies the score.
- If resolved is false, explain exactly which context is missing or insufficient.
- Do not invent organizational processes, policies, deadlines, events, roles, or consequences.
- Prefer a lower score over an unsupported inference.

UNKNOWN CONTEXT

If the supplied information is insufficient to justify a score for an element, set resolved to false. The score field must still contain one of the allowed numeric values because of the response schema, but the application will ignore it when resolved is false.

Do not calculate the final premise-alignment total, category, cue count, or phishing difficulty.

Return only JSON matching the supplied schema, with exactly one result for each requested element.`

func buildPremiseAlignmentPrompt(input EvaluationInput) string {
	generationContext := input.GenerationContext

	return fmt.Sprintf(
		`Evaluate the four semantic premise-alignment elements for this training email.

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

--- SITUATION / EVENT CONTEXT ---
%s
--- END SITUATION / EVENT CONTEXT ---

--- SIMULATED SENDER ---
%s
--- END SIMULATED SENDER ---

--- EXPECTED LEGITIMATE SENDER ---
%s
--- END EXPECTED LEGITIMATE SENDER ---

--- EMAIL SUBJECT ---
%s
--- END EMAIL SUBJECT ---

--- EMAIL PLAIN TEXT ---
%s
--- END EMAIL PLAIN TEXT ---

--- EMAIL HTML ---
%s
--- END EMAIL HTML ---`,
		valueOrDefault(generationContext.TargetAudience, "Not specified"),
		valueOrDefault(generationContext.RecipientRole, "Not specified"),
		valueOrDefault(generationContext.OrganizationContext, "Not specified"),
		valueOrDefault(generationContext.SenderContext, "Not specified"),
		valueOrDefault(input.EvaluationContext.SituationContext, "Not specified"),
		formatSenderIdentity(input.EvaluationContext.SimulatedSender),
		formatSenderIdentity(input.EvaluationContext.ExpectedSender),
		input.Email.Subject,
		input.Email.Text,
		input.Email.HTML,
	)
}

func effectiveScenario(req GenerationRequest) string {
	if req.Scenario == "custom" && req.CustomScenario != "" {
		return req.CustomScenario
	}
	return req.Scenario
}

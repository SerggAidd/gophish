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

situational_alignment:
How well the premise aligns with a concrete situation or event that gives the message additional familiarity or plausibility, including situations external to the workplace.

IMPORTANT: score this element only against the explicitly supplied SITUATION / EVENT CONTEXT. A generation scenario name, the fact that the email matches its own topic, or a generic workplace process is not evidence for this element. If no concrete situation/event context is supplied, this element is unresolved and the application will ignore your score.

consequences_for_not_clicking:
How strongly the supplied email and context create concern about harmful ramifications of NOT taking the requested action.

IMPORTANT: do not infer an unstated downstream consequence from the topic alone. Phrases such as "requires review", "please review", "today", or a payroll/document subject do not by themselves establish that payroll will be delayed, an account will be blocked, or another harmful outcome will occur. A score above 0 requires an explicit or clearly implied harmful ramification supported by the supplied message/context. Prefer 0 over speculation.

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

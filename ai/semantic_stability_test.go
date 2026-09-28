package ai

import (
	"strings"
	"testing"
)

func semanticFinding(id CueCriterionID, min, max int, evidence ...string) CueCriterionResult {
	return CueCriterionResult{ID: id, MinValue: min, MaxValue: max, Source: CueSourceLLM, Evidence: evidence}
}

func checkFinding(t *testing.T, result CueCriterionResult, min, max int) {
	t.Helper()
	if result.MinValue != min || result.MaxValue != max {
		t.Fatalf("%s: wanted %d..%d, got %#v", result.ID, min, max, result)
	}
	if max > 0 && len(result.Evidence) == 0 {
		t.Fatalf("%s: positive or unresolved result has no evidence", result.ID)
	}
}

func TestStabilitySenderWithoutDisplayNameDoesNotMismatch(t *testing.T) {
	context := EvaluationContext{
		SimulatedSender: &SenderIdentity{Email: "notifications@inn0corp.test"},
		ExpectedSender:  &SenderIdentity{DisplayName: "IT Service Desk", Email: "notifications@innocorp.example"},
	}
	for _, model := range []CueCriterionResult{
		semanticFinding(CriterionSenderNameMismatch, 0, 1, "Header display name not provided."),
		semanticFinding(CriterionSenderNameMismatch, 1, 1, "Expected sender has a display name, but simulated sender does not."),
		semanticFinding(CriterionSenderNameMismatch, 0, 0),
	} {
		results := stabilizeContextGroundedCues([]CueCriterionResult{model}, EvaluationInput{EvaluationContext: context})
		checkFinding(t, results[0], 0, 0)
	}
	// A contradictory Reply-To with an actual display name still needs semantic evaluation.
	context.SimulatedSender.DisplayName = "IT Service Desk"
	context.SimulatedSender.ReplyTo = "another-organization@example.test"
	result := stabilizeContextGroundedCues([]CueCriterionResult{
		semanticFinding(CriterionSenderNameMismatch, 1, 1, "Reply-To identifies a different unrelated entity."),
	}, EvaluationInput{EvaluationContext: context})
	checkFinding(t, result[0], 1, 1)
	context.SimulatedSender.ReplyTo = ""
	// Matching the expected display name alone cannot clear a genuine
	// contradiction with an unrelated From domain.
	result = stabilizeContextGroundedCues([]CueCriterionResult{
		semanticFinding(CriterionSenderNameMismatch, 1, 1, "The sender address identifies an unrelated entity."),
	}, EvaluationInput{EvaluationContext: context})
	checkFinding(t, result[0], 1, 1)
}

func TestStabilityBrandingDependsOnDocumentedExpectation(t *testing.T) {
	input := EvaluationInput{GenerationContext: GenerationRequest{SenderContext: "IT Service Desk"}}
	for _, model := range []CueCriterionResult{
		semanticFinding(CriterionMissingBranding, 1, 1, "No corporate logo."),
		semanticFinding(CriterionMissingBranding, 0, 0),
	} {
		result := stabilizeContextGroundedCues([]CueCriterionResult{model}, input)
		checkFinding(t, result[0], 0, 1)
	}
	input.GenerationContext.SenderContext = "Official VendorCloud alerts normally carry a distinctive VendorCloud heading or logo."
	result := stabilizeContextGroundedCues([]CueCriterionResult{
		semanticFinding(CriterionMissingBranding, 1, 1, "Required VendorCloud heading is absent."),
	}, input)
	checkFinding(t, result[0], 1, 1)
	input.GenerationContext.SenderContext = "Alice Smith is a familiar coworker. Ordinary personal notes do not use branded templates."
	result = stabilizeContextGroundedCues([]CueCriterionResult{
		semanticFinding(CriterionMissingBranding, 1, 1, "No logo."),
	}, input)
	checkFinding(t, result[0], 0, 0)
}

func TestStabilityGroupEmailIsNotAnIndividualSigner(t *testing.T) {
	mail := Email{Text: "Dear {{.FirstName}},\nReview the document.\n\nRegards,\nIT Service Desk\nnotifications@inn0corp.test",
		HTML: "<p>Review the document.</p><p>Regards,<br>IT Service Desk<br>notifications@inn0corp.test</p>"}
	for _, model := range []CueCriterionResult{
		semanticFinding(CriterionMissingSignerDetails, 0, 0),
		semanticFinding(CriterionMissingSignerDetails, 1, 1, "Only the group and its mailbox appear in the closing."),
	} {
		result := stabilizeContextGroundedCues([]CueCriterionResult{model}, EvaluationInput{Email: mail})
		checkFinding(t, result[0], 1, 1)
	}
	mail.Text = "Regards,\nAlice Smith\nIT Support\nalice@company.test"
	result := stabilizeContextGroundedCues([]CueCriterionResult{
		semanticFinding(CriterionMissingSignerDetails, 0, 0),
	}, EvaluationInput{Email: mail})
	checkFinding(t, result[0], 0, 0)
	mail.Text = "Regards,\nIT Service Desk\nnotifications@inn0corp.test"
	mail.HTML = "<p>Regards,<br>Alice Smith<br>IT Support<br>alice@company.test</p>"
	result = stabilizeContextGroundedCues([]CueCriterionResult{
		semanticFinding(CriterionMissingSignerDetails, 1, 1, "No individual signer in plain text."),
	}, EvaluationInput{Email: mail})
	checkFinding(t, result[0], 0, 1)
}

func TestStabilitySpellingDeduplicatesAlternativeFormats(t *testing.T) {
	mail := Email{Text: "Please check your device usgae.", HTML: "<p>Please check your device usgae.</p>"}
	for _, model := range []CueCriterionResult{
		semanticFinding(CriterionSpellingErrors, 2, 2, `"device usgae" appears twice in the email.`),
		semanticFinding(CriterionSpellingErrors, 2, 2,
			`"device usgae" in plain text.`, `"device usgae" in HTML.`),
	} {
		result := stabilizeContextGroundedCues([]CueCriterionResult{model}, EvaluationInput{Email: mail})
		checkFinding(t, result[0], 1, 1)
	}
	model := semanticFinding(CriterionSpellingErrors, 2, 2, `"device usgae" appears twice in the email.`)
	mail.Text += " Correct device usgae now."
	result := stabilizeContextGroundedCues([]CueCriterionResult{model}, EvaluationInput{Email: mail})
	checkFinding(t, result[0], 2, 2)
}

func TestStabilityDoesNotInventUrgencyFromActionOnlyOrHiddenHTML(t *testing.T) {
	mail := Email{
		Subject: "Policy review - Action Required",
		Text:    "Please review the updated policy when convenient.",
		HTML:    `<p>Please review the policy.</p><span style="display:none">URGENT: act immediately</span>`,
	}
	if expressions := explicitUrgencyExpressions(mail); len(expressions) != 0 {
		t.Fatalf("action request or hidden text must not add urgency: %v", expressions)
	}
}

func TestStabilityVisibleHTMLUrgencyAndDistinctThreats(t *testing.T) {
	mail := Email{
		Subject: "Urgent: Password reset",
		Text: "This link is only valid for the next 24 hours. If you do not act within 48 hours, your account will be disabled. If you do not reset your password, you may experience a temporary lockout. Contact IT immediately.",
	}
	mail.HTML = "<p>Urgent: Immediate action required.</p><p>" + mail.Text + "</p>"
	for _, model := range []CueCriterionResult{
		semanticFinding(CriterionTimePressure, 3, 3, "Subject urgency; 24-hour and 48-hour deadlines."),
		semanticFinding(CriterionTimePressure, 4, 4, "Subject urgency; HTML urgency; 24-hour and 48-hour deadlines."),
	} {
		result := stabilizeContextGroundedCues([]CueCriterionResult{model}, EvaluationInput{Email: mail})
		checkFinding(t, result[0], 5, 5)
	}
	for _, value := range []int{1, 2} {
		result := stabilizeContextGroundedCues([]CueCriterionResult{
			semanticFinding(CriterionThreats, value, value, "Loss of access to company systems."),
		}, EvaluationInput{Email: mail})
		checkFinding(t, result[0], 2, 2)
	}
}

func TestStabilityAdditionalLinkToSameDestinationIsNotUnrelated(t *testing.T) {
	mail := Email{
		Text: "Read the policy: {{.URL}}\nClick here for additional information: {{.URL}}",
		HTML: `<p><a href="{{.URL}}">Read policy</a></p><p><a href="{{.URL}}">Click here for additional information</a></p>`,
	}
	result := stabilizeContextGroundedCues([]CueCriterionResult{
		semanticFinding(CriterionDistractingDetails, 2, 2, "This is not a test is unrelated.",
			"Click here for additional information is an extra unrelated link."),
	}, EvaluationInput{Email: mail})
	checkFinding(t, result[0], 1, 1)
	if strings.Contains(strings.ToLower(strings.Join(result[0].Evidence, " ")), "additional information") {
		t.Fatalf("same-target link remained in distracting evidence: %v", result[0].Evidence)
	}
}

func TestStabilityCountsUrgentLinkLabelWithoutDuplicatingHTML(t *testing.T) {
	mail := Email{
		Subject: "Password Expiration Notice - Immediate Action Required",
		Text: "Password Expiration Notice - Immediate Action Required\n\n" +
			"IMPORTANT: This message requires your immediate attention.\n" +
			"Your current password will expire in 48 hours. Please reset your password via the portal by clicking the link below.\n" +
			"Resert Your Password Now: {{.URL}}\n" +
			"Please reset your password right now to avoid lockout.",
		HTML: `<body><h1>Password Expiration Notice - Immediate Action Required</h1>` +
			`<p>IMPORTANT: This message requires your immediate attention.</p>` +
			`<p>Your current password will expire in <strong>48 hours</strong>. Please reset your password via the portal by clicking the link below.</p>` +
			`<p><a href="{{.URL}}">Resert Your Password Now</a></p>` +
			`<p>Please reset your password right now to avoid lockout.</p></body>`,
	}
	expressions := explicitUrgencyExpressions(mail)
	if len(expressions) != 5 {
		t.Fatalf("want five distinct urgency expressions, got %d: %v", len(expressions), expressions)
	}
	for _, model := range []CueCriterionResult{
		semanticFinding(CriterionTimePressure, 5, 5, "Subject urgency", "Immediate attention", "Expires in 48 hours", "Act right now", "Please reset via portal by clicking below"),
		semanticFinding(CriterionTimePressure, 4, 4, "Subject urgency", "Immediate attention", "Expires in 48 hours", "Act right now"),
	} {
		result := stabilizeContextGroundedCues([]CueCriterionResult{model}, EvaluationInput{Email: mail})
		checkFinding(t, result[0], 5, 5)
	}
}

func TestStabilityBoilerplateIsNotDistractingDetail(t *testing.T) {
	mail := Email{
		Text: "Пожалуйста, подтвердите отпуск.\n\nПриносим извинения за неудобства.\n\n" +
			"Данный запрос отправлен автоматически. Если вы не запрашивали обновление, проигнорируйте письмо.",
		HTML: `<p>Пожалуйста, подтвердите отпуск.</p><p>Приносим извинения за неудобства.</p>` +
			`<p>Данный запрос отправлен автоматически. Если вы не запрашивали обновление, проигнорируйте письмо.</p>`,
	}
	for _, model := range []CueCriterionResult{
		semanticFinding(CriterionDistractingDetails, 2, 2, "Приносим извинения за неудобства.", "Данный запрос отправлен автоматически."),
		semanticFinding(CriterionDistractingDetails, 0, 0),
	} {
		result := stabilizeContextGroundedCues([]CueCriterionResult{model}, EvaluationInput{Email: mail})
		checkFinding(t, result[0], 0, 0)
	}
	result := stabilizeContextGroundedCues([]CueCriterionResult{
		semanticFinding(CriterionDistractingDetails, 2, 2, "Приносим извинения за неудобства.", "Ненужная ссылка на новости спорта."),
	}, EvaluationInput{Email: mail})
	checkFinding(t, result[0], 1, 1)
	result = stabilizeContextGroundedCues([]CueCriterionResult{
		semanticFinding(CriterionDistractingDetails, 1, 1, "Приносим извинения за неудобства."),
	}, EvaluationInput{Email: Email{Text: "Пожалуйста, подтвердите отпуск."}})
	checkFinding(t, result[0], 1, 1) // Evidence absent from the email must not be rewritten.
}

func TestStabilityDistinctUrgencySentencesDoNotCollapseOnImmediately(t *testing.T) {
	mail := Email{
		Subject: "Urgent: Your Password Expires Soon - Action Required!!!",
		Text: "Urgent: Your Password Expires Soon - Action Required!!!\n" +
			"Your password is expiring soon. Please reset it before the expiration date.\n" +
			"IMPORTANT: Please reset your password NOW!!!\n" +
			"Reset your password now: {{.URL}}\n" +
			"Reset your password now: {{.URL}}\n" +
			"Your password will expire in 3 days.\n" +
			"After resetting, your account will be reactivated immediately.\n" +
			"If you ignore this message, your account will be disabled immediately.\n" +
			"If you notice any unusual activity, report it immediately.",
		HTML: `<body><p>Your password is expiring soon. Please reset it before the expiration date.</p>` +
			`<p>IMPORTANT: Please reset your password NOW!!!</p>` +
			`<p>Reset your password now: <a href="{{.URL}}">Reset Password</a></p>` +
			`<p>Reset your password now: <a href="{{.URL}}">Reset Your Password NOW!</a></p>` +
			`<p>Your password will expire in 3 days.</p>` +
			`<p>After resetting, your account will be reactivated immediately.</p>` +
			`<p>If you ignore this message, your account will be disabled immediately.</p>` +
			`<p>If you notice any unusual activity, report it immediately.</p></body>`,
	}
	expressions := explicitUrgencyExpressions(mail)
	if len(expressions) != 6 {
		t.Fatalf("want six distinct action-related urgency expressions, got %d: %v", len(expressions), expressions)
	}
	for _, expression := range expressions {
		if strings.Contains(expression, "reactivated immediately") || strings.Contains(expression, "report it immediately") {
			t.Fatalf("non-pressuring instruction counted as urgency: %q", expression)
		}
		if strings.Contains(expression, "reset your password now: {{") && !strings.Contains(expression, "{{.URL}}") {
			t.Fatalf("template variable was split inside the urgency evidence: %q", expression)
		}
	}
	for _, count := range []int{4, 6} {
		evidence := make([]string, count)
		for i := range evidence {
			evidence[i] = "Model evidence for a visible timing statement."
		}
		result := stabilizeContextGroundedCues([]CueCriterionResult{
			semanticFinding(CriterionTimePressure, count, count, evidence...),
		}, EvaluationInput{Email: mail})
		checkFinding(t, result[0], 6, 6)
	}
	model := semanticFinding(CriterionTimePressure, 6, 6,
		"Urgent subject", "IMPORTANT: Please reset your password NOW", "Your password is expiring soon",
		"Your password will expire in 3 days", "After resetting, your account will be reactivated immediately",
		"If you notice any unusual activity, report it immediately")
	result := stabilizeContextGroundedCues([]CueCriterionResult{model}, EvaluationInput{Email: mail})
	checkFinding(t, result[0], 6, 6)
	if result[0].Source != CueSourceHybrid {
		t.Fatalf("grounded replacement should identify hybrid evidence: %#v", result[0])
	}
	for _, evidence := range result[0].Evidence {
		if strings.Contains(evidence, "reactivated immediately") || strings.Contains(evidence, "report it immediately") {
			t.Fatalf("model's unsupported urgency evidence remained: %q", evidence)
		}
	}
}

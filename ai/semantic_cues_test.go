package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSemanticCueResponse(t *testing.T) {
	modelResults := validSemanticCueModelResults()
	setSemanticModelResult(t, modelResults, CriterionTimePressure, 2, 2, []string{"Two urgent expressions."})
	setSemanticModelResult(t, modelResults, CriterionMissingBranding, 0, 1, []string{"Organization branding expectations are not fully specified."})

	results, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, modelResults))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byID := make(map[CueCriterionID]CueCriterionResult)
	for _, result := range results {
		byID[result.ID] = result
	}
	if byID[CriterionTimePressure].MinValue != 2 || byID[CriterionTimePressure].MaxValue != 2 {
		t.Fatalf("unexpected time-pressure result: %#v", byID[CriterionTimePressure])
	}
	if byID[CriterionMissingBranding].MinValue != 0 || byID[CriterionMissingBranding].MaxValue != 1 {
		t.Fatalf("unexpected branding range: %#v", byID[CriterionMissingBranding])
	}
}

func TestParseSemanticCueResponseMissingCriterion(t *testing.T) {
	results := validSemanticCueModelResults()
	delete(results, CriterionLimitedTimeOffers)
	_, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err == nil {
		t.Fatal("expected missing-criterion error")
	}
}

func TestParseSemanticCueResponseUnexpectedCriterion(t *testing.T) {
	results := validSemanticCueModelResults()
	delete(results, CriterionLimitedTimeOffers)
	results[CriterionMissingGreeting] = semanticCueModelResult{}
	_, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err == nil {
		t.Fatal("expected unexpected-criterion error")
	}
}

func TestParseSemanticCueResponseInvalidBinaryRange(t *testing.T) {
	results := validSemanticCueModelResults()
	setSemanticModelResult(t, results, CriterionMimicsBusinessProcess, 0, 2, []string{"Invalid range."})
	_, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err == nil {
		t.Fatal("expected binary range validation error")
	}
}

func TestParseSemanticCueResponseExactPositiveWithoutEvidence(t *testing.T) {
	results := validSemanticCueModelResults()
	setSemanticModelResult(t, results, CriterionThreats, 1, 1, nil)
	_, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err == nil {
		t.Fatal("expected evidence error")
	}
}

func TestParseSemanticCueResponseUnresolvedWithoutEvidenceUsesFallback(t *testing.T) {
	results := validSemanticCueModelResults()
	setSemanticModelResult(t, results, CriterionThreats, 0, 1, nil)

	parsed, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, result := range parsed {
		if result.ID != CriterionThreats {
			continue
		}
		if result.MinValue != 0 || result.MaxValue != 1 || len(result.Evidence) == 0 {
			t.Fatalf("unexpected unresolved result: %#v", result)
		}
		return
	}

	t.Fatal("threats criterion not found")
}

func TestSemanticCueCriteriaCoverAllNonDeterministicCriteria(t *testing.T) {
	deterministic := map[CueCriterionID]bool{
		CriterionAttachments:            true,
		CriterionMissingGreeting:        true,
		CriterionMissingPersonalization: true,
		CriterionHiddenURLLinks:         true,
	}
	semantic := make(map[CueCriterionID]bool)
	for _, id := range semanticCueCriterionIDs {
		semantic[id] = true
	}
	for _, definition := range NISTCueCriteria {
		if !deterministic[definition.ID] && !semantic[definition.ID] {
			t.Fatalf("criterion %q is not assigned to a detector", definition.ID)
		}
	}
}

func TestParseSemanticCueResponseMarksDomainSpoofingHybrid(t *testing.T) {
	results := validSemanticCueModelResults()
	setSemanticModelResult(t, results, CriterionSenderDomainSpoofing, 1, 1, []string{"rncrosoft.com plausibly imitates microsoft.com."})
	setSemanticModelResult(t, results, CriterionSpoofedLinkDomains, 1, 1, []string{"login.rncrosoft.com plausibly imitates microsoft.com."})

	parsed, err := parseSemanticCueResponse(marshalSemanticCueResponse(t, results))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byID := make(map[CueCriterionID]CueCriterionResult)
	for _, result := range parsed {
		byID[result.ID] = result
	}

	for _, id := range []CueCriterionID{CriterionSenderDomainSpoofing, CriterionSpoofedLinkDomains} {
		if byID[id].Source != CueSourceHybrid {
			t.Fatalf("criterion %q: expected hybrid source, got %q", id, byID[id].Source)
		}
	}
}

func TestFormatDomainComparisonContext(t *testing.T) {
	context := EvaluationContext{
		SimulatedSender: &SenderIdentity{Email: "support@rncrosoft.com"},
		ExpectedSender:  &SenderIdentity{Email: "support@microsoft.com"},
		Link: LinkContext{
			Usage:          LinkUsageUsed,
			SimulatedURL:   "https://login.rncrosoft.com/path",
			ExpectedDomain: "microsoft.com",
		},
	}

	value := formatDomainComparisonContext(context)
	for _, expected := range []string{"rncrosoft.com", "microsoft.com", "different"} {
		if !strings.Contains(value, expected) {
			t.Fatalf("expected %q in domain comparison context: %s", expected, value)
		}
	}
}

func validSemanticCueModelResults() map[CueCriterionID]semanticCueModelResult {
	results := make(map[CueCriterionID]semanticCueModelResult, len(semanticCueCriterionIDs))
	for _, id := range semanticCueCriterionIDs {
		results[id] = semanticCueModelResult{MinValue: 0, MaxValue: 0, Evidence: []string{}}
	}
	return results
}

func setSemanticModelResult(t *testing.T, results map[CueCriterionID]semanticCueModelResult, id CueCriterionID, minValue, maxValue int, evidence []string) {
	t.Helper()
	if _, ok := results[id]; !ok {
		t.Fatalf("criterion %q not found", id)
	}
	results[id] = semanticCueModelResult{MinValue: minValue, MaxValue: maxValue, Evidence: evidence}
}

func marshalSemanticCueResponse(t *testing.T, results map[CueCriterionID]semanticCueModelResult) string {
	t.Helper()
	data, err := json.Marshal(semanticCueModelResponse{Results: results})
	if err != nil {
		t.Fatalf("marshal semantic response: %v", err)
	}
	return string(data)
}

func TestApplySemanticCueContextRulesSuppressesLocalPartOnlySenderMismatch(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:       CriterionSenderNameMismatch,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceLLM,
			Evidence: []string{"Display name 'Vendor Support' does not match local part 'alerts'."},
		},
	}

	updated := applySemanticCueContextRules(results, EvaluationInput{})
	if updated[0].MinValue != 0 || updated[0].MaxValue != 0 || len(updated[0].Evidence) != 0 {
		t.Fatalf("expected local-part-only mismatch to be suppressed, got %#v", updated[0])
	}
}

func TestApplySemanticCueContextRulesKeepsClearIdentityConflict(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:       CriterionSenderNameMismatch,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceLLM,
			Evidence: []string{"Display name claims Microsoft Support, but Reply-To points to a different unrelated entity."},
		},
	}

	updated := applySemanticCueContextRules(results, EvaluationInput{})
	if updated[0].MinValue != 1 || updated[0].MaxValue != 1 {
		t.Fatalf("expected clear identity conflict to remain positive, got %#v", updated[0])
	}
}

func TestApplySemanticCueContextRulesAddsAttachmentTypeInconsistency(t *testing.T) {
	results := []CueCriterionResult{
		{ID: CriterionInconsistencies, MinValue: 0, MaxValue: 0, Source: CueSourceLLM},
	}
	input := EvaluationInput{
		Email: Email{
			Text: "The attached PDF contains the payroll review form. Please open the PDF.",
		},
		EvaluationContext: EvaluationContext{
			Attachments: AttachmentContext{
				Usage: AttachmentUsageUsed,
				Files: []AttachmentMetadata{{Name: "Payroll_Review.zip", Type: "application/zip"}},
			},
		},
	}

	updated := applySemanticCueContextRules(results, input)
	if updated[0].MinValue != 1 || updated[0].MaxValue != 1 {
		t.Fatalf("expected attachment inconsistency to add one cue, got %#v", updated[0])
	}
	if len(updated[0].Evidence) == 0 || !strings.Contains(updated[0].Evidence[0], "Payroll_Review.zip") {
		t.Fatalf("expected attachment mismatch evidence, got %#v", updated[0].Evidence)
	}
	if updated[0].Source != CueSourceHybrid {
		t.Fatalf("expected attachment/body inconsistency to be marked hybrid, got %q", updated[0].Source)
	}
}

func TestApplySemanticCueContextRulesSuppressesBareSpellingEvidence(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:       CriterionSpellingErrors,
			MinValue: 2,
			MaxValue: 2,
			Source:   CueSourceLLM,
			Evidence: []string{`"password"`, `"resources"`},
		},
	}

	updated := applySemanticCueContextRules(results, EvaluationInput{})
	if updated[0].MinValue != 0 || updated[0].MaxValue != 0 || len(updated[0].Evidence) != 0 {
		t.Fatalf("expected unsubstantiated bare spelling evidence to be suppressed, got %#v", updated[0])
	}
}

func TestApplySemanticCueContextRulesKeepsExplainedSpellingError(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:       CriterionSpellingErrors,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceLLM,
			Evidence: []string{`Visible word "pasword" is misspelled; it should be "password".`},
		},
	}

	updated := applySemanticCueContextRules(results, EvaluationInput{})
	if updated[0].MinValue != 1 || updated[0].MaxValue != 1 {
		t.Fatalf("expected explained spelling error to remain positive, got %#v", updated[0])
	}
}

func TestApplySemanticCueContextRulesSuppressesHiddenDistractingDetail(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:       CriterionDistractingDetails,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceLLM,
			Evidence: []string{`Hidden spam comment in <div style="display:none"> is a distracting detail.`},
		},
	}

	updated := applySemanticCueContextRules(results, EvaluationInput{})
	if updated[0].MinValue != 0 || updated[0].MaxValue != 0 || len(updated[0].Evidence) != 0 {
		t.Fatalf("expected hidden implementation detail to be suppressed, got %#v", updated[0])
	}
}

func TestApplySemanticCueContextRulesKeepsVisibleDistractingDetail(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:       CriterionDistractingDetails,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceLLM,
			Evidence: []string{`Visible footer contains an unrelated paragraph about an office event.`},
		},
	}

	updated := applySemanticCueContextRules(results, EvaluationInput{})
	if updated[0].MinValue != 1 || updated[0].MaxValue != 1 {
		t.Fatalf("expected visible distracting detail to remain positive, got %#v", updated[0])
	}
}

func TestApplySemanticCueContextRulesSuppressesDomainOnlyInconsistencyWhenSpoofingAlreadyDetected(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:       CriterionSenderDomainSpoofing,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceHybrid,
			Evidence: []string{"inn0corp.test plausibly imitates innocorp.example."},
		},
		{
			ID:       CriterionSpoofedLinkDomains,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceHybrid,
			Evidence: []string{"sso.inn0corp.test plausibly imitates innocorp.example."},
		},
		{
			ID:       CriterionInconsistencies,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceLLM,
			Evidence: []string{"The sender domain inn0corp.test differs from the link domain sso.inn0corp.test and expected legitimate domain innocorp.example."},
		},
	}

	updated := applySemanticCueContextRules(results, EvaluationInput{})
	if updated[2].MinValue != 0 || updated[2].MaxValue != 0 || len(updated[2].Evidence) != 0 {
		t.Fatalf("expected generic domain-only inconsistency to be suppressed, got %#v", updated[2])
	}
}

func TestApplySemanticCueContextRulesKeepsNonDomainInconsistency(t *testing.T) {
	results := []CueCriterionResult{
		{
			ID:       CriterionSenderDomainSpoofing,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceHybrid,
			Evidence: []string{"inn0corp.test plausibly imitates innocorp.example."},
		},
		{
			ID:       CriterionInconsistencies,
			MinValue: 1,
			MaxValue: 1,
			Source:   CueSourceLLM,
			Evidence: []string{"The subject says the request is optional, but the body says it is mandatory."},
		},
	}

	updated := applySemanticCueContextRules(results, EvaluationInput{})
	if updated[1].MinValue != 1 || updated[1].MaxValue != 1 {
		t.Fatalf("expected non-domain inconsistency to remain positive, got %#v", updated[1])
	}
}

func TestApplySemanticCueContextRulesCompletedReviewIsNotContradictory(t *testing.T) {
	const subject = "Policy Review Request - Action Required"
	const afterReview = "Please review the policy using the link below. After completing the review, no further action is needed."
	const falseEvidence = `Subject "Policy Review Request - Action Required" conflicts with body statement "After completing the review, no further action is needed."`
	tests := []struct {
		name     string
		text     string
		evidence []string
		value    int
	}{
		{"completed review", afterReview, []string{falseEvidence}, 0},
		{"unqualified cancellation", "Please review the policy using the link below. No further action required.", []string{`Subject "Policy Review Request - Action Required" conflicts with "No further action required."`}, 1},
		{"separate cancellation", "Please review the policy using the link below. No further action required. After completing the review, no further action is needed.", []string{falseEvidence}, 1},
		{"independent contradiction", afterReview + " Reviewing the policy is optional.", []string{`The subject says Action Required, while the body calls the review optional.`}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []CueCriterionResult{{
				ID: CriterionInconsistencies, MinValue: 1, MaxValue: 1,
				Source: CueSourceLLM, Evidence: tt.evidence,
			}}
			input := EvaluationInput{Email: Email{Subject: subject, Text: tt.text}}
			updated := applySemanticCueContextRules(results, input)
			if updated[0].MinValue != tt.value || updated[0].MaxValue != tt.value {
				t.Fatalf("expected inconsistency count %d, got %#v", tt.value, updated[0])
			}
			if tt.value == 0 && len(updated[0].Evidence) != 0 {
				t.Fatalf("expected no evidence for a resolved zero, got %#v", updated[0].Evidence)
			}
		})
	}
}

func TestApplySemanticCueContextRulesClearlyMissingSignerDetails(t *testing.T) {
	tests := []struct {
		name       string
		email      Email
		modelValue int
		want       int
	}{
		{
			name:  "no signature despite sender header",
			email: Email{Text: "Hello {{.FirstName}},\n\nPlease review the document."},
			want:  1,
		},
		{
			name:       "preserve model's correct missing signature",
			email:      Email{Text: "Hello {{.FirstName}},\n\nPlease review the document."},
			modelValue: 1,
			want:       1,
		},
		{
			name:  "signed plain text",
			email: Email{Text: "Hello {{.FirstName}},\n\nPlease review the document.\n\nRegards,\nAlice Smith\nIT Support\nalice@company.test"},
			want:  0,
		},
		{
			name: "signed HTML alternative",
			email: Email{
				Text: "Hello {{.FirstName}},\n\nPlease review the document.",
				HTML: "<p>Hello {{.FirstName}},</p><p>Please review the document.</p><p>Regards,<br>Alice Smith<br>IT Support</p>",
			},
			want: 0,
		},
		{
			name:  "name and job title without sign-off",
			email: Email{Text: "Hello {{.FirstName}},\n\nPlease review the document.\nAlice Smith\nIT Support"},
			want:  0,
		},
		{
			name: "hidden HTML contact is not a signature",
			email: Email{
				Text: "Hello {{.FirstName}},\n\nPlease review the document.",
				HTML: "<p>Hello {{.FirstName}},</p><p>Please review the document.</p><div style=\"display:none\">Regards, Alice Smith, alice@company.test</div>",
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []CueCriterionResult{{
				ID:       CriterionMissingSignerDetails,
				MinValue: tt.modelValue,
				MaxValue: tt.modelValue,
				Source:   CueSourceLLM,
			}}
			input := EvaluationInput{
				Email: tt.email,
				EvaluationContext: EvaluationContext{
					SimulatedSender: &SenderIdentity{DisplayName: "Alice Smith", Email: "alice@company.test"},
				},
			}
			updated := applySemanticCueContextRules(results, input)
			if updated[0].MinValue != tt.want || updated[0].MaxValue != tt.want {
				t.Fatalf("expected missing signer details %d, got %#v", tt.want, updated[0])
			}
			if tt.modelValue == 0 && tt.want == 1 {
				if updated[0].Source != CueSourceHybrid || len(updated[0].Evidence) == 0 {
					t.Fatalf("expected a sourced correction with evidence, got %#v", updated[0])
				}
			}
		})
	}
}

package ai

import "testing"

func TestEvaluationContextZeroValueIsValidAndIncomplete(t *testing.T) {
	var evaluationContext EvaluationContext

	if err := evaluationContext.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	missing := evaluationContext.MissingFields()

	expected := map[string]bool{
		"prior_training_exposure": true,
		"simulated_sender":        true,
		"expected_sender":         true,
		"situation_context":       true,
		"link":                    true,
		"attachments":             true,
	}

	if len(missing) != len(expected) {
		t.Fatalf("expected %d missing fields, got %d: %v", len(expected), len(missing), missing)
	}

	for _, field := range missing {
		if !expected[field] {
			t.Fatalf("unexpected missing field %q", field)
		}
	}
}

func TestEvaluationContextComplete(t *testing.T) {
	evaluationContext := EvaluationContext{
		PriorTrainingExposure: TrainingExposureModerate,
		SimulatedSender: &SenderIdentity{
			DisplayName: "Microsoft Support",
			Email:       "support@rncrosoft.com",
		},
		ExpectedSender: &SenderIdentity{
			DisplayName: "Microsoft Support",
			Email:       "support@microsoft.com",
		},
		SituationContext: "The finance team is currently reviewing payroll documents for the current pay cycle.",
		Link: LinkContext{
			Usage:          LinkUsageUsed,
			SimulatedURL:   "https://login.rncrosoft.com",
			ExpectedDomain: "microsoft.com",
		},
		Attachments: AttachmentContext{
			Usage: AttachmentUsageUsed,
			Files: []AttachmentMetadata{
				{
					Name: "invoice.pdf",
					Type: "application/pdf",
				},
			},
		},
	}

	if err := evaluationContext.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if missing := evaluationContext.MissingFields(); len(missing) != 0 {
		t.Fatalf("expected complete context, got missing fields: %v", missing)
	}
}

func TestEvaluationContextExplicitlyNoLinkAndNoAttachments(t *testing.T) {
	evaluationContext := EvaluationContext{
		PriorTrainingExposure: TrainingExposureNone,
		SimulatedSender: &SenderIdentity{
			Email: "training@example.test",
		},
		ExpectedSender: &SenderIdentity{
			Email: "training@example.test",
		},
		SituationContext: "A routine internal service notice is currently expected.",
		Link: LinkContext{
			Usage: LinkUsageNone,
		},
		Attachments: AttachmentContext{
			Usage: AttachmentUsageNone,
		},
	}

	if err := evaluationContext.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if missing := evaluationContext.MissingFields(); len(missing) != 0 {
		t.Fatalf("expected complete context, got missing fields: %v", missing)
	}
}

func TestEvaluationContextPartialLinkIsValidButIncomplete(t *testing.T) {
	evaluationContext := EvaluationContext{
		Link: LinkContext{
			Usage:        LinkUsageUsed,
			SimulatedURL: "https://login.example.test",
		},
	}

	if err := evaluationContext.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	missing := evaluationContext.MissingFields()
	foundExpectedDomain := false

	for _, field := range missing {
		if field == "expected_link_domain" {
			foundExpectedDomain = true
		}
	}

	if !foundExpectedDomain {
		t.Fatalf("expected missing expected_link_domain, got %v", missing)
	}
}

func TestEvaluationContextRejectsInvalidTrainingExposure(t *testing.T) {
	evaluationContext := EvaluationContext{
		PriorTrainingExposure: TrainingExposure("invalid"),
	}

	if err := evaluationContext.Validate(); err == nil {
		t.Fatal("expected error for invalid training exposure")
	}
}

func TestEvaluationContextRejectsLinkValuesWhenLinkNotUsed(t *testing.T) {
	evaluationContext := EvaluationContext{
		Link: LinkContext{
			Usage:        LinkUsageNone,
			SimulatedURL: "https://example.test",
		},
	}

	if err := evaluationContext.Validate(); err == nil {
		t.Fatal("expected error for link values when link usage is none")
	}
}

func TestEvaluationContextRejectsAttachmentsWhenUsageUnknown(t *testing.T) {
	evaluationContext := EvaluationContext{
		Attachments: AttachmentContext{
			Files: []AttachmentMetadata{
				{Name: "invoice.pdf"},
			},
		},
	}

	if err := evaluationContext.Validate(); err == nil {
		t.Fatal("expected error for files with unknown attachment usage")
	}
}

func TestEvaluationContextRejectsUsedAttachmentsWithoutFiles(t *testing.T) {
	evaluationContext := EvaluationContext{
		Attachments: AttachmentContext{
			Usage: AttachmentUsageUsed,
		},
	}

	if err := evaluationContext.Validate(); err == nil {
		t.Fatal("expected error for used attachment context without files")
	}
}

func TestEvaluationContextRejectsInvalidSenderEmail(t *testing.T) {
	context := EvaluationContext{
		SimulatedSender: &SenderIdentity{Email: "not-an-email"},
	}
	if err := context.Validate(); err == nil {
		t.Fatal("expected invalid sender email error")
	}
}

func TestEvaluationContextRejectsInvalidReplyTo(t *testing.T) {
	context := EvaluationContext{
		SimulatedSender: &SenderIdentity{Email: "valid@example.com", ReplyTo: "invalid"},
	}
	if err := context.Validate(); err == nil {
		t.Fatal("expected invalid reply-to error")
	}
}

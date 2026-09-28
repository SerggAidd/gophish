package ai

import (
	"strings"
	"testing"
)

func TestGenerationPromptOmitsEvaluatorOnlyExpectedIdentity(t *testing.T) {
	context := EvaluationContext{
		SimulatedSender: &SenderIdentity{DisplayName: "IT Service Desk", Email: "notifications@inn0corp.test"},
		ExpectedSender:  &SenderIdentity{DisplayName: "IT Service Desk", Email: "notifications@innocorp.example"},
		Link: LinkContext{
			Usage:          LinkUsageUsed,
			SimulatedURL:   "https://login.inn0corp.test/reset",
			ExpectedDomain: "innocorp.example",
		},
	}

	prompt := buildGenerationPromptWithContext(GenerationRequest{TargetAudience: "Employees"}, &context)
	if strings.Contains(prompt, "notifications@innocorp.example") {
		t.Fatalf("generation prompt leaked expected sender value: %s", prompt)
	}
	if strings.Contains(prompt, "innocorp.example") {
		t.Fatalf("generation prompt leaked expected legitimate domain: %s", prompt)
	}
	if !strings.Contains(prompt, "notifications@inn0corp.test") {
		t.Fatal("generation prompt should retain simulated sender")
	}
}

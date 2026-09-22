package ai

import "testing"

func TestDetectCueCriteria(t *testing.T) {
	email := Email{
		Subject: "Document review required",
		Text:    "Your document is ready for review.",
		HTML: `
			<p>Your document is ready for review.</p>
			<p><a href="{{.URL}}">Review document</a></p>
		`,
	}

	results, err := DetectCueCriteria(email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf(
			"expected 3 deterministic criterion results, got %d",
			len(results),
		)
	}

	byID := make(map[CueCriterionID]CueCriterionResult)
	for _, result := range results {
		byID[result.ID] = result
	}

	if byID[CriterionMissingGreeting].Value != 1 {
		t.Fatalf(
			"expected missing greeting value 1, got %d",
			byID[CriterionMissingGreeting].Value,
		)
	}

	if byID[CriterionMissingPersonalization].Value != 1 {
		t.Fatalf(
			"expected missing personalization value 1, got %d",
			byID[CriterionMissingPersonalization].Value,
		)
	}

	if byID[CriterionHiddenURLLinks].Value != 1 {
		t.Fatalf(
			"expected hidden URL links value 1, got %d",
			byID[CriterionHiddenURLLinks].Value,
		)
	}

	cueResults, err := BuildCueResults(results)
	if err != nil {
		t.Fatalf("unexpected aggregation error: %v", err)
	}

	cues := make(map[CueID]CueResult)
	for _, result := range cueResults {
		cues[result.ID] = result
	}

	genericGreeting, exists := cues[CueGenericGreeting]
	if !exists {
		t.Fatal("expected generic greeting cue result")
	}

	if genericGreeting.Count != 2 {
		t.Fatalf(
			"expected generic greeting count 2, got %d",
			genericGreeting.Count,
		)
	}

	urlHyperlinking, exists := cues[CueURLHyperlinking]
	if !exists {
		t.Fatal("expected URL hyperlinking cue result")
	}

	if urlHyperlinking.Count != 1 {
		t.Fatalf(
			"expected URL hyperlinking count 1, got %d",
			urlHyperlinking.Count,
		)
	}
}

func TestDetectCueCriteriaNoObservedCues(t *testing.T) {
	email := Email{
		Subject: "Information for {{.FirstName}}",
		Text:    "Hello {{.FirstName}},\nPlease see https://example.com.",
		HTML: `
			<p>Hello {{.FirstName}},</p>
			<p><a href="https://example.com">https://example.com</a></p>
		`,
	}

	results, err := DetectCueCriteria(email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cueResults, err := BuildCueResults(results)
	if err != nil {
		t.Fatalf("unexpected aggregation error: %v", err)
	}

	if len(cueResults) != 0 {
		t.Fatalf(
			"expected no observed cues, got %d",
			len(cueResults),
		)
	}
}

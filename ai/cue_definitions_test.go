package ai

import "testing"

func TestNISTCueDefinitions(t *testing.T) {
	if len(NISTCueDefinitions) != 23 {
		t.Fatalf(
			"expected 23 NIST cue definitions, got %d",
			len(NISTCueDefinitions),
		)
	}

	seen := make(map[CueID]struct{})

	for _, definition := range NISTCueDefinitions {
		if definition.ID == "" {
			t.Fatal("cue definition has empty ID")
		}

		if definition.Name == "" {
			t.Fatalf("cue %q has empty name", definition.ID)
		}

		if definition.Type == "" {
			t.Fatalf("cue %q has empty type", definition.ID)
		}

		if _, exists := seen[definition.ID]; exists {
			t.Fatalf("duplicate cue ID: %q", definition.ID)
		}

		seen[definition.ID] = struct{}{}
	}
}

package main

// Tests for the models.dev capability registry parsing and lookup (models.go).
// Network fetch/cache are not exercised here — only the pure parse + lookup, which
// is what decides whether the preflight warning fires.

import "testing"

const modelsFixture = `{
  "openai": {
    "models": {
      "gpt-4o-mini":   {"structured_output": true,  "temperature": true},
      "gpt-3.5-turbo": {"structured_output": false, "temperature": true},
      "o3-mini":       {"structured_output": true,  "temperature": false}
    }
  },
  "anthropic": {
    "models": {
      "gpt-4o-mini": {"structured_output": false, "temperature": false}
    }
  }
}`

func Test_parseModelsRegistry(t *testing.T) {
	reg, err := parseModelsRegistry([]byte(modelsFixture))
	if err != nil {
		t.Fatalf("parseModelsRegistry failed: %v", err)
	}
	if len(reg) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(reg))
	}
	if _, err := parseModelsRegistry([]byte("not json")); err == nil {
		t.Error("expected error on malformed JSON, got nil")
	}
}

func Test_lookupModel(t *testing.T) {
	reg, err := parseModelsRegistry([]byte(modelsFixture))
	if err != nil {
		t.Fatalf("parseModelsRegistry failed: %v", err)
	}

	cases := []struct {
		name            string
		id              string
		wantFound       bool
		wantStructured  bool
		wantTemperature bool
	}{
		{"default supports structured", "gpt-4o-mini", true, true, true},
		{"legacy lacks structured", "gpt-3.5-turbo", true, false, true},
		{"o-series rejects temperature", "o3-mini", true, true, false},
		{"unknown model not found", "made-up-model", false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, found := lookupModel(reg, c.id)
			if found != c.wantFound {
				t.Fatalf("lookupModel(%q) found = %v, want %v", c.id, found, c.wantFound)
			}
			if !found {
				return
			}
			if m.StructuredOutput != c.wantStructured {
				t.Errorf("lookupModel(%q).StructuredOutput = %v, want %v", c.id, m.StructuredOutput, c.wantStructured)
			}
			if m.Temperature != c.wantTemperature {
				t.Errorf("lookupModel(%q).Temperature = %v, want %v", c.id, m.Temperature, c.wantTemperature)
			}
		})
	}
}

func Test_lookupModel_prefersOpenAI(t *testing.T) {
	// gpt-4o-mini exists under both providers with opposite flags; the openai entry
	// must win so a collision with another provider can't flip the verdict.
	reg, err := parseModelsRegistry([]byte(modelsFixture))
	if err != nil {
		t.Fatalf("parseModelsRegistry failed: %v", err)
	}
	m, found := lookupModel(reg, "gpt-4o-mini")
	if !found || !m.StructuredOutput {
		t.Errorf("expected openai gpt-4o-mini (structured_output=true) to win, got found=%v structured=%v", found, m.StructuredOutput)
	}
}

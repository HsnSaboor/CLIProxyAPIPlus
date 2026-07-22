package thinking

import "testing"

// TestParseSuffix_StripsContextMarker verifies that the proxy-local "[1m]"
// context-window marker (used by Claude Code style clients, e.g.
// ANTHROPIC_DEFAULT_SONNET_MODEL="sonnet[1m]") is stripped from the model
// name before provider-routing lookups and before any upstream request is
// constructed, and that it never leaks upstream as part of the model ID.
func TestParseSuffix_StripsContextMarker(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		wantModelName string
		wantHasSuffix bool
		wantRawSuffix string
	}{
		{
			name:          "bare alias with context marker",
			model:         "sonnet[1m]",
			wantModelName: "sonnet",
			wantHasSuffix: false,
		},
		{
			name:          "full model name with context marker",
			model:         "databricks-claude-sonnet-5[1m]",
			wantModelName: "databricks-claude-sonnet-5",
			wantHasSuffix: false,
		},
		{
			name:          "context marker is case-insensitive",
			model:         "sonnet[1M]",
			wantModelName: "sonnet",
			wantHasSuffix: false,
		},
		{
			name:          "context marker combined with thinking-budget suffix",
			model:         "claude-sonnet-4-5[1m](16384)",
			wantModelName: "claude-sonnet-4-5",
			wantHasSuffix: true,
			wantRawSuffix: "16384",
		},
		{
			name:          "context marker combined with level suffix",
			model:         "sonnet[1m](high)",
			wantModelName: "sonnet",
			wantHasSuffix: true,
			wantRawSuffix: "high",
		},
		{
			name:          "no context marker, unaffected",
			model:         "sonnet",
			wantModelName: "sonnet",
			wantHasSuffix: false,
		},
		{
			name:          "no context marker, with thinking suffix, unaffected",
			model:         "claude-sonnet-4-5(16384)",
			wantModelName: "claude-sonnet-4-5",
			wantHasSuffix: true,
			wantRawSuffix: "16384",
		},
		{
			name:          "model name literally ends in 1m but not bracketed - not stripped",
			model:         "some-model-1m",
			wantModelName: "some-model-1m",
			wantHasSuffix: false,
		},
		{
			name:          "empty string",
			model:         "",
			wantModelName: "",
			wantHasSuffix: false,
		},
		{
			name:          "only the marker itself",
			model:         "[1m]",
			wantModelName: "",
			wantHasSuffix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSuffix(tt.model)
			if got.ModelName != tt.wantModelName {
				t.Errorf("ParseSuffix(%q).ModelName = %q, want %q", tt.model, got.ModelName, tt.wantModelName)
			}
			if got.HasSuffix != tt.wantHasSuffix {
				t.Errorf("ParseSuffix(%q).HasSuffix = %v, want %v", tt.model, got.HasSuffix, tt.wantHasSuffix)
			}
			if tt.wantHasSuffix && got.RawSuffix != tt.wantRawSuffix {
				t.Errorf("ParseSuffix(%q).RawSuffix = %q, want %q", tt.model, got.RawSuffix, tt.wantRawSuffix)
			}
		})
	}
}

// TestStripContextMarkerSuffix exercises the helper directly for edge cases
// not covered by end-to-end ParseSuffix expectations above.
func TestStripContextMarkerSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sonnet[1m]", "sonnet"},
		{"SONNET[1M]", "SONNET"},
		{"sonnet[1m", "sonnet[1m"},   // missing closing bracket, not stripped
		{"[1m]", ""},                  // marker only
		{"1m]", "1m]"},                 // missing opening bracket, not stripped
		{"", ""},
		{"m]", "m]"}, // shorter than marker, unaffected
	}
	for _, tt := range tests {
		if got := stripContextMarkerSuffix(tt.input); got != tt.want {
			t.Errorf("stripContextMarkerSuffix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

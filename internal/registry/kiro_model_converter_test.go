package registry

import (
	"strings"
	"testing"
)

func TestConvertKiroAPIModels_SetsExecutionTarget(t *testing.T) {
	models := ConvertKiroAPIModels([]*KiroAPIModel{
		{ModelID: "claude-sonnet-4.5", ModelName: "Claude Sonnet 4.5"},
	})
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].ExecutionTarget != "claude-sonnet-4.5" {
		t.Errorf("ExecutionTarget = %q, want %q", models[0].ExecutionTarget, "claude-sonnet-4.5")
	}
	if models[0].ID != "kiro-claude-sonnet-4-5" {
		t.Errorf("ID = %q, want %q", models[0].ID, "kiro-claude-sonnet-4-5")
	}
}

func TestConvertKiroAPIModels_ContextLengthFromAPI(t *testing.T) {
	models := ConvertKiroAPIModels([]*KiroAPIModel{
		{ModelID: "gpt-4", MaxInputTokens: 8192},
		{ModelID: "claude-sonnet-4.5", MaxInputTokens: 0},
	})
	if models[0].ContextLength != 8192 {
		t.Errorf("ContextLength = %d, want 8192", models[0].ContextLength)
	}
	if models[1].ContextLength != DefaultKiroContextLength {
		t.Errorf("ContextLength = %d, want %d", models[1].ContextLength, DefaultKiroContextLength)
	}
}

func TestNormalizeKiroModelID_Idempotent(t *testing.T) {
	cases := []struct{ in, want string }{
		{"claude-sonnet-4.5", "kiro-claude-sonnet-4-5"},
		{"kiro-claude-sonnet-4-5", "kiro-claude-sonnet-4-5"}, // already normalized
		{"auto", "kiro-auto"},
	}
	for _, c := range cases {
		got := normalizeKiroModelID(c.in)
		if got != c.want {
			t.Errorf("normalizeKiroModelID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMergeWithStaticMetadata_PreservesExecutionTarget(t *testing.T) {
	dynamic := []*ModelInfo{
		{ID: "kiro-claude-sonnet-4-5", ExecutionTarget: "claude-sonnet-4.5", ContextLength: 100000},
	}
	static := []*ModelInfo{
		{ID: "kiro-claude-sonnet-4-5", ExecutionTarget: "", ContextLength: 200000, MaxCompletionTokens: 64000},
	}
	merged := MergeWithStaticMetadata(dynamic, static)
	if len(merged) != 1 {
		t.Fatalf("expected 1 model, got %d", len(merged))
	}
	if merged[0].ExecutionTarget != "claude-sonnet-4.5" {
		t.Errorf("ExecutionTarget = %q, want %q", merged[0].ExecutionTarget, "claude-sonnet-4.5")
	}
	// Static metadata wins for other fields
	if merged[0].ContextLength != 200000 {
		t.Errorf("ContextLength = %d, want 200000 (static wins)", merged[0].ContextLength)
	}
}

func TestMergeWithStaticMetadata_StaticExecutionTargetNotOverwritten(t *testing.T) {
	dynamic := []*ModelInfo{
		{ID: "kiro-auto", ExecutionTarget: "auto-dynamic"},
	}
	static := []*ModelInfo{
		{ID: "kiro-auto", ExecutionTarget: "auto-static"},
	}
	merged := MergeWithStaticMetadata(dynamic, static)
	if merged[0].ExecutionTarget != "auto-static" {
		t.Errorf("ExecutionTarget = %q, want %q (static wins when set)", merged[0].ExecutionTarget, "auto-static")
	}
}

func TestGenerateAgenticVariants_CopiesExecutionTarget(t *testing.T) {
	base := []*ModelInfo{
		{ID: "kiro-claude-sonnet-4-5", ExecutionTarget: "claude-sonnet-4.5", Object: "model", OwnedBy: "aws", Type: "kiro"},
	}
	result := GenerateAgenticVariants(base)
	var agentic *ModelInfo
	for _, m := range result {
		if strings.HasSuffix(m.ID, "-agentic") {
			agentic = m
			break
		}
	}
	if agentic == nil {
		t.Fatal("expected agentic variant to be generated")
	}
	if agentic.ExecutionTarget != "claude-sonnet-4.5" {
		t.Errorf("agentic ExecutionTarget = %q, want %q", agentic.ExecutionTarget, "claude-sonnet-4.5")
	}
}

func TestFullPipeline_ExecutionTargetSurvives(t *testing.T) {
	// Simulate: API returns claude-sonnet-4.5, static has kiro-claude-sonnet-4-5 with no ExecutionTarget.
	apiModels := []*KiroAPIModel{
		{ModelID: "claude-sonnet-4.5", ModelName: "Claude Sonnet 4.5"},
	}
	static := []*ModelInfo{
		{ID: "kiro-claude-sonnet-4-5", MaxCompletionTokens: 64000, ContextLength: 200000},
	}

	dynamic := ConvertKiroAPIModels(apiModels)
	merged := MergeWithStaticMetadata(dynamic, static)
	final := GenerateAgenticVariants(merged)

	// Base model must have ExecutionTarget
	var base, agentic *ModelInfo
	for _, m := range final {
		switch m.ID {
		case "kiro-claude-sonnet-4-5":
			base = m
		case "kiro-claude-sonnet-4-5-agentic":
			agentic = m
		}
	}
	if base == nil {
		t.Fatal("base model missing from pipeline output")
	}
	if base.ExecutionTarget != "claude-sonnet-4.5" {
		t.Errorf("base ExecutionTarget = %q, want %q", base.ExecutionTarget, "claude-sonnet-4.5")
	}
	if agentic == nil {
		t.Fatal("agentic variant missing from pipeline output")
	}
	if agentic.ExecutionTarget != "claude-sonnet-4.5" {
		t.Errorf("agentic ExecutionTarget = %q, want %q", agentic.ExecutionTarget, "claude-sonnet-4.5")
	}
}

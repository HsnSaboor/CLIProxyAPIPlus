package executor

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/tidwall/gjson"
)

func TestNormalizeDeltaContentArray(t *testing.T) {
	t.Run("no data prefix unchanged", func(t *testing.T) {
		input := `{"choices":[{"delta":{"content":"hello"}}]}`
		got := normalizeDeltaContentArray([]byte(input))
		if string(got) != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("data DONE unchanged", func(t *testing.T) {
		input := "data: [DONE]"
		got := normalizeDeltaContentArray([]byte(input))
		if string(got) != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("empty choices unchanged", func(t *testing.T) {
		input := `data: {"choices":[]}`
		got := normalizeDeltaContentArray([]byte(input))
		if string(got) != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("string content unchanged", func(t *testing.T) {
		input := `data: {"choices":[{"delta":{"content":"hello"}}]}`
		got := normalizeDeltaContentArray([]byte(input))
		if string(got) != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("array content normalized strips thinking", func(t *testing.T) {
		input := `data: {"choices":[{"delta":{"content":[{"type":"text","text":"hello"},{"type":"thinking","text":"ignore"}]}}]}`
		got := normalizeDeltaContentArray([]byte(input))
		jsonPart := got[len("data: "):]
		var obj struct {
			Choices []struct {
				Delta struct {
					Content json.RawMessage `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(jsonPart, &obj); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var s string
		if err := json.Unmarshal(obj.Choices[0].Delta.Content, &s); err != nil {
			t.Fatalf("content not string: %v", err)
		}
		if s != "hello" {
			t.Errorf("content = %q, want %q", s, "hello")
		}
	})

	t.Run("multiple choices array content all normalized", func(t *testing.T) {
		input := `data: {"choices":[{"delta":{"content":[{"type":"text","text":"a"},{"type":"thinking","text":"skip"}]}},{"delta":{"content":[{"type":"text","text":"b"}]}}]}`
		got := normalizeDeltaContentArray([]byte(input))
		jsonPart := got[len("data: "):]
		var obj struct {
			Choices []struct {
				Delta struct {
					Content json.RawMessage `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(jsonPart, &obj); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		wantContents := []string{"a", "b"}
		if len(obj.Choices) != len(wantContents) {
			t.Fatalf("got %d choices, want %d", len(obj.Choices), len(wantContents))
		}
		for i, want := range wantContents {
			var s string
			if err := json.Unmarshal(obj.Choices[i].Delta.Content, &s); err != nil {
				t.Fatalf("choice %d content not string: %v", i, err)
			}
			if s != want {
				t.Errorf("choice %d content = %q, want %q", i, s, want)
			}
		}
	})

	t.Run("SSE line without choices unchanged", func(t *testing.T) {
		input := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk"}`
		got := normalizeDeltaContentArray([]byte(input))
		if string(got) != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("multiple text parts in one choice joined", func(t *testing.T) {
		input := `data: {"choices":[{"delta":{"content":[{"type":"text","text":"hello"},{"type":"text","text":" world"}]}}]}`
		got := normalizeDeltaContentArray([]byte(input))
		jsonPart := got[len("data: "):]
		var obj struct {
			Choices []struct {
				Delta struct {
					Content json.RawMessage `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(jsonPart, &obj); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var s string
		if err := json.Unmarshal(obj.Choices[0].Delta.Content, &s); err != nil {
			t.Fatalf("content not string: %v", err)
		}
		if s != "hello world" {
			t.Errorf("content = %q, want %q", s, "hello world")
		}
	})

	// Anthropic-style reasoning blocks nest visible text under
	// summary[].text rather than a direct "text" field. Reasoning text must
	// be preserved via delta.reasoning_content (matching the convention
	// already used by codebuddy/github_copilot/kimi/mistral/iflow
	// executors) rather than silently dropped, while delta.content must
	// still become a plain string to satisfy OpenAI streaming schema
	// validation (e.g. OpenCode/Claude Code reject an array here).
	t.Run("reasoning block with nested summary extracted to reasoning_content", func(t *testing.T) {
		input := `data: {"choices":[{"delta":{"content":[{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking about it"}]},{"type":"text","text":"the answer"}]}}]}`
		got := normalizeDeltaContentArray([]byte(input))
		jsonPart := got[len("data: "):]
		var obj struct {
			Choices []struct {
				Delta struct {
					Content          json.RawMessage `json:"content"`
					ReasoningContent string          `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(jsonPart, &obj); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var content string
		if err := json.Unmarshal(obj.Choices[0].Delta.Content, &content); err != nil {
			t.Fatalf("content not string: %v", err)
		}
		if content != "the answer" {
			t.Errorf("content = %q, want %q", content, "the answer")
		}
		if obj.Choices[0].Delta.ReasoningContent != "thinking about it" {
			t.Errorf("reasoning_content = %q, want %q", obj.Choices[0].Delta.ReasoningContent, "thinking about it")
		}
	})

	t.Run("reasoning-only chunk has empty content string and populated reasoning_content", func(t *testing.T) {
		input := `data: {"choices":[{"delta":{"content":[{"type":"reasoning","summary":[{"type":"summary_text","text":"step one"}]}]}}]}`
		got := normalizeDeltaContentArray([]byte(input))
		jsonPart := got[len("data: "):]
		var obj struct {
			Choices []struct {
				Delta struct {
					Content          json.RawMessage `json:"content"`
					ReasoningContent string          `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(jsonPart, &obj); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var content string
		if err := json.Unmarshal(obj.Choices[0].Delta.Content, &content); err != nil {
			t.Fatalf("content not string: %v", err)
		}
		if content != "" {
			t.Errorf("content = %q, want empty string", content)
		}
		if obj.Choices[0].Delta.ReasoningContent != "step one" {
			t.Errorf("reasoning_content = %q, want %q", obj.Choices[0].Delta.ReasoningContent, "step one")
		}
	})

	t.Run("thinking type alias also extracted to reasoning_content", func(t *testing.T) {
		input := `data: {"choices":[{"delta":{"content":[{"type":"thinking","text":"pondering"},{"type":"text","text":"done"}]}}]}`
		got := normalizeDeltaContentArray([]byte(input))
		jsonPart := got[len("data: "):]
		var obj struct {
			Choices []struct {
				Delta struct {
					Content          json.RawMessage `json:"content"`
					ReasoningContent string          `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(jsonPart, &obj); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if obj.Choices[0].Delta.ReasoningContent != "pondering" {
			t.Errorf("reasoning_content = %q, want %q", obj.Choices[0].Delta.ReasoningContent, "pondering")
		}
	})

	t.Run("reasoning block with empty text omits reasoning_content field", func(t *testing.T) {
		// Claude Sonnet 5's default thinking.display="omitted" returns
		// reasoning blocks with an empty text/summary (only the encrypted
		// signature is populated). No reasoning_content field should be
		// added in that case.
		input := `data: {"choices":[{"delta":{"content":[{"type":"reasoning","summary":[{"type":"summary_text","text":"","signature":"abc"}]},{"type":"text","text":"answer"}]}}]}`
		got := normalizeDeltaContentArray([]byte(input))
		jsonPart := got[len("data: "):]
		if gjson.GetBytes(jsonPart, "choices.0.delta.reasoning_content").Exists() {
			t.Errorf("reasoning_content should be absent when reasoning text is empty, got %s", jsonPart)
		}
	})
}

func TestIsClaudeFamilyModel(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{"databricks-claude-sonnet-5", true},
		{"claude-opus-4-6", true},
		{"CLAUDE-3-5-SONNET", true},
		{"gpt-5", false},
		{"", false},
		{"deepseek-v4", false},
	}
	for _, tc := range cases {
		if got := isClaudeFamilyModel(tc.model); got != tc.want {
			t.Errorf("isClaudeFamilyModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

func TestThinkingFormatFor(t *testing.T) {
	cases := []struct {
		name          string
		defaultFormat string
		model         string
		want          string
	}{
		{"claude model routes to claude format", "openai", "databricks-claude-sonnet-5", "claude"},
		{"non-claude model keeps default format", "openai", "gpt-5", "openai"},
		{"empty model keeps default format", "openai", "", "openai"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := thinkingFormatFor(tc.defaultFormat, tc.model); got != tc.want {
				t.Errorf("thinkingFormatFor(%q, %q) = %q, want %q", tc.defaultFormat, tc.model, got, tc.want)
			}
		})
	}
}

func TestStripLeakedReasoningEffortForClaude(t *testing.T) {
	t.Run("removes reasoning_effort when present", func(t *testing.T) {
		input := `{"model":"databricks-claude-sonnet-5","messages":[],"options":{"reasoningEffort":"xhigh"},"reasoning_effort":"xhigh","reasoningSummary":"auto","include":["reasoning"],"verbosity":"detailed","interleaved":true}`
		got := stripLeakedReasoningEffortForClaude([]byte(input))
		fields := []string{"options", "reasoning", "reasoningSummary", "include", "verbosity", "interleaved", "reasoning_effort"}
		for _, f := range fields {
			if gjson.GetBytes(got, f).Exists() {
				t.Fatalf("field %s should be removed, got %s", f, got)
			}
		}
		if model := gjson.GetBytes(got, "model").String(); model != "databricks-claude-sonnet-5" {
			t.Fatalf("model = %q, want databricks-claude-sonnet-5; payload=%s", model, got)
		}
	})

	t.Run("no-op when reasoning_effort absent", func(t *testing.T) {
		input := `{"model":"databricks-claude-sonnet-5","messages":[],"thinking":{"type":"adaptive"}}`
		got := stripLeakedReasoningEffortForClaude([]byte(input))
		if string(got) != input {
			t.Errorf("got %q, want unchanged %q", got, input)
		}
	})

	t.Run("empty body unchanged", func(t *testing.T) {
		got := stripLeakedReasoningEffortForClaude([]byte(""))
		if string(got) != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("invalid json unchanged", func(t *testing.T) {
		input := "not json"
		got := stripLeakedReasoningEffortForClaude([]byte(input))
		if string(got) != input {
			t.Errorf("got %q, want unchanged %q", got, input)
		}
	})
}

func TestPromoteOptionsReasoningEffort(t *testing.T) {
	t.Run("promotes options.reasoningEffort to reasoning_effort", func(t *testing.T) {
		input := `{"model":"claude-sonnet-5","options":{"reasoningEffort":"xhigh","textVerbosity":"low"}}`
		got := promoteOptionsReasoningEffort([]byte(input))
		if val := gjson.GetBytes(got, "reasoning_effort").String(); val != "xhigh" {
			t.Fatalf("reasoning_effort should be promoted to xhigh, got %s", got)
		}
	})

	t.Run("no-op when options.reasoningEffort absent", func(t *testing.T) {
		input := `{"model":"claude-sonnet-5","options":{"textVerbosity":"low"}}`
		got := promoteOptionsReasoningEffort([]byte(input))
		if string(got) != input {
			t.Errorf("got %q, want unchanged %q", got, input)
		}
	})

	t.Run("empty body unchanged", func(t *testing.T) {
		got := promoteOptionsReasoningEffort([]byte(""))
		if string(got) != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("invalid json unchanged", func(t *testing.T) {
		input := "not json"
		got := promoteOptionsReasoningEffort([]byte(input))
		if string(got) != input {
			t.Errorf("got %q, want unchanged %q", got, input)
		}
	})
}

func TestStripOpenAICompatProviderUnsupportedFields_Kimi(t *testing.T) {
	payload := []byte(`{"model":"kimi-k2.5","messages":[],"reasoning_effort":"high","reasoning":{"enabled":true},"reasoningSummary":"auto","include":["reasoning"],"verbosity":"detailed"}`)
	compat := &config.OpenAICompatibility{Name: "kimi", BaseURL: "https://api.moonshot.cn/v1"}

	got := stripOpenAICompatProviderUnsupportedFields("openai-compatible-kimi", compat, payload)
	for _, field := range []string{"reasoning_effort", "reasoning", "reasoningSummary", "include", "verbosity"} {
		if gjson.GetBytes(got, field).Exists() {
			t.Fatalf("%s should be removed for OpenAI-compatible Kimi provider, got %s", field, got)
		}
	}
	if model := gjson.GetBytes(got, "model").String(); model != "kimi-k2.5" {
		t.Fatalf("model = %q, want kimi-k2.5; payload=%s", model, got)
	}
}

func TestStripOpenAICompatProviderUnsupportedFields_NonKimiUnchanged(t *testing.T) {
	payload := []byte(`{"model":"glm-5","messages":[],"reasoning_effort":"max"}`)
	compat := &config.OpenAICompatibility{Name: "glm", BaseURL: "https://glm.example/v1"}

	got := stripOpenAICompatProviderUnsupportedFields("openai-compatible-glm", compat, payload)
	if string(got) != string(payload) {
		t.Fatalf("non-Kimi compat payload changed: got %s want %s", got, payload)
	}
}

func TestOmitMiniMaxM3ThinkingType_ForResolvedModel(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		payload      string
		wantType     string
		wantThinking bool
	}{
		{name: "adaptive", model: "minimaxai/minimax-m3", payload: `{"thinking":{"type":"adaptive"}}`},
		{name: "disabled", model: "minimax-m3", payload: `{"thinking":{"type":"disabled"}}`},
		{name: "preserve other thinking fields", model: "vendor/minimax-m3-preview", payload: `{"thinking":{"type":"adaptive","budget_tokens":8192}}`, wantThinking: true},
		{name: "missing", model: "vendor/minimax-m3-preview", payload: `{}`},
		{name: "other model", model: "minimax-m2", payload: `{"thinking":{"type":"adaptive"}}`, wantType: "adaptive", wantThinking: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := omitMiniMaxM3ThinkingType(tt.model, []byte(tt.payload))
			if thinkingType := gjson.GetBytes(got, "thinking.type").String(); thinkingType != tt.wantType {
				t.Fatalf("thinking.type = %q, want %q; payload=%s", thinkingType, tt.wantType, got)
			}
			if thinkingExists := gjson.GetBytes(got, "thinking").Exists(); thinkingExists != tt.wantThinking {
				t.Fatalf("thinking exists = %t, want %t; payload=%s", thinkingExists, tt.wantThinking, got)
			}
		})
	}
}

func TestNormalizeMistralReasoningEffort(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		payload    string
		wantEffort string
	}{
		{name: "medium replaced", model: "mistral-medium-latest", payload: `{"reasoning_effort":"medium"}`, wantEffort: "high"},
		{name: "low replaced", model: "mistralai/mistral-large", payload: `{"reasoning_effort":"low"}`, wantEffort: "high"},
		{name: "high preserved", model: "mistral-medium-latest", payload: `{"reasoning_effort":"high"}`, wantEffort: "high"},
		{name: "none replaced", model: "mistral-medium-latest", payload: `{"reasoning_effort":"none"}`, wantEffort: "high"},
		{name: "missing field untouched", model: "mistral-medium-latest", payload: `{}`, wantEffort: ""},
		{name: "non-mistral model untouched", model: "gpt-5", payload: `{"reasoning_effort":"medium"}`, wantEffort: "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeMistralReasoningEffort(tt.model, []byte(tt.payload))
			if effort := gjson.GetBytes(got, "reasoning_effort").String(); effort != tt.wantEffort {
				t.Fatalf("reasoning_effort = %q, want %q; payload=%s", effort, tt.wantEffort, got)
			}
		})
	}
}

func TestFixMistralMessageOrder(t *testing.T) {
	e := &OpenAICompatExecutor{}

	t.Run("empty messages", func(t *testing.T) {
		input := []byte(`{"messages":[]}`)
		got := e.fixMistralMessageOrder(input)
		if string(got) != string(input) {
			t.Errorf("got %s, want unchanged", got)
		}
	})

	t.Run("last message is user", func(t *testing.T) {
		input := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
		got := e.fixMistralMessageOrder(input)
		if string(got) != string(input) {
			t.Errorf("got %s, want unchanged", got)
		}
	})

	t.Run("last assistant no prefix field adds prefix true", func(t *testing.T) {
		input := []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"}]}`)
		got := e.fixMistralMessageOrder(input)
		var obj struct {
			Messages []struct {
				Role   string `json:"role"`
				Prefix *bool  `json:"prefix,omitempty"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(got, &obj); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		last := obj.Messages[len(obj.Messages)-1]
		if last.Role != "assistant" {
			t.Fatalf("last role = %q, want assistant", last.Role)
		}
		if last.Prefix == nil || !*last.Prefix {
			t.Errorf("expected prefix=true, got %v", last.Prefix)
		}
	})

	t.Run("last assistant with prefix true unchanged", func(t *testing.T) {
		input := []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello","prefix":true}]}`)
		got := e.fixMistralMessageOrder(input)
		var obj struct {
			Messages []json.RawMessage `json:"messages"`
		}
		if err := json.Unmarshal(got, &obj); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(obj.Messages) != 2 {
			t.Errorf("got %d messages, want 2", len(obj.Messages))
		}
	})

	t.Run("last assistant with prefix false appends placeholder user", func(t *testing.T) {
		input := []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello","prefix":false}]}`)
		got := e.fixMistralMessageOrder(input)
		var obj struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(got, &obj); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(obj.Messages) != 3 {
			t.Fatalf("got %d messages, want 3", len(obj.Messages))
		}
		last := obj.Messages[len(obj.Messages)-1]
		if last.Role != "user" {
			t.Errorf("appended role = %q, want user", last.Role)
		}
		if last.Content != "." {
			t.Errorf("appended content = %q, want '.'", last.Content)
		}
	})

	t.Run("last message is tool unchanged", func(t *testing.T) {
		input := []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"tool","content":"result","tool_call_id":"call_1"}]}`)
		got := e.fixMistralMessageOrder(input)
		if string(got) != string(input) {
			t.Errorf("got %s, want unchanged", got)
		}
	})

	t.Run("no messages field unchanged", func(t *testing.T) {
		input := []byte(`{"model":"mistral-large","temperature":0.7}`)
		got := e.fixMistralMessageOrder(input)
		if string(got) != string(input) {
			t.Errorf("got %s, want unchanged", got)
		}
	})
}

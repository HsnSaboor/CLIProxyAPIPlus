package executor

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestOpenAICompat_InjectToolsCacheControl(t *testing.T) {
	t.Run("Last Tool Gets Cache Control", func(t *testing.T) {
		input := []byte(`{
			"model": "gpt-4",
			"tools": [
				{"type": "function", "function": {"name": "tool1"}},
				{"type": "function", "function": {"name": "tool2"}},
				{"type": "function", "function": {"name": "tool3"}}
			]
		}`)
		output := injectOpenAIToolsCacheControl(input)

		if gjson.GetBytes(output, "tools.0.cache_control").Exists() {
			t.Errorf("tool 0 should NOT have cache_control")
		}
		if gjson.GetBytes(output, "tools.1.cache_control").Exists() {
			t.Errorf("tool 1 should NOT have cache_control")
		}
		if got := gjson.GetBytes(output, "tools.2.cache_control.type").String(); got != "ephemeral" {
			t.Errorf("tool 2 (last) cache_control.type = %q, want %q. Output: %s", got, "ephemeral", output)
		}
	})

	t.Run("Existing Cache Control Is Noop", func(t *testing.T) {
		input := []byte(`{
			"tools": [
				{"type": "function", "function": {"name": "tool1"}, "cache_control": {"type": "ephemeral"}},
				{"type": "function", "function": {"name": "tool2"}},
				{"type": "function", "function": {"name": "tool3"}}
			]
		}`)
		output := injectOpenAIToolsCacheControl(input)

		if !gjson.GetBytes(output, "tools.0.cache_control").Exists() {
			t.Errorf("tool 0 existing cache_control should be preserved")
		}
		if gjson.GetBytes(output, "tools.2.cache_control").Exists() {
			t.Errorf("tool 2 should NOT get cache_control injected when another tool already has one")
		}
		if string(output) != string(input) {
			t.Errorf("payload should be unchanged (no-op).\noriginal: %s\ngot:      %s", input, output)
		}
	})

	t.Run("Empty Tools Array Is Noop", func(t *testing.T) {
		input := []byte(`{"model": "gpt-4", "tools": []}`)
		output := injectOpenAIToolsCacheControl(input)

		if string(output) != string(input) {
			t.Errorf("payload should be unchanged for empty tools array.\noriginal: %s\ngot:      %s", input, output)
		}
	})

	t.Run("No Tools Key Is Noop", func(t *testing.T) {
		input := []byte(`{"model": "gpt-4", "messages": []}`)
		output := injectOpenAIToolsCacheControl(input)

		if string(output) != string(input) {
			t.Errorf("payload should be unchanged when tools key absent.\noriginal: %s\ngot:      %s", input, output)
		}
	})
}

func TestOpenAICompat_InjectSystemCacheControl(t *testing.T) {
	t.Run("String System Content Converted To Blocks", func(t *testing.T) {
		input := []byte(`{
			"messages": [
				{"role": "system", "content": "plain string"},
				{"role": "user", "content": "hi"}
			]
		}`)
		output := injectOpenAISystemCacheControl(input)

		if got := gjson.GetBytes(output, "messages.0.content.0.text").String(); got != "plain string" {
			t.Errorf("system content text = %q, want %q. Output: %s", got, "plain string", output)
		}
		if got := gjson.GetBytes(output, "messages.0.content.0.cache_control.type").String(); got != "ephemeral" {
			t.Errorf("system content.0.cache_control.type = %q, want %q. Output: %s", got, "ephemeral", output)
		}
	})

	t.Run("Array System Content Cache Control On Last Block", func(t *testing.T) {
		input := []byte(`{
			"messages": [
				{"role": "system", "content": [{"type": "text", "text": "a"}, {"type": "text", "text": "b"}]},
				{"role": "user", "content": "hi"}
			]
		}`)
		output := injectOpenAISystemCacheControl(input)

		if gjson.GetBytes(output, "messages.0.content.0.cache_control").Exists() {
			t.Errorf("first system content block should NOT have cache_control")
		}
		if got := gjson.GetBytes(output, "messages.0.content.1.cache_control.type").String(); got != "ephemeral" {
			t.Errorf("last system content block cache_control.type = %q, want %q. Output: %s", got, "ephemeral", output)
		}
	})

	t.Run("Existing Cache Control Is Noop", func(t *testing.T) {
		input := []byte(`{
			"messages": [
				{"role": "system", "content": [{"type": "text", "text": "a", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": "hi"}
			]
		}`)
		output := injectOpenAISystemCacheControl(input)

		if string(output) != string(input) {
			t.Errorf("payload should be unchanged when system already has cache_control.\noriginal: %s\ngot:      %s", input, output)
		}
	})

	t.Run("No System Message Is Noop", func(t *testing.T) {
		input := []byte(`{
			"messages": [
				{"role": "user", "content": "hi"},
				{"role": "assistant", "content": "hello"}
			]
		}`)
		output := injectOpenAISystemCacheControl(input)

		if string(output) != string(input) {
			t.Errorf("payload should be unchanged when no system message present.\noriginal: %s\ngot:      %s", input, output)
		}
	})
}

func TestOpenAICompat_InjectMessagesCacheControl(t *testing.T) {
	t.Run("Second To Last User Turn Gets Cache Control", func(t *testing.T) {
		input := []byte(`{
			"messages": [
				{"role": "user", "content": [{"type": "text", "text": "turn1"}]},
				{"role": "assistant", "content": [{"type": "text", "text": "reply1"}]},
				{"role": "user", "content": [{"type": "text", "text": "turn2"}]}
			]
		}`)
		output := injectOpenAIMessagesCacheControl(input)

		if got := gjson.GetBytes(output, "messages.0.content.0.cache_control.type").String(); got != "ephemeral" {
			t.Errorf("second-to-last user message (index 0) cache_control.type = %q, want %q. Output: %s", got, "ephemeral", output)
		}
		if gjson.GetBytes(output, "messages.2.content.0.cache_control").Exists() {
			t.Errorf("last user message (index 2) should NOT have cache_control")
		}
	})

	t.Run("Single User Message Is Noop", func(t *testing.T) {
		input := []byte(`{
			"messages": [
				{"role": "user", "content": [{"type": "text", "text": "only turn"}]}
			]
		}`)
		output := injectOpenAIMessagesCacheControl(input)

		if string(output) != string(input) {
			t.Errorf("payload should be unchanged with only 1 user message.\noriginal: %s\ngot:      %s", input, output)
		}
	})

	t.Run("Existing Cache Control Anywhere Is Noop", func(t *testing.T) {
		input := []byte(`{
			"messages": [
				{"role": "user", "content": [{"type": "text", "text": "turn1"}]},
				{"role": "assistant", "content": [{"type": "text", "text": "reply1", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "turn2"}]}
			]
		}`)
		output := injectOpenAIMessagesCacheControl(input)

		if string(output) != string(input) {
			t.Errorf("payload should be unchanged when any message already has cache_control.\noriginal: %s\ngot:      %s", input, output)
		}
	})
}

func TestOpenAICompat_CountCacheControls(t *testing.T) {
	t.Run("Mixed Tools And Messages", func(t *testing.T) {
		input := []byte(`{
			"tools": [
				{"name": "t1", "cache_control": {"type": "ephemeral"}},
				{"name": "t2", "cache_control": {"type": "ephemeral"}}
			],
			"messages": [
				{"role": "user", "content": [{"type": "text", "text": "u1", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "u2"}]}
			]
		}`)

		if got := countOpenAICacheControls(input); got != 3 {
			t.Errorf("countOpenAICacheControls = %d, want 3", got)
		}
	})

	t.Run("Empty Payload Is Zero", func(t *testing.T) {
		input := []byte(`{"model": "gpt-4"}`)

		if got := countOpenAICacheControls(input); got != 0 {
			t.Errorf("countOpenAICacheControls = %d, want 0", got)
		}
	})

	// Regression test: a real-world client (opencode) can place cache_control
	// in locations this package does not itself inject into (e.g. nested inside
	// tools[].function, or top-level system, or elsewhere). The counter must find
	// ALL cache_control blocks anywhere in the payload, not just the two known
	// injection locations (tools[].cache_control and messages[].content[].cache_control),
	// otherwise the server can undercount, inject more of its own, and forward a
	// request exceeding Anthropic's 4-block cap to the upstream provider.
	t.Run("Finds Cache Control In Unexpected Locations", func(t *testing.T) {
		input := []byte(`{
			"tools": [
				{"type": "function", "function": {"name": "t1", "cache_control": {"type": "ephemeral"}}}
			],
			"system": [
				{"type": "text", "text": "sys", "cache_control": {"type": "ephemeral"}}
			],
			"messages": [
				{"role": "user", "content": [{"type": "text", "text": "u1", "cache_control": {"type": "ephemeral"}}]}
			]
		}`)

		if got := countOpenAICacheControls(input); got != 3 {
			t.Errorf("countOpenAICacheControls = %d, want 3 (tool.function, top-level system, message)", got)
		}
	})
}

func TestOpenAICompat_EnforceCacheControlLimit(t *testing.T) {
	t.Run("At Limit Is Noop", func(t *testing.T) {
		input := []byte(`{
			"tools": [
				{"name": "t1", "cache_control": {"type": "ephemeral"}}
			],
			"messages": [
				{"role": "user", "content": [{"type": "text", "text": "u1", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "u2", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "u3", "cache_control": {"type": "ephemeral"}}]}
			]
		}`)

		out := enforceOpenAICacheControlLimit(input, 4)

		if got := countOpenAICacheControls(out); got != 4 {
			t.Errorf("cache_control count = %d, want 4 (noop)", got)
		}
		if string(out) != string(input) {
			t.Errorf("payload should be unchanged when already at limit.\noriginal: %s\ngot:      %s", input, out)
		}
	})

	t.Run("Excess Removed From Messages Not Tools", func(t *testing.T) {
		input := []byte(`{
			"tools": [
				{"name": "t1", "cache_control": {"type": "ephemeral"}}
			],
			"messages": [
				{"role": "user", "content": [{"type": "text", "text": "u1", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "u2", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "u3", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "u4", "cache_control": {"type": "ephemeral"}}]}
			]
		}`)

		out := enforceOpenAICacheControlLimit(input, 4)

		if got := countOpenAICacheControls(out); got != 4 {
			t.Errorf("cache_control count = %d, want 4 after trimming", got)
		}
		if !gjson.GetBytes(out, "tools.0.cache_control").Exists() {
			t.Errorf("tools.0.cache_control (highest priority) should survive trimming")
		}
	})

	// Regression test for the exact production failure: "A maximum of 4 blocks
	// with cache_control may be provided. Found 5." A client-supplied cache_control
	// block in a location outside tools[]/messages[].content[] (here: nested inside
	// tools[].function) combined with 4 more in known locations must still be
	// trimmed down to exactly 4 total, using the catch-all phase, not silently
	// forwarded over the limit.
	t.Run("Excess In Unknown Location Is Still Trimmed", func(t *testing.T) {
		input := []byte(`{
			"tools": [
				{"type": "function", "function": {"name": "hidden", "cache_control": {"type": "ephemeral"}}}
			],
			"messages": [
				{"role": "system", "content": [{"type": "text", "text": "sys", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "u1", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "u2", "cache_control": {"type": "ephemeral"}}]},
				{"role": "user", "content": [{"type": "text", "text": "u3", "cache_control": {"type": "ephemeral"}}]}
			]
		}`)

		if got := countOpenAICacheControls(input); got != 5 {
			t.Fatalf("precondition failed: countOpenAICacheControls = %d, want 5", got)
		}

		out := enforceOpenAICacheControlLimit(input, 4)

		if got := countOpenAICacheControls(out); got != 4 {
			t.Errorf("cache_control count after enforcement = %d, want exactly 4 (would be rejected upstream otherwise)", got)
		}
	})
}

func TestOpenAICompat_EnsureCacheControl_FullPipeline(t *testing.T) {
	input := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "You are a helpful coding assistant with a very long system prompt."},
			{"role": "user", "content": [{"type": "text", "text": "turn 1 request"}]},
			{"role": "assistant", "content": [{"type": "text", "text": "turn 1 reply"}]},
			{"role": "user", "content": [{"type": "text", "text": "turn 2 request"}]}
		],
		"tools": [
			{"type": "function", "function": {"name": "read_file"}},
			{"type": "function", "function": {"name": "write_file"}},
			{"type": "function", "function": {"name": "run_shell"}}
		]
	}`)

	output := ensureOpenAICacheControl(input)

	if got := gjson.GetBytes(output, "tools.2.cache_control.type").String(); got != "ephemeral" {
		t.Errorf("last tool cache_control.type = %q, want %q. Output: %s", got, "ephemeral", output)
	}
	if gjson.GetBytes(output, "tools.0.cache_control").Exists() {
		t.Errorf("first tool should NOT have cache_control")
	}

	if got := gjson.GetBytes(output, "messages.0.content.0.cache_control.type").String(); got != "ephemeral" {
		t.Errorf("system message cache_control.type = %q, want %q. Output: %s", got, "ephemeral", output)
	}

	if got := gjson.GetBytes(output, "messages.1.content.0.cache_control.type").String(); got != "ephemeral" {
		t.Errorf("second-to-last (turn 1) user message cache_control.type = %q, want %q. Output: %s", got, "ephemeral", output)
	}

	if gjson.GetBytes(output, "messages.3.content.0.cache_control").Exists() {
		t.Errorf("last user message (turn 2) should NOT have cache_control")
	}
}

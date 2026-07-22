package executor

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ensureOpenAICacheControl injects Anthropic-style cache_control breakpoints into an
// OpenAI chat-completions shaped payload (messages[]/tools[]) for OpenAI-compatible
// providers whose backend is actually Claude under the hood (e.g. Databricks Claude)
// and therefore honors cache_control even though it is not part of the OpenAI spec.
//
// Breakpoints are placed, in cache-hierarchy order:
//  1. last tool definition (caches all tool schemas)
//  2. last content block of the system message (caches the system prompt)
//  3. last content block of the second-to-last user turn (caches conversation history)
//
// See: https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching
func ensureOpenAICacheControl(payload []byte) []byte {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload
	}
	payload = injectOpenAIToolsCacheControl(payload)
	payload = injectOpenAISystemCacheControl(payload)
	payload = injectOpenAIMessagesCacheControl(payload)
	return payload
}

// countOpenAICacheControls counts existing cache_control blocks anywhere in the
// payload. It walks the entire JSON tree rather than only the known injection
// locations (tools[]/messages[].content[]), because upstream clients (e.g.
// opencode) may place their own cache_control breakpoints in locations this
// package does not otherwise inject into. Undercounting here would let the
// combined client + server breakpoint total silently exceed Anthropic's hard
// cap of 4 cache_control blocks per request.
func countOpenAICacheControls(payload []byte) int {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return 0
	}
	return len(collectCacheControlPaths(gjson.ParseBytes(payload), ""))
}

// collectCacheControlPaths recursively walks a JSON value and returns the sjson-style
// dot/index paths (relative to root) of every object that has a "cache_control" key.
func collectCacheControlPaths(result gjson.Result, prefix string) []string {
	var paths []string
	if !result.Exists() {
		return paths
	}
	switch {
	case result.IsObject():
		result.ForEach(func(key, value gjson.Result) bool {
			k := key.String()
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			if k == "cache_control" {
				paths = append(paths, path)
				return true
			}
			paths = append(paths, collectCacheControlPaths(value, path)...)
			return true
		})
	case result.IsArray():
		idx := 0
		result.ForEach(func(_, value gjson.Result) bool {
			path := fmt.Sprintf("%d", idx)
			if prefix != "" {
				path = prefix + "." + path
			}
			paths = append(paths, collectCacheControlPaths(value, path)...)
			idx++
			return true
		})
	}
	return paths
}

// enforceOpenAICacheControlLimit trims cache_control blocks in an OpenAI chat-completions
// shaped payload down to maxBlocks, preserving the highest-priority breakpoints first, in
// this removal order (least valuable removed first): non-system conversation turns,
// then the system message, then tools (most valuable, removed last).
func enforceOpenAICacheControlLimit(payload []byte, maxBlocks int) []byte {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload
	}

	total := countOpenAICacheControls(payload)
	if total <= maxBlocks {
		return payload
	}
	excess := total - maxBlocks

	// Phase 1: strip cache_control from non-system message turns first (lowest
	// priority — ordinary conversation history is the most disposable breakpoint).
	messages := gjson.GetBytes(payload, "messages")
	if messages.IsArray() {
		messages.ForEach(func(msgIdx, msg gjson.Result) bool {
			if excess <= 0 {
				return false
			}
			if msg.Get("role").String() == "system" {
				return true
			}
			content := msg.Get("content")
			if !content.IsArray() {
				return true
			}
			content.ForEach(func(itemIdx, item gjson.Result) bool {
				if excess <= 0 {
					return false
				}
				if !item.Get("cache_control").Exists() {
					return true
				}
				path := fmt.Sprintf("messages.%d.content.%d.cache_control", int(msgIdx.Int()), int(itemIdx.Int()))
				updated, errDel := sjson.DeleteBytes(payload, path)
				if errDel != nil {
					return true
				}
				payload = updated
				excess--
				return true
			})
			return true
		})
	}
	if excess <= 0 {
		return payload
	}

	// Phase 2: strip cache_control from the system message (higher priority than
	// conversation history, lower priority than tools).
	messages = gjson.GetBytes(payload, "messages")
	if messages.IsArray() {
		messages.ForEach(func(msgIdx, msg gjson.Result) bool {
			if excess <= 0 {
				return false
			}
			if msg.Get("role").String() != "system" {
				return true
			}
			content := msg.Get("content")
			if !content.IsArray() {
				return true
			}
			content.ForEach(func(itemIdx, item gjson.Result) bool {
				if excess <= 0 {
					return false
				}
				if !item.Get("cache_control").Exists() {
					return true
				}
				path := fmt.Sprintf("messages.%d.content.%d.cache_control", int(msgIdx.Int()), int(itemIdx.Int()))
				updated, errDel := sjson.DeleteBytes(payload, path)
				if errDel != nil {
					return true
				}
				payload = updated
				excess--
				return true
			})
			return true
		})
	}
	if excess <= 0 {
		return payload
	}

	// Phase 3: strip cache_control from tools last (highest priority — caches the
	// largest, most stable chunk of the payload).
	tools := gjson.GetBytes(payload, "tools")
	if tools.IsArray() {
		tools.ForEach(func(idx, item gjson.Result) bool {
			if excess <= 0 {
				return false
			}
			if !item.Get("cache_control").Exists() {
				return true
			}
			path := fmt.Sprintf("tools.%d.cache_control", int(idx.Int()))
			updated, errDel := sjson.DeleteBytes(payload, path)
			if errDel != nil {
				return true
			}
			payload = updated
			excess--
			return true
		})
	}
	if excess <= 0 {
		return payload
	}

	// Phase 4 (catch-all): the previous phases only target the two locations this
	// package injects into (tools[]/messages[].content[]). If cache_control still
	// remains in excess of maxBlocks, it means the payload has breakpoints placed
	// elsewhere (e.g. supplied by the upstream client in a shape we don't otherwise
	// walk). Recursively strip cache_control wherever it appears until we're at or
	// under the limit, rather than silently forwarding an over-limit request that
	// the upstream provider will reject outright.
	if excess > 0 {
		remaining := collectCacheControlPaths(gjson.ParseBytes(payload), "")
		for _, path := range remaining {
			if excess <= 0 {
				break
			}
			updated, errDel := sjson.DeleteBytes(payload, path)
			if errDel != nil {
				continue
			}
			payload = updated
			excess--
		}
	}

	return payload
}

// injectOpenAIToolsCacheControl adds cache_control to the last tool definition in an
// OpenAI-shaped tools array, unless any tool already has one.
func injectOpenAIToolsCacheControl(payload []byte) []byte {
	tools := gjson.GetBytes(payload, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return payload
	}

	toolCount := int(tools.Get("#").Int())
	if toolCount == 0 {
		return payload
	}

	hasCacheControlInTools := false
	tools.ForEach(func(_, tool gjson.Result) bool {
		if tool.Get("cache_control").Exists() {
			hasCacheControlInTools = true
			return false
		}
		return true
	})
	if hasCacheControlInTools {
		return payload
	}

	lastToolPath := fmt.Sprintf("tools.%d.cache_control", toolCount-1)
	result, err := sjson.SetBytes(payload, lastToolPath, map[string]string{"type": "ephemeral"})
	if err != nil {
		log.Warnf("openai compat executor: failed to inject cache_control into tools array: %v", err)
		return payload
	}

	return result
}

// injectOpenAISystemCacheControl adds cache_control to the last content block of the
// system message in an OpenAI-shaped messages array, converting string content into
// the array-of-blocks form when necessary. Skipped if any system message already has
// cache_control on any content block.
func injectOpenAISystemCacheControl(payload []byte) []byte {
	messages := gjson.GetBytes(payload, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return payload
	}

	// Target the LAST system-role message (mirrors the Claude-shape behavior of
	// caching the last element of the system array), and check ALL system-role
	// messages for an existing cache_control before injecting.
	systemIdx := -1
	hasCacheControl := false
	messages.ForEach(func(idx, msg gjson.Result) bool {
		if msg.Get("role").String() != "system" {
			return true
		}
		systemIdx = int(idx.Int())
		content := msg.Get("content")
		if content.IsArray() {
			content.ForEach(func(_, item gjson.Result) bool {
				if item.Get("cache_control").Exists() {
					hasCacheControl = true
					return false
				}
				return true
			})
		} else if content.Get("cache_control").Exists() {
			hasCacheControl = true
		}
		return true
	})

	if systemIdx < 0 || hasCacheControl {
		return payload
	}

	contentPath := fmt.Sprintf("messages.%d.content", systemIdx)
	content := gjson.GetBytes(payload, contentPath)

	if content.IsArray() {
		contentCount := int(content.Get("#").Int())
		if contentCount == 0 {
			return payload
		}
		cacheControlPath := fmt.Sprintf("messages.%d.content.%d.cache_control", systemIdx, contentCount-1)
		result, err := sjson.SetBytes(payload, cacheControlPath, map[string]string{"type": "ephemeral"})
		if err != nil {
			log.Warnf("openai compat executor: failed to inject cache_control into system message array: %v", err)
			return payload
		}
		return result
	}

	if content.Type == gjson.String {
		text := content.String()
		newContent := []map[string]interface{}{
			{
				"type": "text",
				"text": text,
				"cache_control": map[string]string{
					"type": "ephemeral",
				},
			},
		}
		result, err := sjson.SetBytes(payload, contentPath, newContent)
		if err != nil {
			log.Warnf("openai compat executor: failed to inject cache_control into system message string: %v", err)
			return payload
		}
		return result
	}

	return payload
}

// injectOpenAIMessagesCacheControl adds cache_control to the last content block of the
// second-to-last user turn, enabling multi-turn conversation caching. Skipped if any
// message content already has cache_control, or if there are fewer than 2 user turns.
func injectOpenAIMessagesCacheControl(payload []byte) []byte {
	messages := gjson.GetBytes(payload, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return payload
	}

	// Only guard against cache_control already present on non-system turns.
	// The system message may have just received its own breakpoint from
	// injectOpenAISystemCacheControl; that is an independent breakpoint and
	// must not block conversation-history caching here.
	hasCacheControlInMessages := false
	messages.ForEach(func(_, msg gjson.Result) bool {
		if msg.Get("role").String() == "system" {
			return true
		}
		content := msg.Get("content")
		if content.IsArray() {
			content.ForEach(func(_, item gjson.Result) bool {
				if item.Get("cache_control").Exists() {
					hasCacheControlInMessages = true
					return false
				}
				return true
			})
		}
		return !hasCacheControlInMessages
	})
	if hasCacheControlInMessages {
		return payload
	}

	var userMsgIndices []int
	messages.ForEach(func(index gjson.Result, msg gjson.Result) bool {
		if msg.Get("role").String() == "user" {
			userMsgIndices = append(userMsgIndices, int(index.Int()))
		}
		return true
	})

	if len(userMsgIndices) < 2 {
		return payload
	}

	secondToLastUserIdx := userMsgIndices[len(userMsgIndices)-2]

	contentPath := fmt.Sprintf("messages.%d.content", secondToLastUserIdx)
	content := gjson.GetBytes(payload, contentPath)

	if content.IsArray() {
		contentCount := int(content.Get("#").Int())
		if contentCount > 0 {
			cacheControlPath := fmt.Sprintf("messages.%d.content.%d.cache_control", secondToLastUserIdx, contentCount-1)
			result, err := sjson.SetBytes(payload, cacheControlPath, map[string]string{"type": "ephemeral"})
			if err != nil {
				log.Warnf("openai compat executor: failed to inject cache_control into messages: %v", err)
				return payload
			}
			payload = result
		}
	} else if content.Type == gjson.String {
		text := content.String()
		newContent := []map[string]interface{}{
			{
				"type": "text",
				"text": text,
				"cache_control": map[string]string{
					"type": "ephemeral",
				},
			},
		}
		result, err := sjson.SetBytes(payload, contentPath, newContent)
		if err != nil {
			log.Warnf("openai compat executor: failed to inject cache_control into message string content: %v", err)
			return payload
		}
		payload = result
	}

	return payload
}

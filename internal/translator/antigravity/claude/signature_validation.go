// Claude thinking signature validation wrappers for Antigravity bypass mode.
package claude

import (
	"github.com/router-for-me/CLIProxyAPI/v7/internal/cache"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/signature"
)

const maxBypassSignatureLen = signature.MaxClaudeThinkingSignatureLen

type claudeSignatureTree = signature.ClaudeSignatureTree

// StripEmptySignatureThinkingBlocks removes thinking blocks whose signatures
// are empty or not valid Claude thinking signatures. These usually come from
// proxy-generated responses where no real Claude signature exists.
func StripEmptySignatureThinkingBlocks(payload []byte) []byte {
	return signature.StripInvalidClaudeThinkingBlocks(payload, signature.ClaudeSignatureValidationOptions{PrefixOnly: true})
}

func StripInvalidBypassSignatureThinkingBlocks(payload []byte) []byte {
	return signature.StripInvalidClaudeThinkingBlocks(payload, claudeBypassSignatureValidationOptions())
}

func ValidateClaudeBypassSignatures(inputRawJSON []byte) error {
	return signature.ValidateClaudeThinkingSignatures(inputRawJSON, claudeBypassSignatureValidationOptions())
}

func normalizeClaudeBypassSignature(rawSignature string) (string, error) {
	return signature.NormalizeClaudeThinkingSignature(rawSignature, claudeBypassSignatureValidationOptions())
}

func inspectDoubleLayerSignature(sig string) (*claudeSignatureTree, error) {
	return signature.InspectClaudeDoubleLayerSignature(sig)
}

func inspectSingleLayerSignature(sig string) (*claudeSignatureTree, error) {
	return signature.InspectClaudeSingleLayerSignature(sig)
}

func inspectClaudeSignaturePayload(payload []byte, encodingLayers int) (*claudeSignatureTree, error) {
	return signature.InspectClaudeSignaturePayload(payload, encodingLayers)
}

func claudeBypassSignatureValidationOptions() signature.ClaudeSignatureValidationOptions {
	return signature.ClaudeSignatureValidationOptions{Strict: cache.SignatureBypassStrictMode()}
}

func StripEmptySignatureThinkingBlocks(payload []byte) []byte {
	messages := gjson.GetBytes(payload, "messages")
	if !messages.IsArray() {
		return payload
	}
	modified := false
	for i, msg := range messages.Array() {
		content := msg.Get("content")
		if !content.IsArray() {
			continue
		}
		var kept []string
		stripped := false
		for _, part := range content.Array() {
			if part.Get("type").String() == "thinking" && !hasValidClaudeSignature(part.Get("signature").String()) {
				stripped = true
				continue
			}
			kept = append(kept, part.Raw)
		}
		if stripped {
			modified = true
			if len(kept) == 0 {
				payload, _ = sjson.SetRawBytes(payload, fmt.Sprintf("messages.%d.content", i), []byte("[]"))
			} else {
				payload, _ = sjson.SetRawBytes(payload, fmt.Sprintf("messages.%d.content", i), []byte("["+strings.Join(kept, ",")+"]"))
			}
		}
	}
	if !modified {
		return payload
	}
	return payload
}

// hasValidClaudeSignature returns true if sig looks like a real Claude thinking
// signature: non-empty and starts with 'E' or 'R' (after stripping optional
// cache prefix like "modelGroup#").
func hasValidClaudeSignature(sig string) bool {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return false
	}
	if idx := strings.IndexByte(sig, '#'); idx >= 0 {
		sig = strings.TrimSpace(sig[idx+1:])
	}
	if sig == "" {
		return false
	}
	return sig[0] == 'E' || sig[0] == 'R'
}

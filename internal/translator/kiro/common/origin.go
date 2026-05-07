package common

// NormalizeOrigin normalizes origin value for Kiro API compatibility.
// The Kiro API accepts "CLI" and "AI_EDITOR" as origin values.
func NormalizeOrigin(origin string) string {
	switch origin {
	case "KIRO_CLI":
		return "CLI"
	case "KIRO_AI_EDITOR":
		return "AI_EDITOR"
	case "AMAZON_Q":
		return "CLI"
	case "KIRO_IDE":
		return "AI_EDITOR"
	default:
		return origin
	}
}

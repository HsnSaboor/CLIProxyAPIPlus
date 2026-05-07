package common

// NormalizeOrigin normalizes origin value for Kiro API compatibility
func NormalizeOrigin(origin string) string {
	switch origin {
	case "KIRO_CLI":
		return "KIRO_CLI"
	case "KIRO_AI_EDITOR":
		return "KIRO_AI_EDITOR"
	case "AMAZON_Q":
		return "CLI"
	case "KIRO_IDE":
		return "AI_EDITOR"
	default:
		return origin
	}
}

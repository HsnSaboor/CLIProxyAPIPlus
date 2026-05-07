package common

// NormalizeOrigin normalizes origin value for Kiro API compatibility.
// Confirmed via traffic capture: the real Kiro CLI sends "KIRO_CLI" and "AI_EDITOR"
// as origin values in the GenerateAssistantResponse request body.
func NormalizeOrigin(origin string) string {
	switch origin {
	case "KIRO_AI_EDITOR":
		return "AI_EDITOR"
	case "AMAZON_Q":
		return "KIRO_CLI"
	case "KIRO_IDE":
		return "AI_EDITOR"
	default:
		return origin
	}
}

//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
	kiroopenai "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/openai"
)

func main() {
	body, _ := os.ReadFile("testdata_req.json")
	payload, _ := kiroopenai.BuildKiroPayloadFromOpenAI(body, "glm-5", "arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK", "AI_EDITOR", false, false, nil, nil)
	fmt.Println(string(payload))
}

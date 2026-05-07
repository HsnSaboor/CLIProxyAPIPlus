//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro/claude"
)

func main() {
	body, _ := os.ReadFile("testdata_req.json")
	payload, _ := claude.BuildKiroPayload(body, "glm-5", "arn", "AI_EDITOR", false, false, nil, nil)
	fmt.Println(string(payload))
}

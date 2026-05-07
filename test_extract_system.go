//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"github.com/tidwall/gjson"
	"os"
)

func main() {
	claudeBody, _ := os.ReadFile("testdata_req.json")
	systemField := gjson.GetBytes(claudeBody, "system")
	fmt.Printf("IsArray: %v\n", systemField.IsArray())
	if systemField.IsArray() {
		for _, block := range systemField.Array() {
			fmt.Printf("block type: %s\n", block.Type.String())
			fmt.Printf("block Get type: %s\n", block.Get("type").String())
			fmt.Printf("block text: %s\n", block.Get("text").String())
		}
	}
}

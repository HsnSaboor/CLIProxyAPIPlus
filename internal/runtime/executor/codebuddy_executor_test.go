package executor

import (
	"encoding/json"
	"testing"
)

func TestConvertCodeBuddyImageResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		model    string
		wantKeys []string
	}{
		{
			name:     "basic image response",
			input:    `{"data":[{"b64_json":"SGVsbG8=","revised_prompt":"A cute cat"}]}`,
			model:    "hunyuan-image-v3.0",
			wantKeys: []string{"object", "model", "created", "data"},
		},
		{
			name:     "multiple images",
			input:    `{"data":[{"b64_json":"AAAA","revised_prompt":"prompt1"},{"b64_json":"BBBB","revised_prompt":"prompt2"}]}`,
			model:    "gemini-3.0-pro-image",
			wantKeys: []string{"object", "model", "created", "data"},
		},
		{
			name:     "invalid json returns original",
			input:    `not valid json`,
			model:    "test-model",
			wantKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertCodeBuddyImageResponse([]byte(tt.input), tt.model)

			// If input was invalid, result should be unchanged
			if tt.wantKeys == nil {
				if string(result) != tt.input {
					t.Errorf("invalid json should return original, got %s", string(result))
				}
				return
			}

			// Parse result and verify keys
			var resp map[string]any
			if err := json.Unmarshal(result, &resp); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}

			for _, key := range tt.wantKeys {
				if _, ok := resp[key]; !ok {
					t.Errorf("missing expected key: %s", key)
				}
			}

			// Verify data array contains expected structure (if data was not empty)
			if data, ok := resp["data"].([]any); ok && len(data) > 0 {
				for i, img := range data {
					if imgMap, ok := img.(map[string]any); ok {
						if _, hasB64 := imgMap["b64_json"]; !hasB64 {
							t.Errorf("image %d missing b64_json", i)
						}
					}
				}
			}
		})
	}
}

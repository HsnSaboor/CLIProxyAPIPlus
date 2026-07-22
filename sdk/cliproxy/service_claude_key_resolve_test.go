package cliproxy

import (
	"testing"

	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

// TestResolveConfigClaudeKey_APIKeyEntries verifies that resolveConfigClaudeKey
// can locate the parent ClaudeKey provider block when the matching credential
// lives in a nested APIKeyEntries[] entry (multi-account pooling), not just in
// the legacy single-key top-level APIKey/BaseURL fields.
func TestResolveConfigClaudeKey_APIKeyEntries(t *testing.T) {
	service := &Service{
		cfg: &config.Config{
			ClaudeKey: []config.ClaudeKey{
				{
					Prefix:  "databricks-claude",
					BaseURL: "https://dbc-default.cloud.databricks.com/ai-gateway/anthropic",
					APIKeyEntries: []config.ClaudeKeyAPIKey{
						{APIKey: "dapi-account-1", BaseURL: "https://dbc-account-1.cloud.databricks.com/ai-gateway/anthropic"},
						{APIKey: "dapi-account-2"}, // no per-entry base-url; relies on provider-level default
					},
				},
				{
					APIKey:  "sk-ant-legacy-single-key",
					BaseURL: "https://api.anthropic.com",
				},
			},
		},
	}

	t.Run("matches nested entry with explicit base-url", func(t *testing.T) {
		auth := &coreauth.Auth{
			Attributes: map[string]string{
				"api_key":  "dapi-account-1",
				"base_url": "https://dbc-account-1.cloud.databricks.com/ai-gateway/anthropic",
			},
		}
		entry := service.resolveConfigClaudeKey(auth)
		if entry == nil {
			t.Fatal("expected a matching ClaudeKey provider entry, got nil")
		}
		if entry.Prefix != "databricks-claude" {
			t.Errorf("expected matched provider prefix databricks-claude, got %s", entry.Prefix)
		}
	})

	t.Run("matches nested entry falling back to provider base-url", func(t *testing.T) {
		auth := &coreauth.Auth{
			Attributes: map[string]string{
				"api_key":  "dapi-account-2",
				"base_url": "https://dbc-default.cloud.databricks.com/ai-gateway/anthropic",
			},
		}
		entry := service.resolveConfigClaudeKey(auth)
		if entry == nil {
			t.Fatal("expected a matching ClaudeKey provider entry, got nil")
		}
		if entry.Prefix != "databricks-claude" {
			t.Errorf("expected matched provider prefix databricks-claude, got %s", entry.Prefix)
		}
	})

	t.Run("matches nested entry by api_key only (no base_url attribute)", func(t *testing.T) {
		auth := &coreauth.Auth{
			Attributes: map[string]string{
				"api_key": "dapi-account-1",
			},
		}
		entry := service.resolveConfigClaudeKey(auth)
		if entry == nil {
			t.Fatal("expected a matching ClaudeKey provider entry, got nil")
		}
		if entry.Prefix != "databricks-claude" {
			t.Errorf("expected matched provider prefix databricks-claude, got %s", entry.Prefix)
		}
	})

	t.Run("still matches legacy single-key entries", func(t *testing.T) {
		auth := &coreauth.Auth{
			Attributes: map[string]string{
				"api_key":  "sk-ant-legacy-single-key",
				"base_url": "https://api.anthropic.com",
			},
		}
		entry := service.resolveConfigClaudeKey(auth)
		if entry == nil {
			t.Fatal("expected a matching ClaudeKey provider entry, got nil")
		}
		if entry.APIKey != "sk-ant-legacy-single-key" {
			t.Errorf("expected legacy single-key match, got %s", entry.APIKey)
		}
	})

	t.Run("no match returns nil", func(t *testing.T) {
		auth := &coreauth.Auth{
			Attributes: map[string]string{
				"api_key": "does-not-exist",
			},
		}
		if entry := service.resolveConfigClaudeKey(auth); entry != nil {
			t.Errorf("expected nil for unmatched api_key, got %+v", entry)
		}
	})
}

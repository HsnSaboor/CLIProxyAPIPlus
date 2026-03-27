package executor

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const codexAuthAffinityMetadataKey = "auth_affinity_key"

type codexContinuity struct {
	Key    string
	Source string
}

func resolveCodexContinuity(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) codexContinuity {
	if promptCacheKey := strings.TrimSpace(gjson.GetBytes(req.Payload, "prompt_cache_key").String()); promptCacheKey != "" {
		return codexContinuity{Key: promptCacheKey, Source: "prompt_cache_key"}
	}
	if opts.Metadata != nil {
		if raw, ok := opts.Metadata[cliproxyexecutor.ExecutionSessionMetadataKey]; ok && raw != nil {
			switch v := raw.(type) {
			case string:
				if trimmed := strings.TrimSpace(v); trimmed != "" {
					return codexContinuity{Key: trimmed, Source: "execution_session"}
				}
			case []byte:
				if trimmed := strings.TrimSpace(string(v)); trimmed != "" {
					return codexContinuity{Key: trimmed, Source: "execution_session"}
				}
			}
		}
	}
	if ginCtx := ginContextFrom(ctx); ginCtx != nil {
		if ginCtx.Request != nil {
			if v := strings.TrimSpace(ginCtx.GetHeader("Idempotency-Key")); v != "" {
				return codexContinuity{Key: v, Source: "idempotency_key"}
			}
		}
		if v, exists := ginCtx.Get("apiKey"); exists && v != nil {
			switch value := v.(type) {
			case string:
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return codexContinuity{Key: uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:prompt-cache:"+trimmed)).String(), Source: "client_principal"}
				}
			case fmt.Stringer:
				if trimmed := strings.TrimSpace(value.String()); trimmed != "" {
					return codexContinuity{Key: uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:prompt-cache:"+trimmed)).String(), Source: "client_principal"}
				}
			default:
				trimmed := strings.TrimSpace(fmt.Sprintf("%v", value))
				if trimmed != "" {
					return codexContinuity{Key: uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:prompt-cache:"+trimmed)).String(), Source: "client_principal"}
				}
			}
		}
	}
	if opts.Metadata != nil {
		if raw, ok := opts.Metadata[codexAuthAffinityMetadataKey]; ok && raw != nil {
			switch v := raw.(type) {
			case string:
				if trimmed := strings.TrimSpace(v); trimmed != "" {
					return codexContinuity{Key: trimmed, Source: "auth_affinity"}
				}
			case []byte:
				if trimmed := strings.TrimSpace(string(v)); trimmed != "" {
					return codexContinuity{Key: trimmed, Source: "auth_affinity"}
				}
			}
		}
	}
	if auth != nil {
		if authID := strings.TrimSpace(auth.ID); authID != "" {
			return codexContinuity{Key: uuid.NewSHA1(uuid.NameSpaceOID, []byte("cli-proxy-api:codex:prompt-cache:auth:"+authID)).String(), Source: "auth_id"}
		}
	}
	return codexContinuity{}
}

func applyCodexContinuityBody(rawJSON []byte, continuity codexContinuity) []byte {
	if continuity.Key == "" {
		return rawJSON
	}
	rawJSON, _ = sjson.SetBytes(rawJSON, "prompt_cache_key", continuity.Key)
	return rawJSON
}

func applyCodexContinuityHeaders(headers http.Header, continuity codexContinuity) {
	if headers == nil || continuity.Key == "" {
		return
	}
	headers.Set("session_id", continuity.Key)
}

func logCodexRequestDiagnostics(ctx context.Context, auth *cliproxyauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options, headers http.Header, body []byte, continuity codexContinuity) {
	if !log.IsLevelEnabled(log.DebugLevel) {
		return
	}
	entry := logWithRequestID(ctx)
	authID := ""
	authFile := ""
	if auth != nil {
		authID = strings.TrimSpace(auth.ID)
		authFile = strings.TrimSpace(auth.FileName)
	}
	selectedAuthID := ""
	executionSessionID := ""
	if opts.Metadata != nil {
		if raw, ok := opts.Metadata[cliproxyexecutor.SelectedAuthMetadataKey]; ok && raw != nil {
			switch v := raw.(type) {
			case string:
				selectedAuthID = strings.TrimSpace(v)
			case []byte:
				selectedAuthID = strings.TrimSpace(string(v))
			}
		}
		if raw, ok := opts.Metadata[cliproxyexecutor.ExecutionSessionMetadataKey]; ok && raw != nil {
			switch v := raw.(type) {
			case string:
				executionSessionID = strings.TrimSpace(v)
			case []byte:
				executionSessionID = strings.TrimSpace(string(v))
			}
		}
	}
	entry.Debugf(
		"codex request diagnostics auth_id=%s selected_auth_id=%s auth_file=%s exec_session=%s continuity_source=%s session_id=%s prompt_cache_key=%s prompt_cache_retention=%s store=%t has_instructions=%t reasoning_effort=%s reasoning_summary=%s chatgpt_account_id=%t originator=%s model=%s source_format=%s",
		authID,
		selectedAuthID,
		authFile,
		executionSessionID,
		continuity.Source,
		strings.TrimSpace(headers.Get("session_id")),
		gjson.GetBytes(body, "prompt_cache_key").String(),
		gjson.GetBytes(body, "prompt_cache_retention").String(),
		gjson.GetBytes(body, "store").Bool(),
		gjson.GetBytes(body, "instructions").Exists(),
		gjson.GetBytes(body, "reasoning.effort").String(),
		gjson.GetBytes(body, "reasoning.summary").String(),
		strings.TrimSpace(headers.Get("Chatgpt-Account-Id")) != "",
		strings.TrimSpace(headers.Get("Originator")),
		req.Model,
		opts.SourceFormat.String(),
	)
}

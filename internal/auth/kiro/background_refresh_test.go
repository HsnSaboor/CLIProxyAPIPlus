package kiro

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type testTokenRepo struct {
	updatedToken *Token
}

func (r *testTokenRepo) FindOldestUnverified(_ int) []*Token {
	return nil
}

func (r *testTokenRepo) UpdateToken(token *Token) error {
	copyToken := *token
	r.updatedToken = &copyToken
	return nil
}

func TestWithConfigSetsCLIOAuth(t *testing.T) {
	r := NewBackgroundRefresher(&testTokenRepo{}, WithConfig(nil))
	if r.cliOAuth == nil {
		t.Fatal("expected cliOAuth to be initialized")
	}
}

func TestRefreshSingle_UsesCLIOAuthForKiroCLI(t *testing.T) {
	repo := &testTokenRepo{}
	cliOAuth := &KiroCLIOAuth{
		httpClient: &http.Client{
			Transport: testRoundTripper(func(req *http.Request) (*http.Response, error) {
				respBody := `{"accessToken":"new-access","refreshToken":"new-refresh","profileArn":"arn:aws:iam::123456789012:role/test","expiresIn":3600}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(respBody)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	r := NewBackgroundRefresher(repo)
	r.cliOAuth = cliOAuth

	token := &Token{
		ID:           "kiro-cli-test.json",
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
		AuthMethod:   "kiro-cli",
	}

	r.refreshSingle(context.Background(), token)

	if repo.updatedToken == nil {
		t.Fatal("expected token update to be persisted")
	}
	if repo.updatedToken.AccessToken != "new-access" {
		t.Fatalf("expected refreshed access token, got %q", repo.updatedToken.AccessToken)
	}
	if repo.updatedToken.RefreshToken != "new-refresh" {
		t.Fatalf("expected refreshed refresh token, got %q", repo.updatedToken.RefreshToken)
	}
}

func TestRefreshSingle_KiroCLIWithoutCLIOAuthUsesFallback(t *testing.T) {
	repo := &testTokenRepo{}
	r := NewBackgroundRefresher(repo)

	token := &Token{
		ID:           "kiro-cli-test.json",
		AccessToken:  "still-valid-token",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		AuthMethod:   "kiro-cli",
	}

	r.refreshSingle(context.Background(), token)

	if repo.updatedToken != nil {
		t.Fatal("did not expect persisted update when fallback token is used")
	}
}

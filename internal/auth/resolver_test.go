package auth

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/cncf/cora/internal/config"
)

func newGETRequest(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return &http.Request{
		Method: http.MethodGet,
		URL:    u,
		Header: make(http.Header),
	}
}

// ── InjectAuth: Discourse ────────────────────────────────────────────────────

func TestInjectAuth_Discourse_InjectsHeaders(t *testing.T) {
	req := newGETRequest(t, "https://forum.example.com/posts.json")
	svc := config.ServiceConfig{
		Auth: config.AuthConfig{
			Discourse: &config.DiscourseAuth{
				APIKey:      "secret-key",
				APIUsername: "system",
			},
		},
	}
	InjectAuth(req, svc, "forum")

	if got := req.Header.Get("Api-Key"); got != "secret-key" {
		t.Errorf("Api-Key = %q, want %q", got, "secret-key")
	}
	if got := req.Header.Get("Api-Username"); got != "system" {
		t.Errorf("Api-Username = %q, want %q", got, "system")
	}
}

func TestInjectAuth_Discourse_SkipsEmptyFields(t *testing.T) {
	req := newGETRequest(t, "https://forum.example.com/posts.json")
	svc := config.ServiceConfig{
		Auth: config.AuthConfig{
			Discourse: &config.DiscourseAuth{APIKey: "", APIUsername: ""},
		},
	}
	InjectAuth(req, svc, "forum")

	if v := req.Header.Get("Api-Key"); v != "" {
		t.Errorf("expected no Api-Key header, got %q", v)
	}
	if v := req.Header.Get("Api-Username"); v != "" {
		t.Errorf("expected no Api-Username header, got %q", v)
	}
}

func TestInjectAuth_Discourse_NilSkipped(t *testing.T) {
	req := newGETRequest(t, "https://forum.example.com/posts.json")
	svc := config.ServiceConfig{Auth: config.AuthConfig{Discourse: nil}}
	InjectAuth(req, svc, "forum")

	if v := req.Header.Get("Api-Key"); v != "" {
		t.Errorf("expected no Api-Key header, got %q", v)
	}
}

// ── InjectAuth: Etherpad ─────────────────────────────────────────────────────

func TestInjectAuth_Etherpad_InjectsQueryParam(t *testing.T) {
	req := newGETRequest(t, "https://pad.example.com/api/1.3.0/getText")
	svc := config.ServiceConfig{
		Auth: config.AuthConfig{
			Etherpad: &config.EtherpadAuth{APIKey: "padkey123"},
		},
	}
	InjectAuth(req, svc, "etherpad")

	q := req.URL.Query()
	if got := q.Get("apikey"); got != "padkey123" {
		t.Errorf("apikey = %q, want %q", got, "padkey123")
	}
}

func TestInjectAuth_Etherpad_SkipsEmptyKey(t *testing.T) {
	req := newGETRequest(t, "https://pad.example.com/api/getText")
	svc := config.ServiceConfig{
		Auth: config.AuthConfig{
			Etherpad: &config.EtherpadAuth{APIKey: ""},
		},
	}
	InjectAuth(req, svc, "etherpad")

	if v := req.URL.Query().Get("apikey"); v != "" {
		t.Errorf("expected no apikey param, got %q", v)
	}
}

func TestInjectAuth_Etherpad_PreservesExistingQueryParams(t *testing.T) {
	req := newGETRequest(t, "https://pad.example.com/api/getText?padID=test")
	svc := config.ServiceConfig{
		Auth: config.AuthConfig{
			Etherpad: &config.EtherpadAuth{APIKey: "k"},
		},
	}
	InjectAuth(req, svc, "etherpad")

	q := req.URL.Query()
	if got := q.Get("padID"); got != "test" {
		t.Errorf("padID = %q, want %q", got, "test")
	}
	if got := q.Get("apikey"); got != "k" {
		t.Errorf("apikey = %q, want %q", got, "k")
	}
}

// ── InjectAuth: GitCode ──────────────────────────────────────────────────────

func TestInjectAuth_Gitcode_InjectsQueryParam(t *testing.T) {
	req := newGETRequest(t, "https://api.gitcode.com/api/v5/repos/owner/repo/issues")
	svc := config.ServiceConfig{
		Auth: config.AuthConfig{
			Gitcode: &config.GitcodeAuth{AccessToken: "my-token"},
		},
	}
	InjectAuth(req, svc, "gitcode")

	if got := req.URL.Query().Get("access_token"); got != "my-token" {
		t.Errorf("access_token = %q, want %q", got, "my-token")
	}
}

func TestInjectAuth_Gitcode_SkipsEmptyToken(t *testing.T) {
	req := newGETRequest(t, "https://api.gitcode.com/issues")
	svc := config.ServiceConfig{
		Auth: config.AuthConfig{
			Gitcode: &config.GitcodeAuth{AccessToken: ""},
		},
	}
	InjectAuth(req, svc, "gitcode")

	if v := req.URL.Query().Get("access_token"); v != "" {
		t.Errorf("expected no access_token param, got %q", v)
	}
}

func TestInjectAuth_Gitcode_NilSkipped(t *testing.T) {
	req := newGETRequest(t, "https://api.gitcode.com/issues")
	svc := config.ServiceConfig{Auth: config.AuthConfig{Gitcode: nil}}
	InjectAuth(req, svc, "gitcode")

	if v := req.URL.Query().Get("access_token"); v != "" {
		t.Errorf("expected no access_token param, got %q", v)
	}
}

// ── InjectAuth: GitHub ───────────────────────────────────────────────────────

func TestInjectAuth_Github_InjectsBearerHeader(t *testing.T) {
	req := newGETRequest(t, "https://api.github.com/repos/cncf/cora/issues")
	svc := config.ServiceConfig{
		Auth: config.AuthConfig{
			Github: &config.GithubAuth{Token: "ghp_secret"},
		},
	}
	InjectAuth(req, svc, "github")

	if got := req.Header.Get("Authorization"); got != "Bearer ghp_secret" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer ghp_secret")
	}
	if got := req.Header.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
		t.Errorf("X-GitHub-Api-Version = %q, want %q", got, "2022-11-28")
	}
}

func TestInjectAuth_Github_SkipsEmptyToken(t *testing.T) {
	req := newGETRequest(t, "https://api.github.com/repos/cncf/cora")
	svc := config.ServiceConfig{
		Auth: config.AuthConfig{
			Github: &config.GithubAuth{Token: ""},
		},
	}
	InjectAuth(req, svc, "github")

	if v := req.Header.Get("Authorization"); v != "" {
		t.Errorf("expected no Authorization header, got %q", v)
	}
	if v := req.Header.Get("X-GitHub-Api-Version"); v != "" {
		t.Errorf("expected no X-GitHub-Api-Version header, got %q", v)
	}
}

func TestInjectAuth_Github_NilSkipped(t *testing.T) {
	req := newGETRequest(t, "https://api.github.com/repos/cncf/cora")
	svc := config.ServiceConfig{Auth: config.AuthConfig{Github: nil}}
	InjectAuth(req, svc, "github")

	if v := req.Header.Get("Authorization"); v != "" {
		t.Errorf("expected no Authorization header, got %q", v)
	}
}

// ── IsDiscourseAuthParam ─────────────────────────────────────────────────────

func TestIsDiscourseAuthParam(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"Api-Key", true},
		{"Api-Username", true},
		{"Authorization", false},
		{"access_token", false},
		{"apikey", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsDiscourseAuthParam(tc.name); got != tc.want {
			t.Errorf("IsDiscourseAuthParam(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// ── IsGitcodeAuthParam ───────────────────────────────────────────────────────

func TestIsGitcodeAuthParam(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"access_token", true},
		{"Authorization", true},
		{"Api-Key", false},
		{"apikey", false},
		{"token", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsGitcodeAuthParam(tc.name); got != tc.want {
			t.Errorf("IsGitcodeAuthParam(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

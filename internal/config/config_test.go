package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeConfig writes content to a temp file and returns the path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// ── LoadFrom: basic loading ──────────────────────────────────────────────────

func TestLoadFrom_ValidYAML(t *testing.T) {
	path := writeConfig(t, `
services:
  gitcode:
    base_url: https://api.gitcode.com
    auth:
      gitcode:
        access_token: mytoken
`)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc, ok := cfg.Services["gitcode"]
	if !ok {
		t.Fatal("expected gitcode service")
	}
	if svc.BaseURL != "https://api.gitcode.com" {
		t.Errorf("BaseURL = %q, want %q", svc.BaseURL, "https://api.gitcode.com")
	}
	if svc.Auth.Gitcode == nil || svc.Auth.Gitcode.AccessToken != "mytoken" {
		t.Errorf("Gitcode.AccessToken = %v, want %q", svc.Auth.Gitcode, "mytoken")
	}
}

func TestLoadFrom_MissingFile_NotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("missing config file should not be an error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestLoadFrom_InvalidYAML_ReturnsError(t *testing.T) {
	path := writeConfig(t, `services: [not a map`)
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

// ── LoadFrom: defaults ───────────────────────────────────────────────────────

func TestLoadFrom_DefaultTTL(t *testing.T) {
	path := writeConfig(t, "services: {}")
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SpecCache.TTL != DefaultCacheTTL {
		t.Errorf("TTL = %v, want %v", cfg.SpecCache.TTL, DefaultCacheTTL)
	}
}

func TestLoadFrom_DefaultCacheDir(t *testing.T) {
	path := writeConfig(t, "services: {}")
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SpecCache.Dir == "" {
		t.Error("expected non-empty SpecCache.Dir")
	}
}

func TestLoadFrom_ServicesMapNotNilOnEmptyFile(t *testing.T) {
	path := writeConfig(t, "")
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services == nil {
		t.Error("Services map should not be nil")
	}
}

// ── LoadFrom: env var overrides ──────────────────────────────────────────────

func TestLoadFrom_EnvOverridesBaseURL(t *testing.T) {
	path := writeConfig(t, `
services:
  forum:
    base_url: http://original.example.com
`)
	t.Setenv("CORA_SERVICES_FORUM_BASE_URL", "http://override.example.com")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services["forum"].BaseURL != "http://override.example.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.Services["forum"].BaseURL, "http://override.example.com")
	}
}

func TestLoadFrom_EnvOverridesDiscourseAPIKey(t *testing.T) {
	path := writeConfig(t, `
services:
  forum:
    base_url: http://forum.example.com
    auth:
      discourse:
        api_key: original-key
        api_username: system
`)
	t.Setenv("CORA_SERVICES_FORUM_AUTH_DISCOURSE_API_KEY", "env-key")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d := cfg.Services["forum"].Auth.Discourse
	if d == nil {
		t.Fatal("expected discourse auth")
	}
	if d.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want %q", d.APIKey, "env-key")
	}
}

func TestLoadFrom_EnvOverridesTTL(t *testing.T) {
	path := writeConfig(t, "services: {}")
	t.Setenv("CORA_SPEC_CACHE_TTL", "12h")

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SpecCache.TTL != 12*time.Hour {
		t.Errorf("TTL = %v, want 12h", cfg.SpecCache.TTL)
	}
}

func TestLoadFrom_EnvOverridesCacheDir(t *testing.T) {
	path := writeConfig(t, "services: {}")
	customDir := t.TempDir()
	t.Setenv("CORA_SPEC_CACHE_DIR", customDir)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SpecCache.Dir != customDir {
		t.Errorf("Dir = %q, want %q", cfg.SpecCache.Dir, customDir)
	}
}

// ── LoadFrom: multiple services ──────────────────────────────────────────────

func TestLoadFrom_MultipleServices(t *testing.T) {
	path := writeConfig(t, `
services:
  gitcode:
    base_url: https://api.gitcode.com
  etherpad:
    base_url: https://pad.example.com
`)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) < 2 {
		t.Errorf("expected at least 2 services, got %d", len(cfg.Services))
	}
	if cfg.Services["gitcode"].BaseURL != "https://api.gitcode.com" {
		t.Errorf("gitcode BaseURL = %q", cfg.Services["gitcode"].BaseURL)
	}
	if cfg.Services["etherpad"].BaseURL != "https://pad.example.com" {
		t.Errorf("etherpad BaseURL = %q", cfg.Services["etherpad"].BaseURL)
	}
}

// ── loadDotEnv ───────────────────────────────────────────────────────────────

func TestLoadDotEnv_AppliesValues(t *testing.T) {
	// Write a .env into a temp dir and set CWD to that dir.
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("CORA_TEST_DOTENV=hello\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	// Ensure the var is not set before calling loadDotEnv.
	os.Unsetenv("CORA_TEST_DOTENV")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	loadDotEnv()

	if got := os.Getenv("CORA_TEST_DOTENV"); got != "hello" {
		t.Errorf("CORA_TEST_DOTENV = %q, want %q", got, "hello")
	}
	// Cleanup so other tests are not affected.
	os.Unsetenv("CORA_TEST_DOTENV")
}

func TestLoadDotEnv_ExistingEnvWins(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("CORA_TEST_WINS=from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("CORA_TEST_WINS", "from-env")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	loadDotEnv()

	if got := os.Getenv("CORA_TEST_WINS"); got != "from-env" {
		t.Errorf("CORA_TEST_WINS = %q, want %q (existing env should win)", got, "from-env")
	}
}

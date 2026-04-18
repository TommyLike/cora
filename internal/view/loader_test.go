package view

import (
	"os"
	"path/filepath"
	"testing"
)

func writeViews(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "views.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write views.yaml: %v", err)
	}
	return path
}

// ── LoadRegistry: missing file ────────────────────────────────────────────────

func TestLoadRegistry_MissingFile_NotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")
	result := LoadRegistry(path)

	if result.Err != nil {
		t.Errorf("expected no error, got %v", result.Err)
	}
	if result.Loaded {
		t.Error("Loaded should be false for missing file")
	}
	if result.Registry == nil {
		t.Error("Registry should not be nil even when file is missing")
	}
}

// ── LoadRegistry: malformed YAML ─────────────────────────────────────────────

func TestLoadRegistry_MalformedYAML_ReturnsErr(t *testing.T) {
	path := writeViews(t, `gitcode: [not-a-map`)
	result := LoadRegistry(path)

	if result.Err == nil {
		t.Error("expected parse error for malformed YAML")
	}
	if result.Loaded {
		t.Error("Loaded should be false when parse fails")
	}
	// Built-ins still available.
	if result.Registry == nil {
		t.Error("Registry should not be nil even on parse error")
	}
}

// ── LoadRegistry: valid YAML ──────────────────────────────────────────────────

func TestLoadRegistry_ValidYAML_Loaded(t *testing.T) {
	path := writeViews(t, `
testservice:
  items/list:
    root_field: items
    columns:
      - field: id
        label: ID
`)
	result := LoadRegistry(path)

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if !result.Loaded {
		t.Error("Loaded should be true for valid file")
	}
	if result.ResolvedPath != path {
		t.Errorf("ResolvedPath = %q, want %q", result.ResolvedPath, path)
	}

	cfg := result.Registry.Lookup("testservice", "items", "list")
	if cfg == nil {
		t.Fatal("expected ViewConfig for testservice/items/list")
	}
	if cfg.RootField != "items" {
		t.Errorf("RootField = %q, want %q", cfg.RootField, "items")
	}
}

// ── LoadRegistry: built-in fallback always present ────────────────────────────

func TestLoadRegistry_BuiltinsAlwaysPresent(t *testing.T) {
	// No views file at all.
	result := LoadRegistry(filepath.Join(t.TempDir(), "nonexistent.yaml"))

	// gitcode issues/list is a built-in; should be present.
	cfg := result.Registry.Lookup("gitcode", "issues", "list")
	if cfg == nil {
		t.Error("expected built-in gitcode/issues/list to be present without views.yaml")
	}
}

// ── LoadRegistry: user overrides built-in ────────────────────────────────────

func TestLoadRegistry_UserOverridesBuiltin(t *testing.T) {
	// gitcode issues/list is a built-in; override one column.
	path := writeViews(t, `
gitcode:
  issues/list:
    columns:
      - field: custom_field
        label: Custom
`)
	result := LoadRegistry(path)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	cfg := result.Registry.Lookup("gitcode", "issues", "list")
	if cfg == nil {
		t.Fatal("expected ViewConfig")
	}
	if len(cfg.Columns) != 1 || cfg.Columns[0].Field != "custom_field" {
		t.Errorf("expected user override, got columns: %v", cfg.Columns)
	}
}

// ── resolveViewsPath ──────────────────────────────────────────────────────────

func TestResolveViewsPath_ExplicitPath(t *testing.T) {
	os.Unsetenv(EnvViewsPath)
	got := resolveViewsPath("/explicit/path.yaml")
	if got != "/explicit/path.yaml" {
		t.Errorf("resolveViewsPath = %q, want %q", got, "/explicit/path.yaml")
	}
}

func TestResolveViewsPath_EnvVar(t *testing.T) {
	t.Setenv(EnvViewsPath, "/from/env.yaml")
	got := resolveViewsPath("")
	if got != "/from/env.yaml" {
		t.Errorf("resolveViewsPath = %q, want %q", got, "/from/env.yaml")
	}
}

func TestResolveViewsPath_DefaultPath(t *testing.T) {
	os.Unsetenv(EnvViewsPath)
	got := resolveViewsPath("")
	if got == "" {
		t.Error("expected non-empty default path")
	}
	// Should end with the expected filename.
	if filepath.Base(got) != "views.yaml" {
		t.Errorf("default path filename = %q, want views.yaml", filepath.Base(got))
	}
}

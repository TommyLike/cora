package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- writeCache / readCache round-trip ---

func TestWriteCache_and_ReadCache_roundtrip(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test_spec.json")

	rawSpec := json.RawMessage(`{"openapi":"3.1.0","info":{"title":"Test","version":"v1"}}`)

	if err := writeCache(file, "https://example.org/openapi.json", rawSpec); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	entry, err := readCache(file)
	if err != nil {
		t.Fatalf("readCache: %v", err)
	}
	if entry.SpecURL != "https://example.org/openapi.json" {
		t.Errorf("SpecURL = %q, want %q", entry.SpecURL, "https://example.org/openapi.json")
	}
	if string(entry.RawSpec) != string(rawSpec) {
		t.Errorf("RawSpec mismatch: got %s, want %s", entry.RawSpec, rawSpec)
	}
	if time.Since(entry.FetchedAt) > 5*time.Second {
		t.Errorf("FetchedAt too old: %v", entry.FetchedAt)
	}
}

func TestWriteCache_createsParentDirectories(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "sub", "nested", "spec.json")
	rawSpec := json.RawMessage(`{"openapi":"3.1.0"}`)

	if err := writeCache(file, "http://x", rawSpec); err != nil {
		t.Fatalf("writeCache: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("cache file not created: %v", err)
	}
}

func TestWriteCache_isAtomic(t *testing.T) {
	// After writeCache, no .tmp sibling should exist.
	dir := t.TempDir()
	file := filepath.Join(dir, "spec.json")
	_ = writeCache(file, "http://x", json.RawMessage(`{}`))

	tmp := file + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("tmp file %q should not exist after successful write", tmp)
	}
}

func TestReadCache_missingFile_returnsError(t *testing.T) {
	_, err := readCache("/nonexistent/path/spec.json")
	if err == nil {
		t.Error("expected error for missing cache file, got nil")
	}
}

func TestReadCache_corruptJSON_returnsError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "corrupt.json")
	_ = os.WriteFile(file, []byte("not-json{{{"), 0o600)

	_, err := readCache(file)
	if err == nil {
		t.Error("expected error for corrupt JSON, got nil")
	}
}

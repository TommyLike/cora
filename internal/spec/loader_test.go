package spec

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// minimalSpec is a valid OpenAPI 3.1.0 document used across tests.
const minimalSpec = `{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "v1"},
  "paths": {}
}`

// --- fetch from local file ---

func TestLoader_Load_fromLocalFile(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "openapi.json")
	_ = os.WriteFile(specFile, []byte(minimalSpec), 0o600)

	loader := &Loader{
		SpecURL:   specFile,
		CacheFile: filepath.Join(dir, "cache.json"),
		TTL:       24 * time.Hour,
	}

	doc, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("Title = %q, want %q", doc.Info.Title, "Test API")
	}
}

func TestLoader_Load_fromFileSchemeURL(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "openapi.json")
	_ = os.WriteFile(specFile, []byte(minimalSpec), 0o600)

	loader := &Loader{
		SpecURL:   "file://" + specFile,
		CacheFile: filepath.Join(dir, "cache.json"),
		TTL:       24 * time.Hour,
	}

	doc, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.Info.Version != "v1" {
		t.Errorf("Version = %q, want %q", doc.Info.Version, "v1")
	}
}

// --- fetch from HTTP ---

func TestLoader_Load_fromHTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(minimalSpec))
	}))
	defer srv.Close()

	dir := t.TempDir()
	loader := &Loader{
		SpecURL:   srv.URL + "/openapi.json",
		CacheFile: filepath.Join(dir, "cache.json"),
		TTL:       24 * time.Hour,
	}

	doc, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load from HTTP: %v", err)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("Title = %q, want %q", doc.Info.Title, "Test API")
	}
}

// --- cache hit (tier 1) ---

func TestLoader_Load_usesFreshCache(t *testing.T) {
	dir := t.TempDir()
	cacheFile := filepath.Join(dir, "cache.json")

	// Pre-populate a fresh cache; SpecURL points to a non-existent file to
	// prove the cache is used and no fetch attempt is made.
	_ = writeCache(cacheFile, "file:///nonexistent.json", []byte(minimalSpec))

	loader := &Loader{
		SpecURL:   "file:///nonexistent.json",
		CacheFile: cacheFile,
		TTL:       24 * time.Hour,
	}

	doc, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("expected cache hit, got error: %v", err)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("Title = %q, want %q", doc.Info.Title, "Test API")
	}
}

// --- stale cache fallback (tier 3) ---

func TestLoader_Load_usesStaleCache_whenFetchFails(t *testing.T) {
	dir := t.TempDir()
	cacheFile := filepath.Join(dir, "cache.json")

	// Write an already-expired cache entry.
	entry := cacheEntry{
		FetchedAt: time.Now().Add(-48 * time.Hour), // 48h ago — expired
		SpecURL:   "http://unreachable.invalid/openapi.json",
		RawSpec:   json.RawMessage(minimalSpec),
	}
	data, _ := json.Marshal(entry)
	_ = os.WriteFile(cacheFile, data, 0o600)

	loader := &Loader{
		SpecURL:   "http://unreachable.invalid/openapi.json",
		CacheFile: cacheFile,
		TTL:       24 * time.Hour,
	}

	doc, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("expected stale cache fallback, got error: %v", err)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("Title = %q, want %q", doc.Info.Title, "Test API")
	}
}

// --- no cache + fetch fails (tier 4) ---

func TestLoader_Load_returnsError_whenNoCacheAndFetchFails(t *testing.T) {
	dir := t.TempDir()
	loader := &Loader{
		SpecURL:   "http://unreachable.invalid/openapi.json",
		CacheFile: filepath.Join(dir, "cache.json"),
		TTL:       24 * time.Hour,
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error when fetch fails and no cache exists")
	}
}

// --- Invalidate ---

func TestLoader_Invalidate_removesCacheFile(t *testing.T) {
	dir := t.TempDir()
	cacheFile := filepath.Join(dir, "cache.json")
	_ = os.WriteFile(cacheFile, []byte(`{}`), 0o600)

	loader := &Loader{CacheFile: cacheFile, TTL: time.Hour}
	if err := loader.Invalidate(); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Error("cache file should be removed after Invalidate")
	}
}

func TestLoader_Invalidate_noopWhenCacheAbsent(t *testing.T) {
	dir := t.TempDir()
	loader := &Loader{
		CacheFile: filepath.Join(dir, "nonexistent.json"),
		TTL:       time.Hour,
	}
	if err := loader.Invalidate(); err != nil {
		t.Errorf("Invalidate on missing file should be a no-op, got: %v", err)
	}
}

// --- HTTP server returns non-200 ---

func TestLoader_Load_httpNon200_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	loader := &Loader{
		SpecURL:   srv.URL + "/openapi.json",
		CacheFile: filepath.Join(dir, "cache.json"),
		TTL:       24 * time.Hour,
	}

	_, err := loader.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
}

// --- cache is written after successful fetch ---

func TestLoader_Load_writesCacheAfterFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(minimalSpec))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cacheFile := filepath.Join(dir, "cache.json")
	loader := &Loader{
		SpecURL:   srv.URL + "/openapi.json",
		CacheFile: cacheFile,
		TTL:       24 * time.Hour,
	}

	if _, err := loader.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := os.Stat(cacheFile); err != nil {
		t.Errorf("cache file not written after fetch: %v", err)
	}
}

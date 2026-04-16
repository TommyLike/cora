package spec

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/cncf/cora/pkg/errs"
)

// Loader loads an OpenAPI spec from a URL or local file, using a local cache
// to avoid repeated network requests.
//
// Three-tier priority (ADR-0004):
//  1. Cache exists AND fetched_at within TTL  →  return cached, no network call
//  2. Cache missing or expired               →  fetch, write cache, return fresh
//  3. Fetch fails AND stale cache available  →  return stale + stderr warning
type Loader struct {
	// SpecURL is the canonical source: http(s)://... or file://... or bare path.
	// Empty when the spec is provided via FallbackData (built-in services).
	SpecURL string
	// FallbackData holds embedded spec bytes for built-in services.
	// Used by fetch() when SpecURL is empty.
	FallbackData []byte
	CacheFile    string
	TTL          time.Duration
}

// NewLoader builds a Loader for the given service.
// cacheDir is the directory where "<service>_spec.json" will live.
func NewLoader(svcName, specURL, cacheDir string, ttl time.Duration) *Loader {
	return &Loader{
		SpecURL:   specURL,
		CacheFile: filepath.Join(cacheDir, svcName+"_spec.json"),
		TTL:       ttl,
	}
}

// NewEmbeddedLoader builds a Loader whose spec source is the provided bytes
// (typically embedded via go:embed). The cache is still written and used for
// performance, but no network call is ever made.
func NewEmbeddedLoader(svcName string, data []byte, cacheDir string, ttl time.Duration) *Loader {
	return &Loader{
		FallbackData: data,
		CacheFile:    filepath.Join(cacheDir, svcName+"_spec.json"),
		TTL:          ttl,
	}
}

// Load returns the OpenAPI document, preferring the local cache when fresh.
func (l *Loader) Load(ctx context.Context) (*openapi3.T, error) {
	// --- Tier 1: fresh cache ---
	if entry, err := readCache(l.CacheFile); err == nil {
		if time.Since(entry.FetchedAt) < l.TTL {
			return parseSpec(entry.RawSpec)
		}
	}

	// --- Tier 2: fetch from source ---
	raw, fetchErr := l.fetch(ctx)
	if fetchErr == nil {
		// Best-effort cache write; ignore error so we still return the spec
		_ = writeCache(l.CacheFile, l.SpecURL, raw)
		return parseSpec(raw)
	}

	// --- Tier 3: stale cache fallback ---
	if entry, err := readCache(l.CacheFile); err == nil {
		fmt.Fprintf(os.Stderr,
			"[warn] could not refresh spec for %q (%v); using cached version from %s\n",
			l.SpecURL, fetchErr, entry.FetchedAt.Format(time.RFC3339),
		)
		return parseSpec(entry.RawSpec)
	}

	return nil, errs.NewSpecError(l.SpecURL, fetchErr)
}

// LoadCached reads the spec from the local cache file only — no network call is
// made. Returns (nil, zero, nil) when no cache file exists yet.
func (l *Loader) LoadCached() (*openapi3.T, time.Time, error) {
	entry, err := readCache(l.CacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, err
	}
	doc, err := parseSpec(entry.RawSpec)
	if err != nil {
		return nil, time.Time{}, err
	}
	return doc, entry.FetchedAt, nil
}

// Invalidate removes the local cache file, forcing a fresh fetch on next Load.
func (l *Loader) Invalidate() error {
	err := os.Remove(l.CacheFile)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// fetch retrieves raw spec bytes from the configured SpecURL.
// Supports http://, https://, file://, bare file paths, and embedded data.
func (l *Loader) fetch(ctx context.Context) ([]byte, error) {
	// Embedded spec (built-in services with no SpecURL).
	if l.SpecURL == "" {
		if l.FallbackData != nil {
			return l.FallbackData, nil
		}
		return nil, fmt.Errorf("no SpecURL and no FallbackData configured")
	}

	u := l.SpecURL

	// file:// scheme or bare path
	if strings.HasPrefix(u, "file://") {
		path := strings.TrimPrefix(u, "file://")
		return os.ReadFile(path)
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		// treat as a bare filesystem path
		return os.ReadFile(u)
	}

	// HTTP/HTTPS
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json, application/yaml, */*")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// parseSpec parses raw bytes into an openapi3.T document.
func parseSpec(raw []byte) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromData(raw)
	if err != nil {
		return nil, fmt.Errorf("parse OpenAPI spec: %w", err)
	}
	return doc, nil
}

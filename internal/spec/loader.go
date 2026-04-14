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
	SpecURL   string
	CacheFile string
	TTL       time.Duration
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

// Invalidate removes the local cache file, forcing a fresh fetch on next Load.
func (l *Loader) Invalidate() error {
	err := os.Remove(l.CacheFile)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// fetch retrieves raw spec bytes from the configured SpecURL.
// Supports http://, https://, file://, and bare file paths.
func (l *Loader) fetch(ctx context.Context) ([]byte, error) {
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

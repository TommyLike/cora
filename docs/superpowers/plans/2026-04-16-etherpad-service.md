# Etherpad Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Etherpad as a built-in service so users can call the Etherpad API via `cora etherpad <resource> <verb>`, with the API key stored once in the config file.

**Architecture:** Etherpad is registered as a built-in service whose OpenAPI spec is embedded in the binary (via `go:embed`), so no network fetch is required for the spec. The API key is read from `~/.config/cora/config.yaml` and automatically injected as the `apikey` query parameter on every request. No flag needs to be passed by the user.

**Tech Stack:** Go 1.22, `go:embed`, cobra, kin-openapi/openapi3, github.com/cncf/cora (existing)

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| CREATE | `assets/openapi/etherpad/openapi.json` | Etherpad OpenAPI spec downloaded from upstream |
| CREATE | `assets/assets.go` | `package assets` — exposes `EtherpadSpec []byte` via `go:embed` |
| MODIFY | `internal/spec/loader.go` | Add `FallbackData []byte` field + `NewEmbeddedLoader()` constructor |
| MODIFY | `internal/spec/loader_test.go` | Add test for embedded loader |
| CREATE | `internal/registry/builtin.go` | `registerBuiltins()` — wires etherpad entry + default cfg injection |
| MODIFY | `internal/registry/registry.go` | Call `registerBuiltins()` at start of `New()`; allow config-only base_url override |
| MODIFY | `internal/config/config.go` | Add `EtherpadAuth` struct + field in `AuthConfig` |
| MODIFY | `internal/auth/resolver.go` | Inject `?apikey=` into URL for etherpad; rename func to `InjectAuth` |
| MODIFY | `internal/executor/executor.go` | Call `auth.InjectAuth` (rename) |

---

## Task 1 — Download Etherpad OpenAPI spec

**Files:**
- Create: `assets/openapi/etherpad/openapi.json`

- [ ] **Step 1: Fetch and save the spec**

```bash
curl -fsSL https://etherpad.openeuler.org/api/openapi.json \
  -o /Users/husheng/codes/cora/assets/openapi/etherpad/openapi.json
```

Expected: file created, valid JSON.

- [ ] **Step 2: Verify it parsed correctly**

```bash
cd /Users/husheng/codes/cora
python3 -c "import json,sys; d=json.load(open('assets/openapi/etherpad/openapi.json')); print('paths:', len(d['paths']), 'version:', d['info']['version'])"
```

Expected output: `paths: <N>  version: 1.3.0`

- [ ] **Step 3: Commit**

```bash
cd /Users/husheng/codes/cora
git add assets/openapi/etherpad/openapi.json
git commit -m "chore: add Etherpad OpenAPI spec snapshot"
```

---

## Task 2 — Create assets embed package

**Files:**
- Create: `assets/assets.go`

- [ ] **Step 1: Write the embed file**

```go
// assets/assets.go
package assets

import _ "embed"

// EtherpadSpec is the OpenAPI spec for the Etherpad service,
// embedded at build time from assets/openapi/etherpad/openapi.json.
//
//go:embed openapi/etherpad/openapi.json
var EtherpadSpec []byte
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /Users/husheng/codes/cora
go build ./assets/...
```

Expected: no output (clean build).

- [ ] **Step 3: Commit**

```bash
git add assets/assets.go
git commit -m "feat: add assets embed package for service specs"
```

---

## Task 3 — Extend Loader with embedded spec support

**Files:**
- Modify: `internal/spec/loader.go`
- Modify: `internal/spec/loader_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/spec/loader_test.go` (after the existing tests):

```go
// --- embedded spec (FallbackData) ---

func TestLoader_NewEmbeddedLoader_returnsSpec(t *testing.T) {
	dir := t.TempDir()
	loader := NewEmbeddedLoader("svc", []byte(minimalSpec), dir, 24*time.Hour)

	doc, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load from embedded data: %v", err)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("Title = %q, want %q", doc.Info.Title, "Test API")
	}
}

func TestLoader_NewEmbeddedLoader_writesCacheOnFirstLoad(t *testing.T) {
	dir := t.TempDir()
	loader := NewEmbeddedLoader("svc", []byte(minimalSpec), dir, 24*time.Hour)

	if _, err := loader.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := os.Stat(loader.CacheFile); err != nil {
		t.Errorf("cache file not created after first load: %v", err)
	}
}

func TestLoader_NewEmbeddedLoader_usesCacheOnSubsequentLoad(t *testing.T) {
	dir := t.TempDir()
	// Use invalid embedded data to prove the second load hits cache, not FallbackData.
	loader := NewEmbeddedLoader("svc", []byte(minimalSpec), dir, 24*time.Hour)

	// First load populates cache.
	if _, err := loader.Load(context.Background()); err != nil {
		t.Fatalf("first Load: %v", err)
	}

	// Corrupt FallbackData; second load must still succeed via cache.
	loader.FallbackData = []byte("not valid json")
	doc, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("second Load (expected cache hit): %v", err)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("Title = %q, want %q", doc.Info.Title, "Test API")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd /Users/husheng/codes/cora
go test ./internal/spec/... -run TestLoader_NewEmbeddedLoader -v
```

Expected: `FAIL` — `NewEmbeddedLoader undefined`.

- [ ] **Step 3: Implement FallbackData and NewEmbeddedLoader in loader.go**

In `internal/spec/loader.go`, add `FallbackData` field to the `Loader` struct and a new constructor. Also update `fetch()` to check `FallbackData` first.

Replace the existing `Loader` struct and `NewLoader` function with:

```go
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
```

Update the `fetch()` method to use `FallbackData` when `SpecURL` is empty:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/husheng/codes/cora
go test ./internal/spec/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/spec/loader.go internal/spec/loader_test.go
git commit -m "feat(spec): add NewEmbeddedLoader for built-in service specs"
```

---

## Task 4 — Add EtherpadAuth to config

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add EtherpadAuth struct and AuthConfig field**

In `internal/config/config.go`, update `AuthConfig` and add `EtherpadAuth`:

```go
// AuthConfig selects and configures an auth provider.
// Only one sub-field should be set per service.
type AuthConfig struct {
	Discourse *DiscourseAuth `yaml:"discourse,omitempty" mapstructure:"discourse"`
	Etherpad  *EtherpadAuth  `yaml:"etherpad,omitempty"  mapstructure:"etherpad"`
}

// EtherpadAuth holds the API key for the Etherpad REST API.
// The key is injected as the ?apikey= query parameter on every request.
//
// Override via environment variable:
//
//	CORA_SERVICES_<NAME>_AUTH_ETHERPAD_API_KEY
type EtherpadAuth struct {
	APIKey string `yaml:"api_key" mapstructure:"api_key"`
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /Users/husheng/codes/cora
go build ./internal/config/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add EtherpadAuth config struct"
```

---

## Task 5 — Inject Etherpad apikey in auth resolver

**Files:**
- Modify: `internal/auth/resolver.go`
- Modify: `internal/executor/executor.go`

- [ ] **Step 1: Update resolver.go to inject apikey as URL query param**

Replace `internal/auth/resolver.go` entirely with:

```go
package auth

import (
	"net/http"

	"github.com/cncf/cora/internal/config"
)

// InjectAuth adds authentication credentials to an outgoing request based on
// the service's configured auth provider.
//
// Discourse: injects Api-Key and Api-Username headers.
// Etherpad:  injects ?apikey= into the request URL's query string.
//
// Both providers inject credentials unconditionally when present; the server
// ignores them for public endpoints and enforces them for protected ones.
func InjectAuth(req *http.Request, svc config.ServiceConfig) {
	if d := svc.Auth.Discourse; d != nil {
		if d.APIKey != "" {
			req.Header.Set("Api-Key", d.APIKey)
		}
		if d.APIUsername != "" {
			req.Header.Set("Api-Username", d.APIUsername)
		}
	}

	if e := svc.Auth.Etherpad; e != nil && e.APIKey != "" {
		q := req.URL.Query()
		q.Set("apikey", e.APIKey)
		req.URL.RawQuery = q.Encode()
	}
}

// IsDiscourseAuthParam reports whether an OpenAPI parameter is one of the
// Discourse auth headers that should be injected automatically (not exposed
// to the user as a CLI flag).
func IsDiscourseAuthParam(name string) bool {
	return name == "Api-Key" || name == "Api-Username"
}
```

- [ ] **Step 2: Update executor.go to call InjectAuth**

In `internal/executor/executor.go`, update the auth injection call (line ~102):

Change:
```go
	// Inject auth headers (Discourse: Api-Key + Api-Username)
	auth.InjectHeaders(httpReq, svcCfg)
```

To:
```go
	// Inject auth credentials (Discourse: headers; Etherpad: ?apikey= query param)
	auth.InjectAuth(httpReq, svcCfg)
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /Users/husheng/codes/cora
go build ./...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/resolver.go internal/executor/executor.go
git commit -m "feat(auth): inject Etherpad apikey as URL query parameter"
```

---

## Task 6 — Create builtin registry for Etherpad

**Files:**
- Create: `internal/registry/builtin.go`
- Modify: `internal/registry/registry.go`

- [ ] **Step 1: Create builtin.go**

```go
// internal/registry/builtin.go
package registry

import (
	"time"

	"github.com/cncf/cora/assets"
	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/internal/spec"
)

const (
	etherpadName       = "etherpad"
	etherpadDefaultURL = "https://etherpad.openeuler.org/api/1.3.0"
)

// registerBuiltins adds built-in service entries to the registry and ensures
// each built-in is present in cfg.Services (so the executor can look it up).
//
// User config takes priority: if the user has already set a base_url or auth
// block for a built-in service, those values are preserved as-is.
func registerBuiltins(r *Registry, cfg *config.Config) {
	cacheDir := cfg.SpecCache.Dir
	ttl := cfg.SpecCache.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	addBuiltin(r, cfg, builtinDef{
		name:       etherpadName,
		defaultURL: etherpadDefaultURL,
		specData:   assets.EtherpadSpec,
		cacheDir:   cacheDir,
		ttl:        ttl,
	})
}

type builtinDef struct {
	name       string
	defaultURL string
	specData   []byte
	cacheDir   string
	ttl        time.Duration
}

func addBuiltin(r *Registry, cfg *config.Config, b builtinDef) {
	// Determine effective base_url: user config wins, otherwise use default.
	baseURL := b.defaultURL
	if svc, ok := cfg.Services[b.name]; ok && svc.BaseURL != "" {
		baseURL = svc.BaseURL
	}

	r.entries[b.name] = &Entry{
		Name:    b.name,
		BaseURL: baseURL,
		SpecURL: "", // embedded — no remote URL
		loader:  spec.NewEmbeddedLoader(b.name, b.specData, b.cacheDir, b.ttl),
	}

	// Inject a default ServiceConfig so the executor can find this service
	// even when the user has not added it to their config file.
	if _, ok := cfg.Services[b.name]; !ok {
		cfg.Services[b.name] = config.ServiceConfig{BaseURL: b.defaultURL}
	}
}
```

- [ ] **Step 2: Update registry.go — call registerBuiltins and handle config overrides**

In `internal/registry/registry.go`, update the `New()` function.

Replace the existing `New()` body with:

```go
// New builds a Registry from the application config.
func New(cfg *config.Config) *Registry {
	r := &Registry{
		entries: make(map[string]*Entry),
		aliases: make(map[string]string),
	}

	cacheDir := cfg.SpecCache.Dir
	ttl := cfg.SpecCache.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	// Register built-in services first (they may be overridden by user config below).
	registerBuiltins(r, cfg)

	// Apply user-configured services.
	for name, svc := range cfg.Services {
		if existing, ok := r.entries[name]; ok {
			// Built-in service — allow the user to override the spec source.
			if svc.SpecURL != "" {
				existing.SpecURL = svc.SpecURL
				existing.loader = spec.NewLoader(name, svc.SpecURL, cacheDir, ttl)
			}
			continue
		}
		// Non-builtin service — requires a spec_url.
		if svc.SpecURL == "" {
			continue
		}
		entry := &Entry{
			Name:    name,
			BaseURL: svc.BaseURL,
			SpecURL: svc.SpecURL,
			loader:  spec.NewLoader(name, svc.SpecURL, cacheDir, ttl),
		}
		r.entries[name] = entry
	}

	return r
}
```

- [ ] **Step 3: Verify full build**

```bash
cd /Users/husheng/codes/cora
go build ./...
```

Expected: no output.

- [ ] **Step 4: Run all tests**

```bash
cd /Users/husheng/codes/cora
go test ./...
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/registry/builtin.go internal/registry/registry.go
git commit -m "feat(registry): add Etherpad as built-in service with embedded spec"
```

---

## Task 7 — Smoke test end-to-end

No new files. Verify the full integration works.

- [ ] **Step 1: Build the binary**

```bash
cd /Users/husheng/codes/cora
go build -o /tmp/cora ./cmd/cora/
```

Expected: `/tmp/cora` created.

- [ ] **Step 2: Verify etherpad appears in services list**

```bash
/tmp/cora services list
```

Expected: output includes a row for `etherpad` with base URL `https://etherpad.openeuler.org/api/1.3.0`.

- [ ] **Step 3: Verify etherpad help shows resource commands**

```bash
/tmp/cora etherpad --help
```

Expected: list of resource sub-commands derived from the spec (e.g. `group`, `pad-operations`, etc.).

- [ ] **Step 4: Dry-run a command to verify apikey injection**

Create a minimal test config:

```bash
mkdir -p /tmp/cora-test
cat > /tmp/cora-test/config.yaml <<'EOF'
services:
  etherpad:
    auth:
      etherpad:
        api_key: test-key-12345
EOF
```

Run with dry-run:

```bash
CORA_CONFIG=/tmp/cora-test/config.yaml /tmp/cora etherpad group list --dry-run
```

Expected: output shows a URL containing `apikey=test-key-12345`, e.g.:
```
[dry-run] GET https://etherpad.openeuler.org/api/1.3.0/listAllGroups?apikey=test-key-12345
```

- [ ] **Step 5: Verify no apikey leakage when not configured**

```bash
/tmp/cora etherpad group list --dry-run
```

Expected: URL shown does NOT contain `apikey=` (no key configured → no injection).

- [ ] **Step 6: Commit**

```bash
cd /Users/husheng/codes/cora
git add -p   # review any residual changes
git commit -m "feat: add Etherpad service support with config-based API key auth" || echo "nothing to commit"
```

---

## Self-Review

### Spec coverage

| Requirement | Task |
|-------------|------|
| Etherpad spec embedded in binary | Tasks 1, 2, 3 |
| API key stored in config file | Task 4 |
| API key auto-injected as query param | Task 5 |
| Service visible in `cora services list` | Task 6 |
| User can override base_url in config | Task 6 (addBuiltin logic) |
| End-to-end dry-run verification | Task 7 |

### Placeholder scan
No TBD/TODO placeholders. All code steps are complete.

### Type consistency
- `NewEmbeddedLoader` defined in Task 3, used in Task 6 (`builtin.go`) ✓
- `auth.InjectAuth` defined in Task 5, called in executor Task 5 ✓
- `EtherpadAuth` defined in Task 4, referenced in `InjectAuth` Task 5 ✓
- `assets.EtherpadSpec` defined in Task 2, used in Task 6 ✓
- `registerBuiltins` defined in Task 6 (`builtin.go`), called in `registry.go` Task 6 ✓

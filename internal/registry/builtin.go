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

	// Ensure cfg.Services has the effective base_url so the executor can find
	// this service. When the user only sets auth (without base_url), backfill
	// the default.
	if existing, ok := cfg.Services[b.name]; !ok {
		cfg.Services[b.name] = config.ServiceConfig{BaseURL: baseURL}
	} else if existing.BaseURL == "" {
		existing.BaseURL = baseURL
		cfg.Services[b.name] = existing
	}
}

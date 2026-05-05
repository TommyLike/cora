package registry

import (
	"time"

	"github.com/cncf/cora/assets"
	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/internal/spec"
)

const (
	etherpadName = "etherpad"
	gitcodeName  = "gitcode"
	githubName   = "github"
	jenkinsName  = "jenkins"
)

// registerBuiltins adds built-in service entries to the registry and ensures
// each built-in is present in cfg.Services (so the executor can look it up).
//
// The OpenAPI spec for each built-in is embedded in the binary; no remote
// spec_url is required. However, base_url MUST be explicitly set in the user's
// config file — there are no hardcoded default URLs. If a built-in service is
// missing its base_url the executor will return a clear config error at runtime.
func registerBuiltins(r *Registry, cfg *config.Config) {
	cacheDir := cfg.SpecCache.Dir
	ttl := cfg.SpecCache.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	addBuiltin(r, cfg, builtinDef{
		name:     etherpadName,
		specData: assets.EtherpadSpec,
		cacheDir: cacheDir,
		ttl:      ttl,
	})

	addBuiltin(r, cfg, builtinDef{
		name:     gitcodeName,
		specData: assets.GitcodeSpec,
		cacheDir: cacheDir,
		ttl:      ttl,
	})

	addBuiltin(r, cfg, builtinDef{
		name:     githubName,
		specData: assets.GithubSpec,
		cacheDir: cacheDir,
		ttl:      ttl,
	})

	addBuiltin(r, cfg, builtinDef{
		name:     jenkinsName,
		specData: assets.JenkinsSpec,
		cacheDir: cacheDir,
		ttl:      ttl,
	})
}

type builtinDef struct {
	name     string
	specData []byte
	cacheDir string
	ttl      time.Duration
}

func addBuiltin(r *Registry, cfg *config.Config, b builtinDef) {
	// base_url comes entirely from user config; empty string is allowed here —
	// the executor will surface a clear "base_url is not set" error at call time.
	baseURL := ""
	if svc, ok := cfg.Services[b.name]; ok {
		baseURL = svc.BaseURL
	}

	r.entries[b.name] = &Entry{
		Name:    b.name,
		BaseURL: baseURL,
		SpecURL: "", // embedded spec — no remote URL needed
		loader:  spec.NewEmbeddedLoader(b.name, b.specData, b.cacheDir, b.ttl),
	}

	// Ensure cfg.Services contains an entry for this built-in so the executor
	// can look it up. If the user omitted the service entirely from config, add
	// a stub; if they configured it (e.g. auth-only), preserve all their values.
	if _, ok := cfg.Services[b.name]; !ok {
		cfg.Services[b.name] = config.ServiceConfig{}
	}
}

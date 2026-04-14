package registry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/internal/spec"
)

// Entry holds metadata and the spec loader for one backend service.
type Entry struct {
	// Name is the primary CLI name, e.g. "forum".
	Name string
	// Aliases are optional alternative names, e.g. ["discourse"].
	Aliases []string
	// BaseURL is the API root address from the config.
	BaseURL string
	// SpecURL is the OpenAPI spec source from the config.
	SpecURL string
	loader  *spec.Loader
}

// Registry maps service names → Entry.
// Entries are created from the config file at runtime.
type Registry struct {
	entries map[string]*Entry // keyed by canonical Name
	aliases map[string]string // alias → canonical name
}

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

	for name, svc := range cfg.Services {
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

// Register adds a manually constructed entry (used in tests or builtin services).
func (r *Registry) Register(entry *Entry) {
	r.entries[entry.Name] = entry
	for _, alias := range entry.Aliases {
		r.aliases[strings.ToLower(alias)] = entry.Name
	}
}

// Lookup returns the Entry for a service name or alias.
func (r *Registry) Lookup(name string) (*Entry, error) {
	name = strings.ToLower(name)
	if canonical, ok := r.aliases[name]; ok {
		name = canonical
	}
	entry, ok := r.entries[name]
	if !ok {
		return nil, fmt.Errorf("unknown service %q (run 'community config init' to add services)", name)
	}
	return entry, nil
}

// Names returns all registered service names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.entries))
	for n := range r.entries {
		names = append(names, n)
	}
	return names
}

// Entries returns all registered entries.
func (r *Registry) Entries() []*Entry {
	entries := make([]*Entry, 0, len(r.entries))
	for _, e := range r.entries {
		entries = append(entries, e)
	}
	return entries
}

// LoadSpec fetches (or returns cached) OpenAPI spec for the entry.
func (e *Entry) LoadSpec(ctx context.Context) (*openapi3.T, error) {
	return e.loader.Load(ctx)
}

// LoadCached reads the spec from the local cache only — no network call.
// Returns (nil, zero, nil) when the service has not been cached yet.
func (e *Entry) LoadCached() (*openapi3.T, time.Time, error) {
	return e.loader.LoadCached()
}

// InvalidateCache removes the cached spec so it is re-fetched on next use.
func (e *Entry) InvalidateCache() error {
	return e.loader.Invalidate()
}

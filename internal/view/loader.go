package view

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EnvViewsPath is the environment variable that overrides the views.yaml path.
const EnvViewsPath = "CORA_VIEWS"

// rawViews mirrors the top-level views.yaml structure:
//
//	<service> → <resource/verb> → ViewConfig
type rawViews map[string]map[string]ViewConfig

// LoadResult carries the Registry and diagnostic information from LoadRegistry.
type LoadResult struct {
	Registry     *Registry
	ResolvedPath string // absolute path that was attempted
	Loaded       bool   // true if the file was found and parsed successfully
	Err          error  // non-nil if the file existed but could not be parsed
}

// LoadRegistry builds a Registry by layering built-in views (lowest priority)
// with user-defined views from views.yaml (highest priority).
//
// viewsFile is an explicit path to the user's views.yaml; pass "" to trigger
// automatic discovery ($CORA_VIEWS → ~/.config/cora/views.yaml).
//
// A missing or malformed views.yaml is silently ignored — the binary always
// ships with the built-in views and those are never affected.
func LoadRegistry(viewsFile string) LoadResult {
	reg := NewRegistry()

	// 1. Register built-in views (overridable by user entries).
	for svc, ops := range builtinViews {
		for opKey, cfg := range ops {
			reg.Register(svc, opKey, cfg)
		}
	}

	result := LoadResult{Registry: reg}

	// 2. Resolve user views.yaml path.
	path := resolveViewsPath(viewsFile)
	result.ResolvedPath = path
	if path == "" {
		return result
	}

	// 3. Read & parse; ignore missing file.
	data, err := os.ReadFile(path)
	if err != nil {
		// File absent is normal; any other error is worth surfacing.
		if !os.IsNotExist(err) {
			result.Err = err
		}
		return result
	}

	var raw rawViews
	if err := yaml.Unmarshal(data, &raw); err != nil {
		result.Err = err
		return result // malformed file — fall back to built-ins
	}

	// 4. User entries override built-in ones (whole ViewConfig replacement).
	for svc, ops := range raw {
		for opKey, cfg := range ops {
			reg.Register(svc, opKey, cfg)
		}
	}

	result.Loaded = true
	return result
}

func resolveViewsPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if p := os.Getenv(EnvViewsPath); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "cora", "views.yaml")
}

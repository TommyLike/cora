package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultCacheTTL = 24 * time.Hour
	EnvConfigPath   = "CORA_CONFIG"
)

// Config is the root of ~/.config/cora/config.yaml.
type Config struct {
	Services  map[string]ServiceConfig `yaml:"services"`
	SpecCache SpecCacheConfig          `yaml:"spec_cache"`
}

// ServiceConfig holds per-service settings.
type ServiceConfig struct {
	// SpecURL is where the OpenAPI spec lives.
	// Supported schemes: http://, https://, file://, or a bare filesystem path.
	SpecURL string `yaml:"spec_url"`

	// BaseURL is the actual API root (overrides spec's servers[0].url).
	BaseURL string `yaml:"base_url"`

	// Auth holds service-level credential configuration.
	Auth AuthConfig `yaml:"auth"`
}

// AuthConfig selects and configures an auth provider.
// Only one sub-field should be set per service.
type AuthConfig struct {
	Discourse *DiscourseAuth `yaml:"discourse,omitempty"`
}

// DiscourseAuth holds the two header values Discourse requires.
type DiscourseAuth struct {
	APIKey      string `yaml:"api_key"`
	APIUsername string `yaml:"api_username"`
}

// SpecCacheConfig controls global Spec caching behaviour.
type SpecCacheConfig struct {
	// TTL is how long a cached spec is considered fresh. Default: 24h.
	TTL time.Duration `yaml:"ttl"`
	// Dir is the directory for cache files. Default: ~/.config/cora/cache.
	Dir string `yaml:"dir"`
}

// Load reads the config from the default location ($CORA_CONFIG or
// ~/.config/cora/config.yaml).  Missing file returns empty defaults.
func Load() (*Config, error) {
	path := os.Getenv(EnvConfigPath)
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		path = filepath.Join(home, ".config", "cora", "config.yaml")
	}
	return LoadFrom(path)
}

// LoadFrom reads config from an explicit path.
func LoadFrom(path string) (*Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	// Apply defaults for zero values left by YAML
	if cfg.SpecCache.TTL == 0 {
		cfg.SpecCache.TTL = DefaultCacheTTL
	}
	if cfg.SpecCache.Dir == "" {
		cfg.SpecCache.Dir = defaultCacheDir()
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Services: map[string]ServiceConfig{},
		SpecCache: SpecCacheConfig{
			TTL: DefaultCacheTTL,
			Dir: defaultCacheDir(),
		},
	}
}

func defaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cora-cache"
	}
	return filepath.Join(home, ".config", "cora", "cache")
}

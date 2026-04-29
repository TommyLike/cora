package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"github.com/cncf/cora/internal/log"
)

const (
	DefaultCacheTTL = 24 * time.Hour

	// EnvConfigPath is the environment variable that overrides the config file path.
	EnvConfigPath = "CORA_CONFIG"

	// EnvPrefix is the prefix for all CORA_ environment variables.
	EnvPrefix = "CORA"
)

// Config is the root of ~/.config/cora/config.yaml.
type Config struct {
	Services  map[string]ServiceConfig `yaml:"services"   mapstructure:"services"`
	SpecCache SpecCacheConfig          `yaml:"spec_cache" mapstructure:"spec_cache"`
	// ViewsFile is an explicit path to views.yaml.
	// Overridden by the CORA_VIEWS environment variable.
	// Defaults to ~/.config/cora/views.yaml when empty.
	ViewsFile string `yaml:"views_file" mapstructure:"views_file"`
}

// ServiceConfig holds per-service settings.
type ServiceConfig struct {
	// SpecURL is where the OpenAPI spec lives.
	// Supported schemes: http://, https://, file://, or a bare filesystem path.
	SpecURL string `yaml:"spec_url" mapstructure:"spec_url"`

	// BaseURL is the actual API root (overrides spec's servers[0].url).
	BaseURL string `yaml:"base_url" mapstructure:"base_url"`

	// Auth holds service-level credential configuration.
	Auth AuthConfig `yaml:"auth" mapstructure:"auth"`
}

// AuthConfig selects and configures an auth provider.
// Only one sub-field should be set per service.
type AuthConfig struct {
	Discourse *DiscourseAuth `yaml:"discourse,omitempty" mapstructure:"discourse"`
	Etherpad  *EtherpadAuth  `yaml:"etherpad,omitempty"  mapstructure:"etherpad"`
	Gitcode   *GitcodeAuth   `yaml:"gitcode,omitempty"   mapstructure:"gitcode"`
	Github    *GithubAuth    `yaml:"github,omitempty"    mapstructure:"github"`
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

// GitcodeAuth holds the personal access token for the GitCode REST API.
// The token is injected as the ?access_token= query parameter on every request.
//
// Override via environment variable:
//
//	CORA_SERVICES_<NAME>_AUTH_GITCODE_ACCESS_TOKEN
type GitcodeAuth struct {
	AccessToken string `yaml:"access_token" mapstructure:"access_token"`
}

// GithubAuth holds the personal access token (PAT) or fine-grained token for the
// GitHub REST API. The token is sent as a Bearer credential in the Authorization
// header on every request.
//
// Override via environment variable:
//
//	CORA_SERVICES_<NAME>_AUTH_GITHUB_TOKEN
type GithubAuth struct {
	Token string `yaml:"token" mapstructure:"token"`
}

// DiscourseAuth holds the two header values Discourse requires.
// Both fields can be overridden via environment variables:
//
//	CORA_SERVICES_<NAME>_AUTH_DISCOURSE_API_KEY
//	CORA_SERVICES_<NAME>_AUTH_DISCOURSE_API_USERNAME
type DiscourseAuth struct {
	APIKey      string `yaml:"api_key"      mapstructure:"api_key"`
	APIUsername string `yaml:"api_username" mapstructure:"api_username"`
}

// SpecCacheConfig controls global Spec caching behaviour.
//
// Environment variable overrides:
//
//	CORA_SPEC_CACHE_TTL  (e.g. "12h")
//	CORA_SPEC_CACHE_DIR  (e.g. "/tmp/cora-cache")
type SpecCacheConfig struct {
	// TTL is how long a cached spec is considered fresh. Default: 24h.
	TTL time.Duration `yaml:"ttl" mapstructure:"ttl"`
	// Dir is the directory for cache files. Default: ~/.config/cora/cache.
	Dir string `yaml:"dir" mapstructure:"dir"`
}

// Load reads the config with the following precedence (highest → lowest):
//
//  1. Environment variables with CORA_ prefix
//  2. Config file ($CORA_CONFIG or ~/.config/cora/config.yaml)
//  3. Built-in defaults
//
// Before reading the config file, Load calls godotenv.Load() to populate
// environment variables from a .env file in the current working directory.
// The .env file is optional — its absence is silently ignored, making this
// behaviour safe for production deployments that use real environment variables.
func Load() (*Config, error) {
	// 1. Populate os env from .env (local dev convenience).
	loadDotEnv()

	// 2. Determine config file path.
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

// loadDotEnv searches for a .env file and applies its variables to the process
// environment. Variables that already have a non-empty value in the environment
// are left unchanged — real env vars always win over .env.
//
// Search order (first found wins):
//  1. ./.env           — CWD; useful when running `./cora` from project root
//  2. ~/.config/cora/.env — user-level overrides; works regardless of CWD
//
// Why not godotenv.Load() directly? godotenv skips any key that exists in
// os.Environ(), even if its value is empty (e.g. `export CORA_CONFIG=""`
// in a shell profile). Our rule is stricter: only a non-empty existing value
// wins, so an empty-set var is treated as "not configured" and .env can supply it.
func loadDotEnv() {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()

	candidates := []string{
		filepath.Join(cwd, ".env"),
		filepath.Join(home, ".config", "cora", ".env"),
	}

	for _, f := range candidates {
		envMap, err := godotenv.Read(f)
		if err != nil {
			// File absent or unreadable — try next candidate.
			continue
		}
		// Apply values only for vars that are not already meaningfully set.
		count := 0
		for k, v := range envMap {
			if os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
				count++
			}
		}
		log.Debug(".env loaded from %s (%d vars applied)", f, count)
		return // stop at the first file found
	}
}

// LoadFrom reads config from an explicit path, applying CORA_* env var overrides.
//
// Environment variables override matching config-file values. The mapping
// from viper key to env var name is: replace "." with "_", uppercase, prepend
// "CORA_". Examples:
//
//	spec_cache.ttl                           → CORA_SPEC_CACHE_TTL
//	spec_cache.dir                           → CORA_SPEC_CACHE_DIR
//	services.forum.base_url                  → CORA_SERVICES_FORUM_BASE_URL
//	services.forum.spec_url                  → CORA_SERVICES_FORUM_SPEC_URL
//	services.forum.auth.discourse.api_key    → CORA_SERVICES_FORUM_AUTH_DISCOURSE_API_KEY
//	services.forum.auth.discourse.api_username → CORA_SERVICES_FORUM_AUTH_DISCOURSE_API_USERNAME
//
// Note: env vars can only override keys that already exist in the config file
// or in built-in defaults. They cannot introduce brand-new services.
func LoadFrom(path string) (*Config, error) {
	v := viper.New()

	// Config file (YAML).
	v.SetConfigFile(path)

	// Env var overlay: CORA_SPEC_CACHE_TTL overrides spec_cache.ttl, etc.
	v.SetEnvPrefix(EnvPrefix)
	// Replace "." in viper key with "_" to form the env var name.
	// "spec_cache.ttl" → "CORA_SPEC_CACHE_TTL" (underscores within segment names are preserved).
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Built-in defaults — used when absent from both file and env.
	v.SetDefault("spec_cache.ttl", DefaultCacheTTL.String())
	v.SetDefault("spec_cache.dir", defaultCacheDir())

	// Read config file; a missing file is not an error.
	if err := v.ReadInConfig(); err != nil && !isFileNotFound(path, err) {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// Decode into Config, using mapstructure tags and string→duration hook.
	var cfg Config
	if err := v.Unmarshal(&cfg, func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "mapstructure"
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		)
	}); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	log.Info("config loaded from %s", path)

	// Guard zero values (viper defaults should cover these, but be defensive).
	if cfg.SpecCache.TTL == 0 {
		cfg.SpecCache.TTL = DefaultCacheTTL
	}
	if cfg.SpecCache.Dir == "" {
		cfg.SpecCache.Dir = defaultCacheDir()
	}
	if cfg.Services == nil {
		cfg.Services = map[string]ServiceConfig{}
	}

	return &cfg, nil
}

// isFileNotFound returns true when err represents "config file does not exist".
func isFileNotFound(path string, err error) bool {
	if os.IsNotExist(err) {
		return true
	}
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return true
	}
	// Viper wraps os.ErrNotExist in some paths — check the underlying error.
	if strings.Contains(err.Error(), "no such file") ||
		strings.Contains(err.Error(), "cannot find the file") {
		return true
	}
	return false
}

func defaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cora-cache"
	}
	return filepath.Join(home, ".config", "cora", "cache")
}

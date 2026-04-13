package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cacheEntry is the on-disk format for a cached spec.
type cacheEntry struct {
	FetchedAt time.Time       `json:"fetched_at"`
	SpecURL   string          `json:"spec_url"`
	RawSpec   json.RawMessage `json:"raw_spec"`
}

// readCache reads the cache file for a service and returns the entry.
// Returns an error if the file does not exist or cannot be parsed.
func readCache(cacheFile string) (*cacheEntry, error) {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("corrupt cache file %s: %w", cacheFile, err)
	}
	return &entry, nil
}

// writeCache atomically writes raw spec bytes to the cache file.
func writeCache(cacheFile string, specURL string, rawSpec []byte) error {
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	entry := cacheEntry{
		FetchedAt: time.Now(),
		SpecURL:   specURL,
		RawSpec:   json.RawMessage(rawSpec),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Atomic write: write to tmp then rename
	tmp := cacheFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write cache tmp: %w", err)
	}
	return os.Rename(tmp, cacheFile)
}

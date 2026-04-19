package smoke

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// namePattern enforces the scenario naming convention:
// lowercase letters, digits, hyphens, dots and forward slashes only.
// Format: <service>/<resource>-<verb>, e.g. "gitcode/issues-list".
// Spaces and special characters are rejected so names are safe to use as
// CLI filter arguments without quoting.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9\-./]*$`)

// LoadScenarios walks dir recursively and parses all *.yaml files as Scenarios.
// Returns an error if any file fails validation.
func LoadScenarios(dir string) ([]Scenario, error) {
	var scenarios []Scenario
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}
		s, skip, err := loadFile(path)
		if err != nil {
			return fmt.Errorf("load %s: %w", path, err)
		}
		if skip {
			return nil
		}
		scenarios = append(scenarios, s)
		return nil
	})
	return scenarios, err
}

// loadFile parses one YAML file. The second return value is true when the file
// is empty or contains only comments and should be silently skipped.
func loadFile(path string) (Scenario, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, false, err
	}
	var s Scenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Scenario{}, false, fmt.Errorf("parse YAML: %w", err)
	}
	// Empty file or pure-comment file: yaml.Unmarshal produces a zero Scenario.
	// Silently skip rather than error.
	if s.Name == "" && s.Service == "" {
		return Scenario{}, true, nil
	}
	// Apply defaults.
	if s.Format == "" {
		s.Format = "table"
	}
	if s.TimeoutMs <= 0 {
		s.TimeoutMs = 10000
	}
	s.FilePath = path
	// Validate required fields.
	if s.Name == "" {
		return Scenario{}, false, fmt.Errorf("missing required field: name")
	}
	if s.Service == "" {
		return Scenario{}, false, fmt.Errorf("missing required field: service")
	}
	// Enforce naming convention: lowercase, no spaces, safe for CLI filter args.
	if !namePattern.MatchString(s.Name) {
		return Scenario{}, false, fmt.Errorf(
			"invalid scenario name %q: must match %s (e.g. \"gitcode/issues-list\")",
			s.Name, namePattern,
		)
	}
	return s, false, nil
}

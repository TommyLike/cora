package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cncf/cora/internal/smoke"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR]", err)
		os.Exit(1)
	}
}

func run() error {
	coraBin := flag.String("cora-bin", "./bin/cora", "path to cora binary")
	configPath := flag.String("config", "./config/smoke-config.yaml", "cora config file for smoke tests (supports ${VAR} expansion)")
	scenariosDir := flag.String("scenarios-dir", "./scenarios", "directory containing scenario YAML files")
	reportDir := flag.String("report-dir", "./smoke-report", "directory to write report.html into")
	filter := flag.String("filter", "", "only run scenarios whose name or service contains this string")
	verbose := flag.Bool("verbose", false, "print stdout/stderr for every scenario, not just failures")
	flag.Parse()

	// Verify cora binary exists.
	if _, err := os.Stat(*coraBin); err != nil {
		return fmt.Errorf("cora binary not found at %q: run 'make build-prod' first", *coraBin)
	}

	// Expand ${VAR} references in config and write to a temp file.
	// The temp file path is passed via CORA_CONFIG env var to each cora invocation.
	expandedConfig, err := expandConfigEnvVars(*configPath)
	if err != nil {
		return fmt.Errorf("expand config env vars: %w", err)
	}
	if expandedConfig != "" {
		defer os.Remove(expandedConfig)
	}

	// Load scenarios.
	scenarios, err := smoke.LoadScenarios(*scenariosDir)
	if err != nil {
		return fmt.Errorf("load scenarios from %q: %w", *scenariosDir, err)
	}
	if len(scenarios) == 0 {
		fmt.Println("No scenarios found in", *scenariosDir)
		return nil
	}

	// Apply name/service filter.
	if *filter != "" {
		var filtered []smoke.Scenario
		for _, s := range scenarios {
			if containsCI(s.Name, *filter) || containsCI(s.Service, *filter) {
				filtered = append(filtered, s)
			}
		}
		scenarios = filtered
		fmt.Printf("Filter %q matched %d scenario(s)\n\n", *filter, len(scenarios))
	}

	fmt.Printf("Running %d scenario(s)...\n\n", len(scenarios))

	// Execute all scenarios.
	runner := smoke.NewRunner(*coraBin, expandedConfig)
	report := runner.RunAll(scenarios, *configPath)

	// Print live results.
	for _, result := range report.Results {
		smoke.PrintConsole(result)
		if *verbose && result.Status != smoke.StatusSkip {
			if result.Stdout != "" {
				fmt.Printf("  stdout:\n%s\n", indent(result.Stdout, "    "))
			}
			if result.Stderr != "" {
				fmt.Printf("  stderr:\n%s\n", indent(result.Stderr, "    "))
			}
		}
	}

	// Write HTML report into a dated subdirectory for archiving.
	// e.g. ./smoke-report/2026-04-19/report.html
	dateDir := filepath.Join(*reportDir, time.Now().UTC().Format("2006-01-02"))
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return fmt.Errorf("create report dir: %w", err)
	}
	html, err := smoke.GenerateHTML(report)
	if err != nil {
		return fmt.Errorf("generate HTML: %w", err)
	}
	reportPath := filepath.Join(dateDir, "report.html")
	if err := os.WriteFile(reportPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	smoke.PrintSummary(report, reportPath)

	if report.Failed() > 0 {
		return fmt.Errorf("%d scenario(s) failed", report.Failed())
	}
	return nil
}

// expandConfigEnvVars reads configPath, expands ${VAR} via os.ExpandEnv,
// writes the result to a temp file, and returns the temp file path.
// Returns "" if configPath does not exist.
func expandConfigEnvVars(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	expanded := os.ExpandEnv(string(data))
	tmp, err := os.CreateTemp("", "smoke-config-*.yaml")
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	if _, err := tmp.WriteString(expanded); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

// containsCI reports whether s contains sub, case-insensitively.
func containsCI(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// indent prefixes each line of s with prefix.
func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

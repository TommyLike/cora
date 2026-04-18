package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// buildEnvCmd returns `cora env` — a diagnostic command that prints everything
// cora sees at startup: CWD, .env search results, and all resolved CORA_* vars.
func buildEnvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "env",
		Short: "Show resolved environment: .env search path and all CORA_* variables",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, _ := os.Getwd()
			home, _ := os.UserHomeDir()

			fmt.Println("=== Runtime diagnostics ===")
			fmt.Printf("CWD:  %s\n", cwd)
			fmt.Printf("HOME: %s\n", home)
			fmt.Println()

			// ── .env search ───────────────────────────────────────────────
			candidates := []string{
				filepath.Join(cwd, ".env"),
				filepath.Join(home, ".config", "cora", ".env"),
			}

			fmt.Println("=== .env file search ===")
			foundDotenv := ""
			for _, f := range candidates {
				if _, err := os.Stat(f); err == nil {
					fmt.Printf("  FOUND:   %s\n", f)
					foundDotenv = f
					break
				} else {
					fmt.Printf("  missing: %s\n", f)
				}
			}
			fmt.Println()

			// ── .env contents ─────────────────────────────────────────────
			if foundDotenv != "" {
				fmt.Printf("=== .env contents (%s) ===\n", foundDotenv)
				envMap, err := godotenv.Read(foundDotenv)
				switch {
				case err != nil:
					fmt.Printf("  error reading: %v\n", err)
				case len(envMap) == 0:
					fmt.Println("  (empty)")
				default:
					keys := make([]string, 0, len(envMap))
					for k := range envMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						v := envMap[k]
						if strings.Contains(strings.ToLower(k), "key") ||
							strings.Contains(strings.ToLower(k), "secret") ||
							strings.Contains(strings.ToLower(k), "password") {
							v = "***"
						}
						fmt.Printf("  %s=%s\n", k, v)
					}
				}
				fmt.Println()
			}

			// ── Resolved CORA_* env vars ──────────────────────────────────
			fmt.Println("=== Resolved CORA_* environment variables ===")
			var coraVars []string
			for _, kv := range os.Environ() {
				if strings.HasPrefix(kv, "CORA_") {
					coraVars = append(coraVars, kv)
				}
			}
			sort.Strings(coraVars)
			if len(coraVars) == 0 {
				fmt.Println("  (none)")
			} else {
				for _, kv := range coraVars {
					parts := strings.SplitN(kv, "=", 2)
					k, v := parts[0], parts[1]
					if strings.Contains(strings.ToLower(k), "key") ||
						strings.Contains(strings.ToLower(k), "secret") ||
						strings.Contains(strings.ToLower(k), "password") {
						v = "***"
					}
					fmt.Printf("  %s=%s\n", k, v)
				}
			}
			fmt.Println()

			// ── Config file resolution ─────────────────────────────────────
			fmt.Println("=== Config file resolution ===")
			coraConfig := os.Getenv("CORA_CONFIG")
			if coraConfig != "" {
				fmt.Printf("  source:  CORA_CONFIG env var\n")
				fmt.Printf("  path:    %s\n", coraConfig)
			} else {
				defaultPath := filepath.Join(home, ".config", "cora", "config.yaml")
				fmt.Printf("  source:  default path (CORA_CONFIG not set)\n")
				fmt.Printf("  path:    %s\n", defaultPath)
				coraConfig = defaultPath
			}
			if _, err := os.Stat(coraConfig); err == nil {
				fmt.Printf("  status:  exists\n")
			} else {
				fmt.Printf("  status:  NOT FOUND (%v)\n", err)
			}

			return nil
		},
	}
}

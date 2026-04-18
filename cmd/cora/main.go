package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cncf/cora/internal/builder"
	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/internal/executor"
	"github.com/cncf/cora/internal/log"
	"github.com/cncf/cora/internal/registry"
	"github.com/cncf/cora/internal/view"
	"github.com/cncf/cora/pkg/errs"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR]", err)
		if e, ok := err.(*errs.CLIError); ok && e.Hint != "" {
			fmt.Fprintln(os.Stderr, "→", e.Hint)
		}
		os.Exit(int(errs.GetExitCode(err)))
	}
}

func run() error {
	// ── Init logging (pre-scan os.Args before cobra parses) ───────────────────
	// We must scan os.Args directly because config loading and spec loading
	// happen before cobra's Execute() and need the log level set upfront.
	verbose := containsFlag(os.Args, "--verbose")
	log.Init(verbose)

	// ── Load config ───────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		return errs.NewConfigError(fmt.Sprintf("load config: %v", err))
	}

	// ── Core objects ──────────────────────────────────────────────────────────
	reg := registry.New(cfg)
	exec := executor.New(cfg)
	viewResult := view.LoadRegistry(cfg.ViewsFile)
	viewReg := viewResult.Registry
	if viewResult.ResolvedPath != "" {
		switch {
		case viewResult.Loaded:
			log.Info("views loaded from %s", viewResult.ResolvedPath)
		case viewResult.Err != nil:
			log.Warn("views file %s could not be loaded: %v", viewResult.ResolvedPath, viewResult.Err)
		default:
			log.Debug("views file not found at %s, using built-in views only", viewResult.ResolvedPath)
		}
	}

	// ── Root command ──────────────────────────────────────────────────────────
	root := &cobra.Command{
		Use:   "cora",
		Short: "cora — unified access to open-source community services",
		Long: `cora (Community Collaboration) aggregates access to community services
(forums, mailing lists, meeting calendars, issue trackers, …)
driven by OpenAPI specs published by each backend service.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global persistent flags available to every sub-command.
	root.PersistentFlags().String("format", "table", "output format: table|json|yaml")
	root.PersistentFlags().Bool("dry-run", false, "print the HTTP request without sending it")
	root.PersistentFlags().Bool("refresh-spec", false, "bypass cache and re-fetch the service spec")
	root.PersistentFlags().Bool("verbose", false, "enable verbose output for debugging (INFO + DEBUG logs)")

	// ── Two-phase command loading (mirrors Google CLI) ────────────────────────
	//
	// Phase 1: peek at os.Args to determine which service the user is targeting.
	// Phase 2: load that service's OpenAPI spec (cache-first, 24 h TTL) and
	//          inject the derived Cobra sub-tree BEFORE root.Execute() parses.
	//
	// This means --help, shell completion, and --dry-run all see the full tree.
	if err := injectServiceCommands(root, reg, cfg, exec, viewReg); err != nil {
		// Non-fatal: print a warning so "cora --help" still works.
		log.Warn("%v", err)
	}

	// ── Built-in commands ─────────────────────────────────────────────────────
	root.AddCommand(buildSpecCmd(reg))
	root.AddCommand(buildServicesCmd(reg))
	root.AddCommand(buildEnvCmd())

	return root.Execute()
}

// injectServiceCommands peeks at os.Args[1] to identify the service name,
// then loads its OpenAPI spec and adds the derived command tree to root.
//
// If --refresh-spec is present in os.Args the local cache is invalidated first.
func injectServiceCommands(
	root *cobra.Command,
	reg *registry.Registry,
	cfg *config.Config,
	exec *executor.Executor,
	viewReg *view.Registry,
) error {
	// Find the first non-flag argument – that is the service name.
	svcName := firstNonFlag(os.Args[1:])
	if svcName == "" {
		// No service specified (e.g. "cora --help"); register all services
		// as stub commands so the root help looks complete.
		for _, name := range reg.Names() {
			name := name
			root.AddCommand(&cobra.Command{
				Use:   name,
				Short: fmt.Sprintf("[%s] Commands for the %q service", "spec loaded on demand", name),
			})
		}
		return nil
	}

	entry, err := reg.Lookup(svcName)
	if err != nil {
		// Print a specific hint before cobra's generic "unknown command" fires.
		log.Warn("service %q not found in config — check the service name or add it to your config file", svcName)
		return nil
	}

	// --refresh-spec: clear the cache before loading.
	for _, arg := range os.Args {
		if arg == "--refresh-spec" {
			_ = entry.InvalidateCache()
			break
		}
	}

	log.Debug("looking up service %q", svcName)
	log.Info("loading spec for %q", svcName)
	spec, err := entry.LoadSpec(root.Context())
	if err != nil {
		return fmt.Errorf("load spec for %q: %w", svcName, err)
	}

	svcCmd := builder.Build(svcName, spec, cfg, exec, viewReg)
	root.AddCommand(svcCmd)
	return nil
}

// containsFlag reports whether flag (e.g. "--verbose") appears in args.
// Used to pre-scan os.Args before cobra has parsed the command line.
func containsFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// firstNonFlag returns the first element of args that is not a flag (i.e. does
// not start with "-") and is not a known built-in command name.
func firstNonFlag(args []string) string {
	builtins := map[string]bool{"spec": true, "services": true, "env": true, "auth": true, "config": true, "help": true}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if builtins[a] {
			return ""
		}
		return a
	}
	return ""
}

// buildSpecCmd returns `cora spec <subcommand>` for cache management.
func buildSpecCmd(reg *registry.Registry) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Manage OpenAPI spec cache",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "refresh <service>",
		Short: "Re-fetch and cache the OpenAPI spec for a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := reg.Lookup(args[0])
			if err != nil {
				return err
			}
			if err = entry.InvalidateCache(); err != nil {
				return fmt.Errorf("invalidate cache: %w", err)
			}
			if _, err = entry.LoadSpec(cmd.Context()); err != nil {
				return err
			}
			fmt.Printf("spec for %q refreshed successfully\n", args[0])
			return nil
		},
	})

	return cmd
}

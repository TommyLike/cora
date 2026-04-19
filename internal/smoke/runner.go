package smoke

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cncf/cora/internal/view"
)

// Runner executes scenarios by invoking the cora binary as a subprocess.
type Runner struct {
	coraBin    string         // path to cora binary
	configPath string         // expanded config file path (may be empty)
	verbose    bool           // pass --verbose to every cora invocation
	viewReg    *view.Registry // may be nil when --views not provided
}

// NewRunner creates a Runner. configPath may be "" to skip CORA_CONFIG injection.
// viewsFile is the path to views.yaml; pass "" to skip view-column validation.
func NewRunner(coraBin, configPath string, verbose bool, viewsFile string) *Runner {
	var reg *view.Registry
	if viewsFile != "" {
		reg = view.LoadRegistry(viewsFile).Registry
	}
	return &Runner{coraBin: coraBin, configPath: configPath, verbose: verbose, viewReg: reg}
}

// resourceVerb extracts the first two positional (non-flag) args from a scenario's
// arg list. These correspond to the CLI resource and verb, e.g. ["issues", "list"].
func resourceVerb(args []string) (resource, verb string) {
	pos := 0
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		switch pos {
		case 0:
			resource = a
		case 1:
			verb = a
		}
		pos++
		if pos == 2 {
			break
		}
	}
	return
}

// Run executes a single Scenario and returns its result.
func (r *Runner) Run(s Scenario) ScenarioResult {
	result := ScenarioResult{Scenario: s}

	if s.Skip {
		result.Status = StatusSkip
		return result
	}

	// Expand env vars in args (allows ${SMOKE_GITCODE_OWNER} in scenario files).
	expandedArgs := make([]string, len(s.Args))
	for i, arg := range s.Args {
		expandedArgs[i] = os.ExpandEnv(arg)
	}

	// Build command: <service> <args...> --format <format> [--verbose]
	args := append([]string{s.Service}, expandedArgs...)
	args = append(args, "--format", s.Format)
	if r.verbose {
		args = append(args, "--verbose")
	}

	if r.verbose {
		fmt.Printf("  [cmd] %s %s\n", r.coraBin, strings.Join(args, " "))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.TimeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.coraBin, args...)

	// Inject expanded smoke config via CORA_CONFIG env var.
	cmd.Env = os.Environ()
	if r.configPath != "" {
		cmd.Env = append(cmd.Env, "CORA_CONFIG="+r.configPath)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	result.DurationMs = time.Since(start).Milliseconds()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if ctx.Err() == context.DeadlineExceeded {
		result.Status = StatusTimeout
		result.Err = fmt.Sprintf("timed out after %dms", s.TimeoutMs)
		return result
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Status = StatusError
			result.Err = fmt.Sprintf("subprocess error: %v", err)
			return result
		}
	}

	// Evaluate assertions.
	resource, verb := resourceVerb(s.Args)
	evalCtx := EvalContext{
		ViewRegistry: r.viewReg,
		Service:      s.Service,
		Resource:     resource,
		Verb:         verb,
		Format:       s.Format,
	}
	allPass := true
	for _, a := range s.Assertions {
		ar := EvaluateAssertion(a, evalCtx, result.Stdout, result.Stderr, result.ExitCode, result.DurationMs)
		result.AssertionResults = append(result.AssertionResults, ar)
		if !ar.Passed {
			allPass = false
		}
	}

	if allPass {
		result.Status = StatusPass
	} else {
		result.Status = StatusFail
	}
	return result
}

// RunAll executes all scenarios sequentially and returns a RunReport.
func (r *Runner) RunAll(scenarios []Scenario, configPath string) *RunReport {
	report := &RunReport{
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		ConfigPath:  configPath,
	}
	start := time.Now()
	for _, s := range scenarios {
		report.Results = append(report.Results, r.Run(s))
	}
	report.TotalDurationMs = time.Since(start).Milliseconds()
	return report
}

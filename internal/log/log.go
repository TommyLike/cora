// Package log provides a lightweight, levelled logger for CLI tools.
//
// Design constraints (see spec/logging-design.md):
//   - All output goes to stderr (stdout is reserved for user data).
//   - No timestamps — shell history provides that context.
//   - Two visibility tiers: always-on (ERROR/WARN) and verbose-only (INFO/DEBUG).
//   - Call Init(verbose) once at program start, before any other operation.
package log

import (
	"fmt"
	"os"
	"sync"
)

// level represents the minimum severity that will be emitted.
type level int

const (
	levelWarn  level = iota // default: only WARN and ERROR
	levelDebug              // verbose: all levels
)

var (
	mu      sync.Mutex
	current level = levelWarn
)

// Init sets the global log level.
// verbose=true  → INFO and DEBUG are also printed.
// verbose=false → only WARN and ERROR are printed (default).
//
// Call this once at program start, before any other operation that may log.
func Init(verbose bool) {
	mu.Lock()
	defer mu.Unlock()
	if verbose {
		current = levelDebug
	} else {
		current = levelWarn
	}
}

// Error prints an always-visible error message to stderr.
// Use for unrecoverable failures that terminate or abort the current operation.
func Error(format string, args ...any) {
	write("[ERROR]", format, args...)
}

// Warn prints an always-visible warning to stderr.
// Use for degraded behaviour the program can recover from (stale cache, missing service, etc.).
func Warn(format string, args ...any) {
	write("[WARN] ", format, args...)
}

// Info prints an informational message when verbose mode is enabled.
// Use for key milestones in the normal execution path (spec loaded, config read, etc.).
func Info(format string, args ...any) {
	mu.Lock()
	lvl := current
	mu.Unlock()
	if lvl >= levelDebug {
		write("[INFO] ", format, args...)
	}
}

// Debug prints a diagnostic message when verbose mode is enabled.
// Use for fine-grained tracing (HTTP requests/responses, auth injection, etc.).
func Debug(format string, args ...any) {
	mu.Lock()
	lvl := current
	mu.Unlock()
	if lvl >= levelDebug {
		write("[DEBUG]", format, args...)
	}
}

func write(prefix, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
}

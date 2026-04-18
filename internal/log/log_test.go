package log

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStderr redirects os.Stderr for the duration of fn and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = orig
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stderr: %v", err)
	}
	return buf.String()
}

// resetLevel restores the default log level after a test.
func resetLevel(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { Init(false) })
}

// ── Init ─────────────────────────────────────────────────────────────────────

func TestInit_VerboseEnablesDebug(t *testing.T) {
	resetLevel(t)
	Init(true)

	out := captureStderr(t, func() { Debug("hello debug") })
	if !strings.Contains(out, "hello debug") {
		t.Errorf("verbose mode: Debug output expected, got %q", out)
	}
}

func TestInit_NonVerboseSupressesDebug(t *testing.T) {
	resetLevel(t)
	Init(false)

	out := captureStderr(t, func() { Debug("should not appear") })
	if out != "" {
		t.Errorf("non-verbose mode: Debug should be suppressed, got %q", out)
	}
}

func TestInit_NonVerboseSupressesInfo(t *testing.T) {
	resetLevel(t)
	Init(false)

	out := captureStderr(t, func() { Info("should not appear") })
	if out != "" {
		t.Errorf("non-verbose mode: Info should be suppressed, got %q", out)
	}
}

// ── Error ─────────────────────────────────────────────────────────────────────

func TestError_AlwaysVisible(t *testing.T) {
	resetLevel(t)
	Init(false) // non-verbose

	out := captureStderr(t, func() { Error("something broke: %d", 42) })
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("Error: expected [ERROR] prefix, got %q", out)
	}
	if !strings.Contains(out, "something broke: 42") {
		t.Errorf("Error: expected message in output, got %q", out)
	}
}

func TestError_AlwaysVisibleInVerboseMode(t *testing.T) {
	resetLevel(t)
	Init(true)

	out := captureStderr(t, func() { Error("oops") })
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("Error in verbose mode: expected [ERROR], got %q", out)
	}
}

// ── Warn ─────────────────────────────────────────────────────────────────────

func TestWarn_AlwaysVisible(t *testing.T) {
	resetLevel(t)
	Init(false)

	out := captureStderr(t, func() { Warn("stale cache %s", "spec.json") })
	if !strings.Contains(out, "[WARN]") {
		t.Errorf("Warn: expected [WARN] prefix, got %q", out)
	}
	if !strings.Contains(out, "stale cache spec.json") {
		t.Errorf("Warn: expected message, got %q", out)
	}
}

// ── Info ─────────────────────────────────────────────────────────────────────

func TestInfo_OnlyInVerboseMode(t *testing.T) {
	resetLevel(t)

	Init(false)
	out := captureStderr(t, func() { Info("config loaded") })
	if out != "" {
		t.Errorf("Info in quiet mode should be suppressed, got %q", out)
	}

	Init(true)
	out = captureStderr(t, func() { Info("config loaded from %s", "/etc/cora") })
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("Info in verbose mode: expected [INFO], got %q", out)
	}
	if !strings.Contains(out, "/etc/cora") {
		t.Errorf("Info: expected path in message, got %q", out)
	}
}

// ── Debug ─────────────────────────────────────────────────────────────────────

func TestDebug_OnlyInVerboseMode(t *testing.T) {
	resetLevel(t)

	Init(false)
	out := captureStderr(t, func() { Debug("request: GET /api") })
	if out != "" {
		t.Errorf("Debug in quiet mode should be suppressed, got %q", out)
	}

	Init(true)
	out = captureStderr(t, func() { Debug("→ GET %s", "/api/v1/issues") })
	if !strings.Contains(out, "[DEBUG]") {
		t.Errorf("Debug in verbose mode: expected [DEBUG], got %q", out)
	}
	if !strings.Contains(out, "/api/v1/issues") {
		t.Errorf("Debug: expected URL in message, got %q", out)
	}
}

// ── Output format ─────────────────────────────────────────────────────────────

func TestOutput_EndsWithNewline(t *testing.T) {
	resetLevel(t)
	Init(false)

	out := captureStderr(t, func() { Error("msg") })
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("log output should end with newline, got %q", out)
	}
}

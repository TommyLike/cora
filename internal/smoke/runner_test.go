package smoke

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func buildFakeBinary(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	bin := filepath.Join(dir, "fake"+ext)
	cmd := exec.Command("go", "build", "-o", bin, srcFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake binary: %v\n%s", err, out)
	}
	return bin
}

const fakeBinSuccess = `package main
import "fmt"
func main() { fmt.Println("| ID | TITLE |") }`

const fakeBinExitOne = `package main
import (
	"fmt"
	"os"
)
func main() {
	fmt.Fprintln(os.Stderr, "[ERROR] something failed")
	os.Exit(1)
}`

func TestRunner_SkipScenario(t *testing.T) {
	s := Scenario{Name: "skip me", Skip: true, SkipReason: "not ready"}
	r := NewRunner("", "", false, "")
	result := r.Run(s)
	if result.Status != StatusSkip {
		t.Errorf("expected SKIP, got %s", result.Status)
	}
}

func TestRunner_SuccessCapture(t *testing.T) {
	bin := buildFakeBinary(t, fakeBinSuccess)
	s := Scenario{
		Name:      "success",
		Service:   "svc",
		Args:      []string{"issues", "list"},
		Format:    "table",
		TimeoutMs: 5000,
		Assertions: []Assertion{
			{Type: "exit_code", Value: 0},
			{Type: "stdout_contains", Str: "TITLE"},
		},
	}
	r := NewRunner(bin, "", false, "")
	result := r.Run(s)
	if result.Status != StatusPass {
		t.Errorf("expected PASS, got %s: %s", result.Status, result.Err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}
	if result.DurationMs <= 0 {
		t.Errorf("duration should be > 0, got %d", result.DurationMs)
	}
}

func TestRunner_NonZeroExit_AssertionFails(t *testing.T) {
	bin := buildFakeBinary(t, fakeBinExitOne)
	s := Scenario{
		Name:      "exit1",
		Service:   "svc",
		Args:      []string{"bad"},
		Format:    "table",
		TimeoutMs: 5000,
		Assertions: []Assertion{
			{Type: "exit_code", Value: 0},
		},
	}
	r := NewRunner(bin, "", false, "")
	result := r.Run(s)
	if result.Status != StatusFail {
		t.Errorf("expected FAIL, got %s", result.Status)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestRunner_StderrCaptured(t *testing.T) {
	bin := buildFakeBinary(t, fakeBinExitOne)
	s := Scenario{
		Name: "stderr", Service: "svc", Args: []string{"x"},
		Format: "table", TimeoutMs: 5000,
		Assertions: []Assertion{{Type: "exit_code", Value: 1}},
	}
	r := NewRunner(bin, "", false, "")
	result := r.Run(s)
	if result.Stderr == "" {
		t.Error("expected stderr to be captured")
	}
}

func TestRunner_Timeout(t *testing.T) {
	bin := buildFakeBinary(t, `package main
import "time"
func main() { time.Sleep(10 * time.Second) }`)
	s := Scenario{
		Name: "timeout", Service: "svc", Args: []string{"x"},
		Format: "table", TimeoutMs: 200,
		Assertions: []Assertion{{Type: "exit_code", Value: 0}},
	}
	r := NewRunner(bin, "", false, "")
	result := r.Run(s)
	if result.Status != StatusTimeout {
		t.Errorf("expected TIMEOUT, got %s", result.Status)
	}
}

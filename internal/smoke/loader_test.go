package smoke

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

func TestLoad_ValidScenario(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "s1.yaml"), `
name: "gitcode/issues-list"
service: gitcode
args: ["issues", "list", "--owner", "org"]
format: table
timeout_ms: 3000
assertions:
  - type: exit_code
    value: 0
  - type: table_has_columns
    values: ["Title", "State"]
`)
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	s := scenarios[0]
	if s.Name != "gitcode/issues-list" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Service != "gitcode" {
		t.Errorf("service = %q", s.Service)
	}
	if s.TimeoutMs != 3000 {
		t.Errorf("timeout_ms = %d", s.TimeoutMs)
	}
	if len(s.Assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(s.Assertions))
	}
	if s.Assertions[0].Type != "exit_code" || s.Assertions[0].Value != 0 {
		t.Errorf("assertion[0] = %+v", s.Assertions[0])
	}
	if s.Assertions[1].Type != "table_has_columns" || len(s.Assertions[1].Values) != 2 {
		t.Errorf("assertion[1] = %+v", s.Assertions[1])
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "minimal.yaml"), `
name: "gitcode/minimal"
service: gitcode
args: ["issues", "list"]
assertions:
  - type: exit_code
    value: 0
`)
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := scenarios[0]
	if s.Format != "table" {
		t.Errorf("default format should be 'table', got %q", s.Format)
	}
	if s.TimeoutMs != 10000 {
		t.Errorf("default timeout_ms should be 10000, got %d", s.TimeoutMs)
	}
}

func TestLoad_MissingName_Error(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `
service: gitcode
args: ["issues", "list"]
assertions:
  - type: exit_code
    value: 0
`)
	_, err := LoadScenarios(dir)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoad_MissingService_Error(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `
name: "gitcode/no-service"
args: ["issues", "list"]
assertions:
  - type: exit_code
    value: 0
`)
	_, err := LoadScenarios(dir)
	if err == nil {
		t.Fatal("expected error for missing service")
	}
}

func TestLoad_InvalidName_Spaces(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `
name: "GitCode issues list"
service: gitcode
args: ["issues", "list"]
assertions:
  - type: exit_code
    value: 0
`)
	_, err := LoadScenarios(dir)
	if err == nil {
		t.Fatal("expected error for name with spaces")
	}
}

func TestLoad_InvalidName_UpperCase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `
name: "Gitcode/Issues-List"
service: gitcode
args: ["issues", "list"]
assertions:
  - type: exit_code
    value: 0
`)
	_, err := LoadScenarios(dir)
	if err == nil {
		t.Fatal("expected error for name with uppercase letters")
	}
}

func TestLoad_ValidName_WithDotAndSlash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ok.yaml"), `
name: "gitcode/repos-list.v2"
service: gitcode
args: ["repos", "list"]
assertions:
  - type: exit_code
    value: 0
`)
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scenarios[0].Name != "gitcode/repos-list.v2" {
		t.Errorf("name = %q", scenarios[0].Name)
	}
}

func TestLoad_EmptyDir_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scenarios) != 0 {
		t.Errorf("expected 0 scenarios, got %d", len(scenarios))
	}
}

func TestLoad_RecursiveSubdirs(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "gitcode")
	_ = os.MkdirAll(sub, 0755)
	writeFile(t, filepath.Join(sub, "s1.yaml"), `
name: "gitcode/issues-list"
service: gitcode
args: ["issues", "list"]
assertions:
  - type: exit_code
    value: 0
`)
	writeFile(t, filepath.Join(sub, "s2.yaml"), `
name: "gitcode/repos-list"
service: gitcode
args: ["repos", "list"]
assertions:
  - type: exit_code
    value: 0
`)
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scenarios) != 2 {
		t.Errorf("expected 2 scenarios, got %d", len(scenarios))
	}
}

func TestLoad_SkipScenario(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skip.yaml"), `
name: "etherpad/pads-list"
service: etherpad
args: ["pad", "list"]
skip: true
skip_reason: "staging not available"
assertions:
  - type: exit_code
    value: 0
`)
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scenarios[0].Skip {
		t.Error("expected skip=true")
	}
	if scenarios[0].SkipReason != "staging not available" {
		t.Errorf("skip_reason = %q", scenarios[0].SkipReason)
	}
}

func TestLoad_EmptyFile_Skipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "empty.yaml"), "")
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scenarios) != 0 {
		t.Errorf("expected 0 scenarios, got %d", len(scenarios))
	}
}

func TestLoad_CommentsOnly_Skipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "comments.yaml"), `
# name: "disabled"
# service: etherpad
# skip: true
`)
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scenarios) != 0 {
		t.Errorf("expected 0 scenarios, got %d", len(scenarios))
	}
}

func TestLoad_FilePath_Populated(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.yaml")
	writeFile(t, p, `
name: "gitcode/fp-test"
service: gitcode
args: ["issues", "list"]
assertions:
  - type: exit_code
    value: 0
`)
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scenarios[0].FilePath != p {
		t.Errorf("FilePath = %q, want %q", scenarios[0].FilePath, p)
	}
}

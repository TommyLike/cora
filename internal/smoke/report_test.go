package smoke

import (
	"strings"
	"testing"
)

func makeReport(statuses ...Status) *RunReport {
	r := &RunReport{GeneratedAt: "2026-04-19 10:00:00", ConfigPath: "/tmp/smoke.yaml"}
	for i, st := range statuses {
		res := ScenarioResult{
			Scenario: Scenario{Name: "scenario-" + string(rune('A'+i)), Service: "svc"},
			Status:   st,
		}
		if st != StatusSkip {
			res.AssertionResults = []AssertionResult{
				{Assertion: Assertion{Type: "exit_code", Value: 0}, Passed: st == StatusPass},
			}
		}
		r.Results = append(r.Results, res)
	}
	return r
}

func TestReport_Counts(t *testing.T) {
	r := makeReport(StatusPass, StatusPass, StatusFail, StatusSkip)
	if r.Passed() != 2 {
		t.Errorf("Passed() = %d, want 2", r.Passed())
	}
	if r.Failed() != 1 {
		t.Errorf("Failed() = %d, want 1", r.Failed())
	}
	if r.Skipped() != 1 {
		t.Errorf("Skipped() = %d, want 1", r.Skipped())
	}
}

func TestHTMLReport_ContainsServiceName(t *testing.T) {
	r := makeReport(StatusPass, StatusFail)
	html, err := GenerateHTML(r)
	if err != nil {
		t.Fatalf("GenerateHTML: %v", err)
	}
	if !strings.Contains(html, "svc") {
		t.Error("HTML report should contain service name 'svc'")
	}
}

func TestHTMLReport_ContainsSummary(t *testing.T) {
	r := makeReport(StatusPass, StatusFail, StatusSkip)
	html, err := GenerateHTML(r)
	if err != nil {
		t.Fatalf("GenerateHTML: %v", err)
	}
	for _, want := range []string{"CORA Smoke Test Report", "2026-04-19"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

func TestHTMLReport_IsSelfContained(t *testing.T) {
	r := makeReport(StatusPass)
	html, err := GenerateHTML(r)
	if err != nil {
		t.Fatalf("GenerateHTML: %v", err)
	}
	// Should not reference any external URLs (CDN, fonts, etc.)
	for _, ext := range []string{"cdn.jsdelivr", "fonts.googleapis", "unpkg.com"} {
		if strings.Contains(html, ext) {
			t.Errorf("HTML references external URL: %s", ext)
		}
	}
}

func TestHTMLReport_ContainsScenarioName(t *testing.T) {
	r := &RunReport{
		GeneratedAt: "2026-04-19",
		Results: []ScenarioResult{
			{Scenario: Scenario{Name: "my-unique-scenario-name", Service: "svc"}, Status: StatusPass},
		},
	}
	html, err := GenerateHTML(r)
	if err != nil {
		t.Fatalf("GenerateHTML: %v", err)
	}
	if !strings.Contains(html, "my-unique-scenario-name") {
		t.Error("HTML should contain scenario name")
	}
}

func TestHTMLReport_FailedScenarioExpandedByDefault(t *testing.T) {
	r := makeReport(StatusFail)
	html, err := GenerateHTML(r)
	if err != nil {
		t.Fatalf("GenerateHTML: %v", err)
	}
	// Failing scenarios should have "open" class on detail div so they're expanded.
	if !strings.Contains(html, "detail open") {
		t.Error("failed scenario detail should have 'open' class")
	}
}

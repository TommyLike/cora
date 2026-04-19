package smoke

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

// PrintConsole writes a real-time result line for one scenario.
func PrintConsole(result ScenarioResult) {
	icon := map[Status]string{
		StatusPass:    "[PASS]",
		StatusFail:    "[FAIL]",
		StatusSkip:    "[SKIP]",
		StatusTimeout: "[TIMEOUT]",
		StatusError:   "[ERROR]",
	}[result.Status]

	suffix := ""
	switch result.Status {
	case StatusSkip:
		if result.Scenario.SkipReason != "" {
			suffix = " — " + result.Scenario.SkipReason
		}
	case StatusPass:
		// no suffix
	default:
		for _, ar := range result.AssertionResults {
			if !ar.Passed {
				suffix = " — " + ar.Message
				break
			}
		}
		if result.Err != "" {
			suffix = " — " + result.Err
		}
	}

	fmt.Printf("%-12s %-45s (%dms)%s\n", icon, result.Scenario.Name, result.DurationMs, suffix)
}

// PrintSummary writes the final counts and report path.
func PrintSummary(report *RunReport, reportPath string) {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 70))
	total := time.Duration(report.TotalDurationMs) * time.Millisecond
	fmt.Printf("Result: %d passed, %d failed, %d skipped  | Total: %s\n",
		report.Passed(), report.Failed(), report.Skipped(), total.Round(time.Millisecond))
	if reportPath != "" {
		fmt.Printf("Report: %s\n", reportPath)
	}
}

// serviceGroup is used by the HTML template.
type serviceGroup struct {
	Service     string
	Results     []ScenarioResult
	PassCount   int
	TotalCount  int
	HeaderClass string
}

// GenerateHTML renders a self-contained HTML report.
func GenerateHTML(report *RunReport) (string, error) {
	// Group results by service, preserving first-seen order.
	var order []string
	groups := map[string]*serviceGroup{}
	for _, res := range report.Results {
		svc := res.Scenario.Service
		if _, ok := groups[svc]; !ok {
			order = append(order, svc)
			groups[svc] = &serviceGroup{Service: svc}
		}
		g := groups[svc]
		// Sort: failures first, then passes, then skips.
		if res.Status == StatusFail || res.Status == StatusTimeout || res.Status == StatusError {
			g.Results = append([]ScenarioResult{res}, g.Results...)
		} else {
			g.Results = append(g.Results, res)
		}
		g.TotalCount++
		if res.Status == StatusPass {
			g.PassCount++
		}
	}

	orderedGroups := make([]serviceGroup, 0, len(order))
	for _, svc := range order {
		g := groups[svc]
		// Determine header colour class.
		allSkip := true
		for _, r := range g.Results {
			if r.Status != StatusSkip {
				allSkip = false
				break
			}
		}
		switch {
		case allSkip:
			g.HeaderClass = "all-skip"
		case g.PassCount == g.TotalCount:
			g.HeaderClass = "all-pass"
		default:
			g.HeaderClass = "has-fail"
		}
		orderedGroups = append(orderedGroups, *g)
	}

	total := time.Duration(report.TotalDurationMs) * time.Millisecond

	data := struct {
		GeneratedAt   string
		ConfigPath    string
		TotalDuration string
		Passed        int
		Failed        int
		Skipped       int
		Groups        []serviceGroup
	}{
		GeneratedAt:   report.GeneratedAt,
		ConfigPath:    report.ConfigPath,
		TotalDuration: total.Round(time.Millisecond).String(),
		Passed:        report.Passed(),
		Failed:        report.Failed(),
		Skipped:       report.Skipped(),
		Groups:        orderedGroups,
	}

	funcMap := template.FuncMap{
		"statusIcon": func(s Status) string {
			switch s {
			case StatusPass:
				return "&#9989;"
			case StatusFail:
				return "&#10060;"
			case StatusTimeout:
				return "&#9203;"
			case StatusError:
				return "&#128165;"
			default:
				return "&#9193;"
			}
		},
		"badgeClass": func(s Status) string {
			switch s {
			case StatusPass:
				return "badge-pass"
			case StatusFail:
				return "badge-fail"
			case StatusTimeout:
				return "badge-timeout"
			case StatusError:
				return "badge-error"
			default:
				return "badge-skip"
			}
		},
		"assertionDesc": AssertionDesc,
		"not":           func(b bool) bool { return !b },
		"isNotPass":     func(s Status) bool { return s != StatusPass },
		"isSkip":        func(s Status) bool { return s == StatusSkip },
	}

	tmpl, err := template.New("report").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>CORA Smoke Test Report</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#f1f5f9;color:#1e293b;padding:24px;font-size:14px}
h1{font-size:22px;margin-bottom:4px}
.meta{color:#64748b;font-size:13px;margin-bottom:20px}
.summary{display:flex;gap:12px;margin-bottom:20px}
.sc{background:#fff;border-radius:8px;padding:16px 20px;flex:1;box-shadow:0 1px 2px rgba(0,0,0,.08);text-align:center}
.sc .count{font-size:32px;font-weight:700}
.sc .label{font-size:12px;color:#64748b;text-transform:uppercase;letter-spacing:.05em}
.pass .count{color:#16a34a}.fail .count{color:#dc2626}.skip .count{color:#94a3b8}
.group{background:#fff;border-radius:8px;margin-bottom:12px;box-shadow:0 1px 2px rgba(0,0,0,.08);overflow:hidden}
.gh{padding:10px 16px;background:#f8fafc;border-bottom:1px solid #e2e8f0;display:flex;justify-content:space-between;align-items:center;font-weight:600;font-size:13px}
.gh.all-pass{border-left:4px solid #16a34a}
.gh.has-fail{border-left:4px solid #dc2626}
.gh.all-skip{border-left:4px solid #94a3b8}
.scenario{border-bottom:1px solid #f1f5f9}
.scenario:last-child{border-bottom:none}
.sh{padding:10px 16px;display:flex;justify-content:space-between;align-items:center;cursor:pointer;user-select:none}
.sh:hover{background:#f8fafc}
.sn{display:flex;align-items:center;gap:8px}
.dur{color:#94a3b8;font-size:12px}
.detail{padding:12px 16px 16px;display:none;border-top:1px solid #f1f5f9;background:#fafafa}
.detail.open{display:block}
.ar{display:flex;gap:6px;padding:3px 0;font-size:13px;align-items:flex-start;line-height:1.4}
.aok{color:#16a34a;flex-shrink:0}.afl{color:#dc2626;flex-shrink:0}
.ob{margin-top:10px}
.ol{font-size:11px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:.05em;margin-bottom:4px}
.op{background:#1e1e2e;color:#cdd6f4;padding:10px;border-radius:6px;font-family:'JetBrains Mono',monospace;font-size:12px;white-space:pre-wrap;word-break:break-all;max-height:250px;overflow-y:auto}
.oe{color:#94a3b8;font-style:italic;font-size:13px}
.badge{font-size:11px;padding:2px 8px;border-radius:10px;font-weight:600}
.badge-pass{background:#dcfce7;color:#15803d}
.badge-fail{background:#fee2e2;color:#b91c1c}
.badge-skip{background:#f1f5f9;color:#475569}
.badge-timeout{background:#ffedd5;color:#c2410c}
.badge-error{background:#fef9c3;color:#92400e}
.skip-r{color:#94a3b8;font-size:13px;font-style:italic;padding:8px 16px}
</style>
</head>
<body>
<h1>&#128269; CORA Smoke Test Report</h1>
<div class="meta">Generated: {{.GeneratedAt}} &nbsp;|&nbsp; Duration: {{.TotalDuration}} &nbsp;|&nbsp; Config: {{.ConfigPath}}</div>
<div class="summary">
  <div class="sc pass"><div class="count">{{.Passed}}</div><div class="label">Passed</div></div>
  <div class="sc fail"><div class="count">{{.Failed}}</div><div class="label">Failed</div></div>
  <div class="sc skip"><div class="count">{{.Skipped}}</div><div class="label">Skipped</div></div>
</div>
{{range .Groups}}
<div class="group">
  <div class="gh {{.HeaderClass}}">
    <span>{{.Service}}</span>
    <span>{{.PassCount}} / {{.TotalCount}}</span>
  </div>
  {{range .Results}}
  {{if isSkip .Status}}
  <div class="scenario">
    <div class="sh" style="cursor:default">
      <div class="sn"><span>&#9193;</span><span>{{.Scenario.Name}}</span></div>
      <span class="badge badge-skip">SKIP</span>
    </div>
    {{if .Scenario.SkipReason}}<div class="skip-r">{{.Scenario.SkipReason}}</div>{{end}}
  </div>
  {{else}}
  <div class="scenario">
    <div class="sh" onclick="toggle(this)">
      <div class="sn"><span>{{statusIcon .Status}}</span><span>{{.Scenario.Name}}</span></div>
      <div style="display:flex;gap:10px;align-items:center">
        <span class="dur">{{.DurationMs}}ms</span>
        <span class="badge {{badgeClass .Status}}">{{.Status}}</span>
      </div>
    </div>
    <div class="detail{{if isNotPass .Status}} open{{end}}">
      <div>
        {{range .AssertionResults}}
        <div class="ar">
          <span class="{{if .Passed}}aok{{else}}afl{{end}}">{{if .Passed}}&#10003;{{else}}&#10007;{{end}}</span>
          <span>{{assertionDesc .Assertion}}{{if not .Passed}} &mdash; <em>{{.Message}}</em>{{end}}</span>
        </div>
        {{end}}
      </div>
      <div class="ob">
        <div class="ol">Stdout</div>
        {{if .Stdout}}<pre class="op">{{.Stdout}}</pre>{{else}}<div class="oe">(empty)</div>{{end}}
      </div>
      <div class="ob" style="margin-top:8px">
        <div class="ol">Stderr</div>
        {{if .Stderr}}<pre class="op">{{.Stderr}}</pre>{{else}}<div class="oe">(empty)</div>{{end}}
      </div>
    </div>
  </div>
  {{end}}
  {{end}}
</div>
{{end}}
<script>
function toggle(h){var d=h.nextElementSibling;d.classList.toggle('open')}
</script>
</body>
</html>`

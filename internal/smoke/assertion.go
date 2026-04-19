package smoke

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cncf/cora/internal/view"
)

// EvalContext carries scenario-level metadata needed by assertions that look
// beyond raw stdout/stderr (e.g. view_columns_match).
type EvalContext struct {
	ViewRegistry *view.Registry // nil when --views was not provided
	Service      string
	Resource     string // first non-flag arg, e.g. "issues"
	Verb         string // second non-flag arg, e.g. "list"
	Format       string // scenario format: "table" | "json" | "yaml"
}

// EvaluateAssertion checks one Assertion against captured execution output.
func EvaluateAssertion(a Assertion, ctx EvalContext, stdout, stderr string, exitCode int, durationMs int64) AssertionResult {
	res := AssertionResult{Assertion: a}
	switch a.Type {
	case "exit_code":
		res.Actual = fmt.Sprintf("%d", exitCode)
		if exitCode == a.Value {
			res.Passed = true
		} else {
			res.Message = fmt.Sprintf("expected exit code %d, got %d", a.Value, exitCode)
		}

	case "response_time_lt":
		res.Actual = fmt.Sprintf("%dms", durationMs)
		if durationMs < int64(a.Value) {
			res.Passed = true
		} else {
			res.Message = fmt.Sprintf("expected < %dms, got %dms", a.Value, durationMs)
		}

	case "stdout_not_empty":
		res.Actual = fmt.Sprintf("len=%d", len(strings.TrimSpace(stdout)))
		if strings.TrimSpace(stdout) != "" {
			res.Passed = true
		} else {
			res.Message = "stdout is empty"
		}

	case "stdout_contains":
		res.Actual = truncStr(stdout, 80)
		if strings.Contains(stdout, a.Str) {
			res.Passed = true
		} else {
			res.Message = fmt.Sprintf("stdout does not contain %q", a.Str)
		}

	case "stdout_not_contains":
		res.Actual = truncStr(stdout, 80)
		if !strings.Contains(stdout, a.Str) {
			res.Passed = true
		} else {
			res.Message = fmt.Sprintf("stdout contains unexpected %q", a.Str)
		}

	case "stderr_not_contains":
		res.Actual = truncStr(stderr, 80)
		if !strings.Contains(stderr, a.Str) {
			res.Passed = true
		} else {
			res.Message = fmt.Sprintf("stderr contains unexpected %q", a.Str)
		}

	case "table_has_columns":
		header := tableHeader(stdout)
		var missing []string
		for _, col := range a.Values {
			if !strings.Contains(strings.ToUpper(header), strings.ToUpper(col)) {
				missing = append(missing, col)
			}
		}
		res.Actual = fmt.Sprintf("header=%q", truncStr(header, 100))
		if len(missing) == 0 {
			res.Passed = true
		} else {
			res.Message = fmt.Sprintf("missing columns: %s", strings.Join(missing, ", "))
		}

	case "table_row_count_gte":
		count := tableDataRowCount(stdout)
		res.Actual = fmt.Sprintf("%d rows", count)
		if count >= a.Value {
			res.Passed = true
		} else {
			res.Message = fmt.Sprintf("expected >= %d rows, got %d", a.Value, count)
		}

	case "json_has_keys":
		var obj map[string]any
		res.Actual = truncStr(strings.TrimSpace(stdout), 80)
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
			res.Message = fmt.Sprintf("stdout is not valid JSON: %v", err)
			return res
		}
		var missing []string
		for _, key := range a.Values {
			if _, ok := obj[key]; !ok {
				missing = append(missing, key)
			}
		}
		if len(missing) == 0 {
			res.Passed = true
		} else {
			res.Message = fmt.Sprintf("missing keys: %s", strings.Join(missing, ", "))
		}

	case "json_key_not_empty":
		var obj map[string]any
		res.Actual = truncStr(strings.TrimSpace(stdout), 80)
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
			res.Message = fmt.Sprintf("stdout is not valid JSON: %v", err)
			return res
		}
		val, exists := obj[a.Str]
		if !exists {
			res.Message = fmt.Sprintf("key %q not found", a.Str)
			return res
		}
		switch v := val.(type) {
		case nil:
			res.Message = fmt.Sprintf("key %q is null", a.Str)
		case string:
			if v == "" {
				res.Message = fmt.Sprintf("key %q is empty string", a.Str)
			} else {
				res.Passed = true
			}
		default:
			res.Passed = true
		}

	case "view_columns_match":
		// Prerequisite: format must be table.
		if ctx.Format != "table" {
			res.Message = fmt.Sprintf("view_columns_match requires format=table, got %q", ctx.Format)
			return res
		}
		if ctx.ViewRegistry == nil {
			res.Message = "no view registry configured; pass --views flag to smoke runner"
			return res
		}
		if ctx.Resource == "" || ctx.Verb == "" {
			res.Message = "cannot derive resource/verb from scenario args (need at least 2 positional args)"
			return res
		}
		cfg := ctx.ViewRegistry.Lookup(ctx.Service, ctx.Resource, ctx.Verb)
		if cfg == nil {
			res.Message = fmt.Sprintf("no view defined for %s/%s/%s; add column definitions to views.yaml", ctx.Service, ctx.Resource, ctx.Verb)
			return res
		}

		headerCells, dataRows := parseTable(stdout)
		res.Actual = fmt.Sprintf("header=%v rows=%d", headerCells, len(dataRows))

		// Build case-insensitive index: uppercase label → column index.
		colIndex := make(map[string]int, len(headerCells))
		for i, h := range headerCells {
			colIndex[strings.ToUpper(h)] = i
		}

		var missingHeader []string  // columns absent from header
		var emptyValue []string     // columns present but all values empty across all rows

		for _, col := range cfg.Columns {
			label := view.LabelFor(col)
			idx, found := colIndex[strings.ToUpper(label)]
			if !found {
				missingHeader = append(missingHeader, label)
				continue
			}
			// If there are data rows, at least one must have a non-empty value.
			if len(dataRows) > 0 {
				hasValue := false
				for _, row := range dataRows {
					if idx < len(row) && strings.TrimSpace(row[idx]) != "" {
						hasValue = true
						break
					}
				}
				if !hasValue {
					emptyValue = append(emptyValue, label)
				}
			}
		}

		var problems []string
		if len(missingHeader) > 0 {
			problems = append(problems, fmt.Sprintf("missing columns: %s", strings.Join(missingHeader, ", ")))
		}
		if len(emptyValue) > 0 {
			problems = append(problems, fmt.Sprintf("columns with all-empty values: %s", strings.Join(emptyValue, ", ")))
		}
		if len(problems) == 0 {
			res.Passed = true
		} else {
			res.Message = strings.Join(problems, "; ")
		}

	default:
		res.Message = fmt.Sprintf("unknown assertion type: %q", a.Type)
	}
	return res
}

// AssertionDesc returns a short human-readable description of an assertion.
func AssertionDesc(a Assertion) string {
	switch a.Type {
	case "exit_code":
		return fmt.Sprintf("exit_code = %d", a.Value)
	case "response_time_lt":
		return fmt.Sprintf("response_time < %dms", a.Value)
	case "stdout_not_empty":
		return "stdout is not empty"
	case "stdout_contains":
		return fmt.Sprintf("stdout contains %q", a.Str)
	case "stdout_not_contains":
		return fmt.Sprintf("stdout does not contain %q", a.Str)
	case "stderr_not_contains":
		return fmt.Sprintf("stderr does not contain %q", a.Str)
	case "table_has_columns":
		return fmt.Sprintf("table has columns: %s", strings.Join(a.Values, ", "))
	case "table_row_count_gte":
		return fmt.Sprintf("table has >= %d rows", a.Value)
	case "json_has_keys":
		return fmt.Sprintf("JSON has keys: %s", strings.Join(a.Values, ", "))
	case "json_key_not_empty":
		return fmt.Sprintf("JSON[%q] is not empty", a.Str)
	case "view_columns_match":
		return "all view-defined columns present in table output"
	default:
		return fmt.Sprintf("unknown(%s)", a.Type)
	}
}

// parseTable parses tablewriter output into a header cell slice and a slice of
// data row cell slices. Each cell is trimmed of surrounding whitespace.
//
// tablewriter format:
//
//	+------+-------+
//	| COL1 | COL2  |   ← header (first | row after first + row)
//	+------+-------+
//	| v1   | v2    |   ← data rows (| rows after second + row)
//	+------+-------+
func parseTable(stdout string) (header []string, dataRows [][]string) {
	lines := strings.Split(stdout, "\n")
	sepCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+") {
			sepCount++
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := splitTableRow(trimmed)
		if sepCount == 1 {
			header = cells
		} else if sepCount >= 2 {
			dataRows = append(dataRows, cells)
		}
	}
	return
}

// splitTableRow splits a tablewriter row string into trimmed cell values,
// preserving empty cells so that column indices stay aligned.
// "| foo |     | bar |" → ["foo", "", "bar"]
func splitTableRow(row string) []string {
	parts := strings.Split(row, "|")
	// parts[0] is "" (before leading |) and parts[last] is "" (after trailing |).
	// The actual cells are the elements in between.
	if len(parts) < 3 {
		return nil
	}
	inner := parts[1 : len(parts)-1]
	cells := make([]string, len(inner))
	for i, p := range inner {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// tableHeader extracts the first header row (the | line after the first + separator).
func tableHeader(stdout string) string {
	lines := strings.Split(stdout, "\n")
	seenSep := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+") {
			seenSep = true
			continue
		}
		if seenSep && strings.HasPrefix(trimmed, "|") {
			return trimmed
		}
	}
	return ""
}

// tableDataRowCount counts data rows (| lines after the second + separator).
func tableDataRowCount(stdout string) int {
	lines := strings.Split(stdout, "\n")
	sepCount := 0
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+") {
			sepCount++
			continue
		}
		if sepCount >= 2 && strings.HasPrefix(trimmed, "|") {
			count++
		}
	}
	return count
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

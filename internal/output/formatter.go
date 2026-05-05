package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v3"

	"github.com/cncf/cora/internal/view"
)

// Print renders raw JSON response bytes in the requested format.
//
//   - "json"        → pretty-print full JSON, ignoring viewCfg
//   - "yaml"        → convert to YAML, ignoring viewCfg
//   - "table" / ""  → apply viewCfg if non-nil; generic fallback otherwise
func Print(data []byte, format string, viewCfg *view.ViewConfig) error {
	switch strings.ToLower(format) {
	case "json":
		return printJSON(data)
	case "yaml":
		return printYAML(data)
	default: // "table" or empty
		if viewCfg != nil {
			return printView(data, viewCfg)
		}
		return printTable(data)
	}
}

// ── JSON ──────────────────────────────────────────────────────────────────────

func printJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		// Wrap non-JSON content (e.g. text/plain) so --format json
		// always produces valid JSON.
		wrapped := map[string]string{"content": string(data)}
		pretty, err := json.MarshalIndent(wrapped, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(pretty))
		return nil
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(pretty))
	return nil
}

// ── YAML ──────────────────────────────────────────────────────────────────────

func printYAML(data []byte) error {
	// JSON → interface{} → YAML preserves the full structure.
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		// Wrap non-JSON content (e.g. text/plain) so --format yaml
		// always produces valid YAML.
		wrapped := map[string]string{"content": string(data)}
		out, err := yaml.Marshal(wrapped)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		return nil
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Print(string(out))
	return nil
}

// ── View-aware rendering ──────────────────────────────────────────────────────

func printView(data []byte, cfg *view.ViewConfig) error {
	items, obj := view.DetectItems(data, cfg)

	if items != nil {
		renderListTable(items, cfg.Columns)
		return nil
	}
	if obj != nil {
		renderKVTable(obj, cfg.Columns)
		return nil
	}
	// Fallback: print raw JSON.
	return printJSON(data)
}

// renderKVTable renders a single object as a two-column key/value table.
// Each ViewColumn becomes one row: left = label, right = formatted value.
func renderKVTable(obj map[string]interface{}, cols []view.ViewColumn) {
	t := tablewriter.NewWriter(os.Stdout)
	t.SetHeader([]string{"Field", "Value"})
	t.SetBorder(true)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetAutoWrapText(true)
	t.SetColWidth(80)

	for _, col := range cols {
		val := view.ExtractField(obj, col.Field)
		rendered := view.FormatValue(val, col)
		label := view.LabelFor(col)
		t.Append([]string{label, sanitize(rendered, col.Format)})
	}
	t.Render()
}

// renderListTable renders a slice of objects as a horizontal table,
// one ViewColumn per table column.
func renderListTable(items []map[string]interface{}, cols []view.ViewColumn) {
	if len(items) == 0 {
		fmt.Println("(no results)")
		return
	}

	headers := make([]string, len(cols))
	for i, col := range cols {
		headers[i] = view.LabelFor(col)
	}

	t := tablewriter.NewWriter(os.Stdout)
	t.SetHeader(headers)
	t.SetBorder(false)
	t.SetAutoWrapText(false)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)

	// Apply fixed column widths where specified.
	for i, col := range cols {
		if col.Width > 0 {
			t.SetColMinWidth(i, col.Width)
		}
	}

	for _, item := range items {
		row := make([]string, len(cols))
		for i, col := range cols {
			val := view.ExtractField(item, col.Field)
			// In list mode, always collapse multiline to a single line.
			rendered := view.FormatValue(val, col)
			rendered = strings.ReplaceAll(rendered, "\n", " ")
			rendered = strings.ReplaceAll(rendered, "\r", "")
			row[i] = sanitize(rendered, view.FormatText)
		}
		t.Append(row)
	}
	t.Render()
}

// ── Generic fallback (no ViewConfig) ─────────────────────────────────────────

func printTable(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		// Wrap non-JSON content (e.g. text/plain) so --format table
		// always produces a readable result.
		fmt.Println(string(data))
		return nil
	}

	items := extractItems(v)
	if len(items) > 0 {
		return printListTable(items)
	}
	if obj, ok := v.(map[string]interface{}); ok {
		printKVTable(obj)
		return nil
	}
	return printJSON(data)
}

// extractItems finds the primary list payload in a JSON response.
func extractItems(v interface{}) []map[string]interface{} {
	if arr, ok := v.([]interface{}); ok {
		return toObjectSlice(arr)
	}
	if obj, ok := v.(map[string]interface{}); ok {
		skip := map[string]bool{"meta": true, "pagination": true, "links": true}
		for k, val := range obj {
			if skip[k] || strings.HasPrefix(k, "_") {
				continue
			}
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				if items := toObjectSlice(arr); len(items) > 0 {
					return items
				}
			}
		}
	}
	return nil
}

func toObjectSlice(items []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

// printListTable renders a generic list using auto-selected headers.
func printListTable(items []map[string]interface{}) error {
	if len(items) == 0 {
		fmt.Println("(no results)")
		return nil
	}
	headers := selectHeaders(items, 7)
	t := tablewriter.NewWriter(os.Stdout)
	t.SetHeader(headers)
	t.SetBorder(false)
	t.SetAutoWrapText(false)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)

	for _, item := range items {
		row := make([]string, len(headers))
		for i, h := range headers {
			row[i] = sanitize(stringify(item[h], 60), view.FormatText)
		}
		t.Append(row)
	}
	t.Render()
	return nil
}

// printKVTable renders a single object as a generic two-column key/value table.
func printKVTable(obj map[string]interface{}) {
	t := tablewriter.NewWriter(os.Stdout)
	t.SetHeader([]string{"Field", "Value"})
	t.SetBorder(true)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetAutoWrapText(true)
	t.SetColWidth(80)

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		t.Append([]string{k, sanitize(stringify(obj[k], 200), view.FormatText)})
	}
	t.Render()
}

// selectHeaders picks up to maxCols column names from the first item.
func selectHeaders(items []map[string]interface{}, maxCols int) []string {
	preferred := []string{"id", "number", "name", "title", "username", "state", "created_at", "updated_at"}
	seen := map[string]bool{}
	var headers []string

	for _, pref := range preferred {
		if len(headers) >= maxCols {
			break
		}
		if _, ok := items[0][pref]; ok && !seen[pref] {
			headers = append(headers, pref)
			seen[pref] = true
		}
	}
	for k := range items[0] {
		if len(headers) >= maxCols {
			break
		}
		if !seen[k] {
			headers = append(headers, k)
			seen[k] = true
		}
	}
	return headers
}

func stringify(v interface{}, maxLen int) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}

// sanitize strips ASCII control characters to prevent terminal injection.
// In multiline mode, newlines are preserved.
func sanitize(s string, format view.ColumnFormat) string {
	var b strings.Builder
	for _, r := range s {
		if format == view.FormatMultiline {
			if r >= 0x20 || r == '\t' || r == '\n' {
				b.WriteRune(r)
			}
		} else {
			if r >= 0x20 || r == '\t' {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

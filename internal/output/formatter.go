package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
)

// Print renders raw JSON response bytes in the requested format.
// Supported formats: "table" (default), "json".
func Print(data []byte, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return printJSON(data)
	case "table", "":
		return printTable(data)
	default:
		return printJSON(data)
	}
}

// --- JSON ---

func printJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		// Not valid JSON; print raw.
		fmt.Println(string(data))
		return nil
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(pretty))
	return nil
}

// --- Table ---

func printTable(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Println(string(data))
		return nil
	}

	items := extractItems(v)
	switch {
	case len(items) > 0:
		return printListTable(items)
	default:
		if obj, ok := v.(map[string]interface{}); ok {
			printKVTable(obj)
			return nil
		}
		return printJSON(data)
	}
}

// extractItems finds the primary list payload inside a JSON response.
// It handles both top-level arrays and the common "{ items: [...] }" envelope.
func extractItems(v interface{}) []map[string]interface{} {
	if arr, ok := v.([]interface{}); ok {
		return toObjectSlice(arr)
	}
	if obj, ok := v.(map[string]interface{}); ok {
		// Skip meta/pagination fields and look for the first non-empty array.
		skip := map[string]bool{
			"meta": true, "pagination": true, "links": true,
		}
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

// printListTable renders a slice of objects as a columnar table.
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
			row[i] = sanitize(stringify(item[h], 60))
		}
		t.Append(row)
	}
	t.Render()
	return nil
}

// printKVTable renders a single object as a two-column key/value table.
func printKVTable(obj map[string]interface{}) {
	t := tablewriter.NewWriter(os.Stdout)
	t.SetHeader([]string{"Field", "Value"})
	t.SetBorder(false)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)

	for k, v := range obj {
		t.Append([]string{k, sanitize(stringify(v, 100))})
	}
	t.Render()
}

// selectHeaders picks up to maxCols column names, favouring common fields.
func selectHeaders(items []map[string]interface{}, maxCols int) []string {
	preferred := []string{"id", "username", "topic_id", "topic_slug", "raw", "created_at", "updated_at"}

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
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 || r == '\t' || r == '\n' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

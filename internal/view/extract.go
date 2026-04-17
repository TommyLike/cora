package view

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// LabelFor returns the display label for a column.
// Uses col.Label when set; otherwise title-cases the last dot-segment of col.Field.
func LabelFor(col ViewColumn) string {
	if col.Label != "" {
		return col.Label
	}
	parts := strings.Split(col.Field, ".")
	last := parts[len(parts)-1]
	return toTitle(strings.ReplaceAll(last, "_", " "))
}

// ExtractField navigates a decoded JSON object using a dot-separated path.
// Returns nil when any segment is missing or the intermediate value is not an object.
func ExtractField(obj map[string]interface{}, path string) interface{} {
	if obj == nil || path == "" {
		return nil
	}
	head, tail, _ := strings.Cut(path, ".")
	val, ok := obj[head]
	if !ok {
		return nil
	}
	if tail == "" {
		return val
	}
	nested, ok := val.(map[string]interface{})
	if !ok {
		return nil
	}
	return ExtractField(nested, tail)
}

// FormatValue renders a JSON value according to the column's display settings.
func FormatValue(v interface{}, col ViewColumn) string {
	if v == nil {
		return "—"
	}

	switch col.Format {
	case FormatJSON:
		raw, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		if col.Indent {
			var buf bytes.Buffer
			_ = json.Indent(&buf, raw, "", "  ")
			return applyTruncate(buf.String(), col.Truncate)
		}
		return applyTruncate(string(raw), col.Truncate)

	case FormatDate:
		s := primitiveToString(v)
		for _, layout := range []string{
			time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05",
		} {
			if t, err := time.Parse(layout, s); err == nil {
				dfmt := col.DateFmt
				if dfmt == "" {
					dfmt = "2006-01-02"
				}
				return t.Format(dfmt)
			}
		}
		return s // parse failed: return original

	case FormatMultiline:
		s := primitiveToString(v)
		return applyTruncate(s, col.Truncate)

	default: // FormatText
		var s string
		switch val := v.(type) {
		case map[string]interface{}:
			s = "[object]"
		case []interface{}:
			s = "[array]"
		default:
			s = primitiveToString(val)
		}
		// Collapse whitespace for single-line display.
		s = strings.ReplaceAll(s, "\r\n", " ")
		s = strings.ReplaceAll(s, "\n", " ")
		s = strings.ReplaceAll(s, "\r", " ")
		return applyTruncate(s, col.Truncate)
	}
}

// DetectItems resolves the displayable payload from raw JSON, honouring RootField.
//
//   - Returns (items, nil)  when the effective root is a JSON array.
//   - Returns (nil,   obj)  when it is a JSON object (single-object / get mode).
//   - Returns (nil,   nil)  when parsing fails.
//
// Auto-detection: when RootField is empty and the root object contains exactly
// one non-empty array field, that field is used as the list payload.
func DetectItems(raw []byte, cfg *ViewConfig) ([]map[string]interface{}, map[string]interface{}) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, nil
	}

	// Apply explicit root_field.
	if cfg != nil && cfg.RootField != "" {
		if obj, ok := v.(map[string]interface{}); ok {
			v = obj[cfg.RootField]
		}
	}

	// Top-level array → list mode.
	if arr, ok := v.([]interface{}); ok {
		return toObjSlice(arr), nil
	}

	// Top-level object.
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, nil
	}

	// Auto-detect a wrapped list: exactly one non-empty array field present.
	// Only applies in generic fallback mode (no ViewConfig). When a ViewConfig
	// is registered for the operation, the root node type is authoritative:
	// an array root → list mode (handled above), an object root → object mode.
	// This prevents false positives where a single-object response (e.g. an
	// issue) has exactly one non-empty array field (e.g. labels) and would
	// otherwise be misidentified as a list of label objects.
	if cfg == nil {
		var candidate []map[string]interface{}
		arrayCount := 0
		for _, val := range obj {
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				if items := toObjSlice(arr); len(items) > 0 {
					candidate = items
					arrayCount++
				}
			}
		}
		if arrayCount == 1 {
			return candidate, nil
		}
	}

	return nil, obj
}

// ── internal helpers ──────────────────────────────────────────────────────────

func primitiveToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func applyTruncate(s string, n int) string {
	if n <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func toObjSlice(arr []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

func toTitle(s string) string {
	if s == "" {
		return s
	}
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

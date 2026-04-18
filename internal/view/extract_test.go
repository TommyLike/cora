package view

import (
	"strings"
	"testing"
)

// ── LabelFor ─────────────────────────────────────────────────────────────────

func TestLabelFor_UsesLabelWhenSet(t *testing.T) {
	col := ViewColumn{Field: "user.login", Label: "Author"}
	if got := LabelFor(col); got != "Author" {
		t.Errorf("LabelFor = %q, want %q", got, "Author")
	}
}

func TestLabelFor_DerivesFromField(t *testing.T) {
	cases := []struct {
		field string
		want  string
	}{
		{"id", "Id"},
		{"created_at", "Created At"},
		{"user.login", "Login"},
		{"pull_request.head.label", "Label"},
	}
	for _, tc := range cases {
		col := ViewColumn{Field: tc.field}
		if got := LabelFor(col); got != tc.want {
			t.Errorf("LabelFor(%q) = %q, want %q", tc.field, got, tc.want)
		}
	}
}

// ── ExtractField ─────────────────────────────────────────────────────────────

func TestExtractField_SimpleKey(t *testing.T) {
	obj := map[string]interface{}{"id": float64(42)}
	if got := ExtractField(obj, "id"); got != float64(42) {
		t.Errorf("ExtractField = %v, want 42", got)
	}
}

func TestExtractField_NestedPath(t *testing.T) {
	obj := map[string]interface{}{
		"user": map[string]interface{}{"login": "alice"},
	}
	if got := ExtractField(obj, "user.login"); got != "alice" {
		t.Errorf("ExtractField = %v, want %q", got, "alice")
	}
}

func TestExtractField_MissingKey_ReturnsNil(t *testing.T) {
	obj := map[string]interface{}{"id": 1}
	if got := ExtractField(obj, "missing"); got != nil {
		t.Errorf("ExtractField = %v, want nil", got)
	}
}

func TestExtractField_IntermediateNotObject_ReturnsNil(t *testing.T) {
	obj := map[string]interface{}{"user": "not-an-object"}
	if got := ExtractField(obj, "user.login"); got != nil {
		t.Errorf("ExtractField = %v, want nil", got)
	}
}

func TestExtractField_NilObj_ReturnsNil(t *testing.T) {
	if got := ExtractField(nil, "id"); got != nil {
		t.Errorf("ExtractField(nil) = %v, want nil", got)
	}
}

func TestExtractField_EmptyPath_ReturnsNil(t *testing.T) {
	obj := map[string]interface{}{"id": 1}
	if got := ExtractField(obj, ""); got != nil {
		t.Errorf("ExtractField(empty path) = %v, want nil", got)
	}
}

// ── FormatValue ──────────────────────────────────────────────────────────────

func TestFormatValue_NilReturnsPlaceholder(t *testing.T) {
	if got := FormatValue(nil, ViewColumn{}); got != "—" {
		t.Errorf("FormatValue(nil) = %q, want %q", got, "—")
	}
}

func TestFormatValue_Text_String(t *testing.T) {
	col := ViewColumn{Format: FormatText}
	if got := FormatValue("hello world", col); got != "hello world" {
		t.Errorf("FormatValue = %q, want %q", got, "hello world")
	}
}

func TestFormatValue_Text_CollapsesNewlines(t *testing.T) {
	col := ViewColumn{Format: FormatText}
	got := FormatValue("line1\nline2\r\nline3", col)
	if strings.Contains(got, "\n") || strings.Contains(got, "\r") {
		t.Errorf("FormatValue should collapse newlines, got %q", got)
	}
}

func TestFormatValue_Text_MapReturnsPlaceholder(t *testing.T) {
	col := ViewColumn{Format: FormatText}
	got := FormatValue(map[string]interface{}{"key": "val"}, col)
	if got != "[object]" {
		t.Errorf("FormatValue(map) = %q, want %q", got, "[object]")
	}
}

func TestFormatValue_Text_SliceReturnsPlaceholder(t *testing.T) {
	col := ViewColumn{Format: FormatText}
	got := FormatValue([]interface{}{"a", "b"}, col)
	if got != "[array]" {
		t.Errorf("FormatValue(slice) = %q, want %q", got, "[array]")
	}
}

func TestFormatValue_Text_Truncate(t *testing.T) {
	col := ViewColumn{Format: FormatText, Truncate: 5}
	got := FormatValue("1234567890", col)
	// Truncated to 5 runes + ellipsis.
	if !strings.HasSuffix(got, "…") {
		t.Errorf("FormatValue with truncate should end with '…', got %q", got)
	}
	runes := []rune(got)
	if len(runes) != 6 { // 5 chars + "…"
		t.Errorf("FormatValue with truncate = %d runes, want 6", len(runes))
	}
}

func TestFormatValue_Multiline_PreservesNewlines(t *testing.T) {
	col := ViewColumn{Format: FormatMultiline}
	got := FormatValue("line1\nline2", col)
	if !strings.Contains(got, "\n") {
		t.Errorf("FormatMultiline should preserve newlines, got %q", got)
	}
}

func TestFormatValue_JSON_Compact(t *testing.T) {
	col := ViewColumn{Format: FormatJSON}
	got := FormatValue([]interface{}{"a", "b"}, col)
	if got != `["a","b"]` {
		t.Errorf("FormatJSON = %q, want %q", got, `["a","b"]`)
	}
}

func TestFormatValue_JSON_Indented(t *testing.T) {
	col := ViewColumn{Format: FormatJSON, Indent: true}
	got := FormatValue(map[string]interface{}{"k": "v"}, col)
	if !strings.Contains(got, "\n") {
		t.Errorf("FormatJSON(indent) should contain newlines, got %q", got)
	}
}

func TestFormatValue_Date_ParsesRFC3339(t *testing.T) {
	col := ViewColumn{Format: FormatDate}
	got := FormatValue("2024-03-15T10:30:00Z", col)
	if got != "2024-03-15" {
		t.Errorf("FormatDate = %q, want %q", got, "2024-03-15")
	}
}

func TestFormatValue_Date_CustomFormat(t *testing.T) {
	col := ViewColumn{Format: FormatDate, DateFmt: "01/2006"}
	got := FormatValue("2024-03-15T10:30:00Z", col)
	if got != "03/2024" {
		t.Errorf("FormatDate(custom) = %q, want %q", got, "03/2024")
	}
}

func TestFormatValue_Date_InvalidFallsThrough(t *testing.T) {
	col := ViewColumn{Format: FormatDate}
	got := FormatValue("not-a-date", col)
	if got != "not-a-date" {
		t.Errorf("FormatDate(invalid) = %q, want original %q", got, "not-a-date")
	}
}

func TestFormatValue_Number(t *testing.T) {
	col := ViewColumn{Format: FormatText}
	// JSON numbers unmarshal as float64.
	if got := FormatValue(float64(42), col); got != "42" {
		t.Errorf("FormatValue(42) = %q, want %q", got, "42")
	}
}

func TestFormatValue_Bool(t *testing.T) {
	col := ViewColumn{Format: FormatText}
	if got := FormatValue(true, col); got != "true" {
		t.Errorf("FormatValue(true) = %q, want %q", got, "true")
	}
	if got := FormatValue(false, col); got != "false" {
		t.Errorf("FormatValue(false) = %q, want %q", got, "false")
	}
}

// ── DetectItems ───────────────────────────────────────────────────────────────

func TestDetectItems_TopLevelArray(t *testing.T) {
	raw := []byte(`[{"id":1},{"id":2}]`)
	items, obj := DetectItems(raw, nil)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	if obj != nil {
		t.Error("expected nil obj for array response")
	}
}

func TestDetectItems_TopLevelObject(t *testing.T) {
	raw := []byte(`{"id":1,"title":"foo"}`)
	items, obj := DetectItems(raw, nil)
	if items != nil {
		t.Errorf("expected nil items for object response, got %v", items)
	}
	if obj == nil {
		t.Error("expected non-nil obj")
	}
	if obj["title"] != "foo" {
		t.Errorf("obj[title] = %v, want %q", obj["title"], "foo")
	}
}

func TestDetectItems_WrappedArray_AutoDetect(t *testing.T) {
	// Exactly one non-empty array field → auto-detect as list.
	raw := []byte(`{"items":[{"id":1},{"id":2}]}`)
	items, obj := DetectItems(raw, nil)
	if len(items) != 2 {
		t.Errorf("expected 2 auto-detected items, got %d (obj: %v)", len(items), obj)
	}
}

func TestDetectItems_MultipleObjectArrays_NoAutoDetect(t *testing.T) {
	// Two non-empty arrays of objects → ambiguous, fall back to object mode.
	// (Arrays of primitives like strings are ignored by toObjSlice.)
	raw := []byte(`{"items":[{"id":1}],"related":[{"id":2}]}`)
	items, obj := DetectItems(raw, nil)
	if items != nil {
		t.Errorf("expected nil items when two object arrays present, got %v", items)
	}
	if obj == nil {
		t.Error("expected obj when auto-detect is ambiguous")
	}
}

func TestDetectItems_StringArray_NotCountedAsItems(t *testing.T) {
	// Array of strings does not count toward auto-detection (not object slices).
	// One object-array + one string-array → still auto-detected as list.
	raw := []byte(`{"items":[{"id":1}],"tags":["x","y"]}`)
	items, _ := DetectItems(raw, nil)
	if len(items) != 1 {
		t.Errorf("expected 1 auto-detected item, got %d", len(items))
	}
}

func TestDetectItems_RootField_Override(t *testing.T) {
	raw := []byte(`{"data":[{"id":1},{"id":2}],"meta":{}}`)
	cfg := &ViewConfig{RootField: "data"}
	items, _ := DetectItems(raw, cfg)
	if len(items) != 2 {
		t.Errorf("expected 2 items from root_field, got %d", len(items))
	}
}

func TestDetectItems_ViewConfig_DisablesAutoDetect(t *testing.T) {
	// Single-object response with one array field — with ViewConfig, must stay as object.
	raw := []byte(`{"id":1,"labels":[{"name":"bug"}]}`)
	cfg := &ViewConfig{} // cfg non-nil disables auto-detect
	items, obj := DetectItems(raw, cfg)
	if items != nil {
		t.Errorf("ViewConfig should disable auto-detect, but got items: %v", items)
	}
	if obj == nil {
		t.Error("expected obj for single-object response with ViewConfig")
	}
}

func TestDetectItems_InvalidJSON_ReturnsNil(t *testing.T) {
	items, obj := DetectItems([]byte(`not json`), nil)
	if items != nil || obj != nil {
		t.Errorf("expected nil,nil for invalid JSON, got %v, %v", items, obj)
	}
}

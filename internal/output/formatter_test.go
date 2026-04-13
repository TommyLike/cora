package output

import (
	"testing"
)

// --- extractItems ---

func TestExtractItems_topLevelArray(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"id": 1.0, "name": "Alice"},
		map[string]interface{}{"id": 2.0, "name": "Bob"},
	}
	items := extractItems(input)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", items[0]["name"])
	}
}

func TestExtractItems_envelopedArray(t *testing.T) {
	input := map[string]interface{}{
		"meta":  map[string]interface{}{"total": 2},
		"posts": []interface{}{map[string]interface{}{"id": 1.0}},
	}
	items := extractItems(input)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestExtractItems_skipsMetaAndPaginationKeys(t *testing.T) {
	input := map[string]interface{}{
		"meta":       []interface{}{map[string]interface{}{"k": "v"}},
		"pagination": []interface{}{map[string]interface{}{"p": "q"}},
		"data":       []interface{}{map[string]interface{}{"id": 1.0}},
	}
	items := extractItems(input)
	if len(items) != 1 {
		t.Fatalf("expected only the 'data' array, got %d items", len(items))
	}
	if items[0]["id"] != 1.0 {
		t.Errorf("unexpected item: %v", items[0])
	}
}

func TestExtractItems_skipsUnderscorePrefixKeys(t *testing.T) {
	input := map[string]interface{}{
		"_links": []interface{}{map[string]interface{}{"href": "x"}},
		"items":  []interface{}{map[string]interface{}{"id": 1.0}},
	}
	items := extractItems(input)
	if len(items) != 1 || items[0]["id"] != 1.0 {
		t.Errorf("expected items array, got %v", items)
	}
}

func TestExtractItems_singleObject_returnsNil(t *testing.T) {
	input := map[string]interface{}{"id": 1.0, "title": "hello"}
	items := extractItems(input)
	if items != nil {
		t.Errorf("expected nil for single object, got %v", items)
	}
}

func TestExtractItems_emptyArray_returnsNil(t *testing.T) {
	input := map[string]interface{}{
		"posts": []interface{}{},
	}
	items := extractItems(input)
	if items != nil {
		t.Errorf("expected nil for empty array, got %v", items)
	}
}

// --- sanitize ---

func TestSanitize_removesControlCharacters(t *testing.T) {
	input := "hello\x01\x1bworld"
	got := sanitize(input)
	want := "helloworld"
	if got != want {
		t.Errorf("sanitize(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitize_keepsTabAndNewline(t *testing.T) {
	input := "line1\nline2\ttab"
	got := sanitize(input)
	if got != input {
		t.Errorf("sanitize should preserve \\t and \\n, got %q", got)
	}
}

func TestSanitize_plainString_unchanged(t *testing.T) {
	input := "hello world 123 !@#"
	if got := sanitize(input); got != input {
		t.Errorf("expected unchanged, got %q", got)
	}
}

// --- stringify ---

func TestStringify_nilReturnsEmpty(t *testing.T) {
	if got := stringify(nil, 100); got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
}

func TestStringify_truncatesLongString(t *testing.T) {
	s := "abcdefghij"
	got := stringify(s, 5)
	if got != "abcde…" {
		t.Errorf("expected truncation, got %q", got)
	}
}

func TestStringify_shortString_unchanged(t *testing.T) {
	s := "short"
	got := stringify(s, 100)
	if got != "short" {
		t.Errorf("expected %q, got %q", s, got)
	}
}

// --- Print (smoke tests) ---

func TestPrint_validJSON_noError(t *testing.T) {
	data := []byte(`{"id":1,"title":"hello"}`)
	if err := Print(data, "json"); err != nil {
		t.Errorf("Print json: %v", err)
	}
}

func TestPrint_invalidJSON_noError(t *testing.T) {
	// Non-JSON content should be printed raw without returning an error.
	data := []byte(`not json at all`)
	if err := Print(data, "json"); err != nil {
		t.Errorf("Print with invalid JSON should not return error, got: %v", err)
	}
}

func TestPrint_tableFormat_noError(t *testing.T) {
	data := []byte(`[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`)
	if err := Print(data, "table"); err != nil {
		t.Errorf("Print table: %v", err)
	}
}

func TestPrint_unknownFormat_fallsBackToJSON(t *testing.T) {
	data := []byte(`{"key":"val"}`)
	if err := Print(data, "yaml"); err != nil {
		t.Errorf("Print unknown format: %v", err)
	}
}

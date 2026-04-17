package log

import (
	"net/http"
	"testing"
)

// ── MaskURL ──────────────────────────────────────────────────────────────────

func TestMaskURL_AccessToken(t *testing.T) {
	in := "https://api.gitcode.com/api/v5/repos/o/r/issues?access_token=secret123&state=open"
	got := MaskURL(in)
	if contains(got, "secret123") {
		t.Errorf("MaskURL: access_token value should be redacted; got %q", got)
	}
	if !contains(got, "access_token=***") {
		t.Errorf("MaskURL: access_token key should be preserved with *** value; got %q", got)
	}
	if !contains(got, "state=open") {
		t.Errorf("MaskURL: non-sensitive params should be unchanged; got %q", got)
	}
}

func TestMaskURL_Apikey(t *testing.T) {
	in := "https://etherpad.example.org/api/1.3.0/createPad?apikey=topsecret&padID=foo"
	got := MaskURL(in)
	if contains(got, "topsecret") {
		t.Errorf("MaskURL should redact apikey value; got %q", got)
	}
	if !contains(got, "apikey=***") {
		t.Errorf("MaskURL should keep apikey key; got %q", got)
	}
}

func TestMaskURL_MultipleParams(t *testing.T) {
	in := "https://example.com/api?access_token=secretA&api_key=secretB&token=secretC&normal=ok"
	got := MaskURL(in)
	for _, v := range []string{"secretA", "secretB", "secretC"} {
		if contains(got, v) {
			t.Errorf("MaskURL: value %q should be redacted; got %q", v, got)
		}
	}
	if !contains(got, "normal=ok") {
		t.Errorf("MaskURL: non-sensitive param should be unchanged; got %q", got)
	}
}

func TestMaskURL_NoSensitiveParams(t *testing.T) {
	in := "https://example.com/api?state=open&page=1"
	got := MaskURL(in)
	if got != in {
		t.Errorf("MaskURL: no sensitive params, URL should be unchanged\ngot  %q\nwant %q", got, in)
	}
}

func TestMaskURL_InvalidURL(t *testing.T) {
	in := "://not a url"
	got := MaskURL(in)
	if got != in {
		t.Errorf("MaskURL: invalid URL should be returned as-is\ngot  %q\nwant %q", got, in)
	}
}

func TestMaskURL_EmptyURL(t *testing.T) {
	got := MaskURL("")
	if got != "" {
		t.Errorf("MaskURL(\"\") = %q, want \"\"", got)
	}
}

// ── MaskHeader ───────────────────────────────────────────────────────────────

func TestMaskHeader_ApiKey(t *testing.T) {
	h := http.Header{
		"Api-Key":      {"mysecretkey"},
		"Content-Type": {"application/json"},
	}
	got := MaskHeader(h)
	if got.Get("Api-Key") != "***" {
		t.Errorf("MaskHeader: Api-Key should be redacted; got %q", got.Get("Api-Key"))
	}
	if got.Get("Content-Type") != "application/json" {
		t.Errorf("MaskHeader: Content-Type should be unchanged; got %q", got.Get("Content-Type"))
	}
	// Ensure original is not modified.
	if h.Get("Api-Key") != "mysecretkey" {
		t.Errorf("MaskHeader: original header must not be modified")
	}
}

func TestMaskHeader_Authorization(t *testing.T) {
	h := http.Header{"Authorization": {"Bearer token123"}}
	got := MaskHeader(h)
	if got.Get("Authorization") != "***" {
		t.Errorf("MaskHeader: Authorization should be redacted; got %q", got.Get("Authorization"))
	}
}

func TestMaskHeader_NoSensitiveFields(t *testing.T) {
	h := http.Header{
		"Content-Type": {"application/json"},
		"Accept":       {"*/*"},
	}
	got := MaskHeader(h)
	if got.Get("Content-Type") != "application/json" || got.Get("Accept") != "*/*" {
		t.Errorf("MaskHeader: non-sensitive headers should be unchanged; got %v", got)
	}
}

// ── FormatBody ───────────────────────────────────────────────────────────────

func TestFormatBody_ShortJSON(t *testing.T) {
	body := []byte(`{"id":1,"title":"hello"}`)
	got := FormatBody(body, 2048)
	// Should be pretty-printed (contains a newline).
	if !contains(got, "\n") {
		t.Errorf("FormatBody: valid JSON should be pretty-printed; got %q", got)
	}
	if !contains(got, `"id": 1`) {
		t.Errorf("FormatBody: pretty output should contain fields; got %q", got)
	}
}

func TestFormatBody_ShortNonJSON(t *testing.T) {
	body := []byte("plain text response")
	got := FormatBody(body, 2048)
	if got != string(body) {
		t.Errorf("FormatBody: non-JSON short body should be returned as-is; got %q", got)
	}
}

func TestFormatBody_Exact(t *testing.T) {
	// Exactly at the limit, non-JSON — returned unchanged.
	body := make([]byte, 2048)
	for i := range body {
		body[i] = 'x'
	}
	got := FormatBody(body, 2048)
	if got != string(body) {
		t.Errorf("FormatBody: body exactly at limit should be returned unchanged")
	}
}

func TestFormatBody_Long(t *testing.T) {
	body := make([]byte, 3000)
	for i := range body {
		body[i] = 'a'
	}
	got := FormatBody(body, 2048)
	if !contains(got, "[truncated, total: 3000 bytes]") {
		t.Errorf("FormatBody: long body should include truncation notice; got %q…", got[:100])
	}
	if !contains(got[:2048], "aaaa") {
		t.Errorf("FormatBody: first 2048 bytes should be preserved")
	}
}

func TestFormatBody_Empty(t *testing.T) {
	got := FormatBody([]byte{}, 2048)
	if got != "" {
		t.Errorf("FormatBody: empty body should return empty string, got %q", got)
	}
}

func TestFormatBody_LongJSON(t *testing.T) {
	// A JSON body that exceeds the limit should be truncated, not pretty-printed.
	big := make([]byte, 3000)
	for i := range big {
		big[i] = 'a'
	}
	// Wrap in a JSON string literal — still > 2048 bytes.
	body := append([]byte(`"`), append(big, '"')...)
	got := FormatBody(body, 2048)
	if !contains(got, "[truncated") {
		t.Errorf("FormatBody: oversized JSON should be truncated, not pretty-printed; got %q…", got[:80])
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

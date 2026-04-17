package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// sensitiveQueryParams is the fixed set of URL query parameter names whose
// values must be redacted in log output. This list is intentionally closed —
// do not expand it dynamically.
var sensitiveQueryParams = map[string]bool{
	"access_token": true,
	"apikey":       true,
	"api_key":      true,
	"token":        true,
	"secret":       true,
	"password":     true,
	"key":          true,
}

// sensitiveHeaders is the fixed set of HTTP header names whose values must be
// redacted in log output.
var sensitiveHeaders = map[string]bool{
	"api-key":       true,
	"api-username":  true,
	"authorization": true,
}

// MaskURL replaces the values of sensitive query parameters in rawURL with "***".
// The parameter names are preserved so the log entry remains readable.
// If rawURL cannot be parsed it is returned unchanged (no panic).
func MaskURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	q := u.Query()
	masked := false
	for k := range q {
		if sensitiveQueryParams[strings.ToLower(k)] {
			q[k] = []string{"***"}
			masked = true
		}
	}
	if !masked {
		return rawURL
	}

	// Build the raw query manually to avoid url.Values.Encode() percent-encoding "***".
	var parts []string
	for k, vs := range q {
		for _, v := range vs {
			if v == "***" {
				parts = append(parts, url.QueryEscape(k)+"=***")
			} else {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
	}
	u.RawQuery = strings.Join(parts, "&")
	return u.String()
}

// MaskHeader returns a shallow copy of h with sensitive header values replaced
// by "***". The original h is never modified.
func MaskHeader(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, vs := range h {
		if sensitiveHeaders[strings.ToLower(k)] {
			out[k] = []string{"***"}
		} else {
			out[k] = vs
		}
	}
	return out
}

// FormatBody formats body for log output:
//   - If len(body) > maxBytes: truncate to maxBytes and append a size notice (plain text).
//   - If len(body) ≤ maxBytes and body is valid JSON: pretty-print with 2-space indent.
//   - Otherwise: return as plain text unchanged.
func FormatBody(body []byte, maxBytes int) string {
	if len(body) > maxBytes {
		return fmt.Sprintf("%s... [truncated, total: %d bytes]", body[:maxBytes], len(body))
	}
	var buf bytes.Buffer
	if json.Indent(&buf, body, "", "  ") == nil {
		return buf.String()
	}
	return string(body)
}

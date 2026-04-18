package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/internal/view"
)

func svcConfig(serverURL string) *config.Config {
	return &config.Config{
		Services: map[string]config.ServiceConfig{
			"svc": {BaseURL: serverURL},
		},
	}
}

// ── Execute: success paths ────────────────────────────────────────────────────

func TestExecute_JSONResponse_TableFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"title":"hello"}`))
	}))
	defer srv.Close()

	ex := New(svcConfig(srv.URL))
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodGet,
		PathTemplate: "/issue",
		Format:       "json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecute_EmptyBody_204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ex := New(svcConfig(srv.URL))
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodDelete,
		PathTemplate: "/issue/1",
	})
	if err != nil {
		t.Fatalf("204 No Content should not return error, got: %v", err)
	}
}

func TestExecute_4xxReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	ex := New(svcConfig(srv.URL))
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodGet,
		PathTemplate: "/missing",
	})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

func TestExecute_5xxReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server exploded"}`))
	}))
	defer srv.Close()

	ex := New(svcConfig(srv.URL))
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodGet,
		PathTemplate: "/broken",
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got: %v", err)
	}
}

// ── Execute: path params ──────────────────────────────────────────────────────

func TestExecute_PathParamSubstituted(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ex := New(svcConfig(srv.URL))
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodGet,
		PathTemplate: "/issues/{number}",
		PathParams:   map[string]string{"number": "42"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/issues/42" {
		t.Errorf("path = %q, want %q", gotPath, "/issues/42")
	}
}

func TestExecute_QueryParamAppended(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ex := New(svcConfig(srv.URL))
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodGet,
		PathTemplate: "/issues",
		QueryParams:  map[string]string{"state": "open"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotQuery, "state=open") {
		t.Errorf("query = %q, expected state=open", gotQuery)
	}
}

// ── Execute: dry-run ──────────────────────────────────────────────────────────

func TestExecute_DryRun_NoHTTPCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ex := New(svcConfig(srv.URL))
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodGet,
		PathTemplate: "/issues",
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("dry-run should not error: %v", err)
	}
	if called {
		t.Error("dry-run must not send an actual HTTP request")
	}
}

// ── Execute: request body ─────────────────────────────────────────────────────

func TestExecute_RequestBody_SentAsJSON(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ex := New(svcConfig(srv.URL))
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodPost,
		PathTemplate: "/issues",
		Body:         map[string]interface{}{"title": "bug"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, "title") {
		t.Errorf("request body = %q, expected 'title' field", gotBody)
	}
}

// ── Execute: view config ──────────────────────────────────────────────────────

func TestExecute_WithViewConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"title":"test issue","state":"open"}`))
	}))
	defer srv.Close()

	ex := New(svcConfig(srv.URL))
	vc := &view.ViewConfig{
		Columns: []view.ViewColumn{
			{Field: "id", Label: "ID"},
			{Field: "title", Label: "Title"},
		},
	}
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodGet,
		PathTemplate: "/issues/1",
		Format:       "table",
		ViewConfig:   vc,
	})
	if err != nil {
		t.Fatalf("unexpected error with view config: %v", err)
	}
}

// ── truncate ─────────────────────────────────────────────────────────────────

func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello…"},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc…"},
	}
	for _, tc := range cases {
		got := truncate(tc.s, tc.n)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
		}
	}
}

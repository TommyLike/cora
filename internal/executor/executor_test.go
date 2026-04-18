package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/pkg/errs"
)

// cliErr extracts a *errs.CLIError from err; fails the test if not found.
func asCLIError(t *testing.T, err error) *errs.CLIError {
	t.Helper()
	var cliErr *errs.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *errs.CLIError, got %T: %v", err, err)
	}
	return cliErr
}

// newTestExecutor builds an Executor wired to a test HTTP server.
func newTestExecutor(t *testing.T, serverURL string) *Executor {
	t.Helper()
	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"svc": {BaseURL: serverURL},
		},
	}
	return New(cfg)
}

// --- isTransientNetworkError ---

func TestIsTransientNetworkError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "context cancelled not transient",
			err:  &url.Error{Op: "Get", URL: "http://x", Err: context.Canceled},
			want: false,
		},
		{
			name: "context deadline not transient",
			err:  &url.Error{Op: "Get", URL: "http://x", Err: context.DeadlineExceeded},
			want: false,
		},
		{
			name: "EOF is transient",
			err:  &url.Error{Op: "Get", URL: "http://x", Err: io.EOF},
			want: true,
		},
		{
			name: "unexpected EOF is transient",
			err:  &url.Error{Op: "Get", URL: "http://x", Err: io.ErrUnexpectedEOF},
			want: true,
		},
		{
			name: "connection reset is transient",
			err:  &url.Error{Op: "Get", URL: "http://x", Err: fmt.Errorf("connection reset by peer")},
			want: true,
		},
		{
			name: "plain error not transient",
			err:  fmt.Errorf("some random error"),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isTransientNetworkError(tc.err)
			if got != tc.want {
				t.Errorf("isTransientNetworkError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// --- classifyError ---

func TestClassifyError(t *testing.T) {
	t.Run("context deadline becomes timeout message", func(t *testing.T) {
		err := classifyError(&url.Error{Op: "Get", URL: "http://x", Err: context.DeadlineExceeded})
		cliErr := asCLIError(t, err)
		if !strings.Contains(cliErr.Msg, "timed out") {
			t.Errorf("expected timeout message, got %q", cliErr.Msg)
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		err := classifyError(context.Canceled)
		cliErr := asCLIError(t, err)
		if !strings.Contains(cliErr.Msg, "cancel") {
			t.Errorf("expected cancel message, got %q", cliErr.Msg)
		}
	})

	t.Run("url error with masked credentials", func(t *testing.T) {
		inner := &url.Error{Op: "Get", URL: "http://host?access_token=SECRET", Err: io.EOF}
		err := classifyError(inner)
		cliErr := asCLIError(t, err)
		if strings.Contains(cliErr.Msg, "SECRET") {
			t.Errorf("classified error leaks credential: %q", cliErr.Msg)
		}
	})
}

// --- HTTP client timeout ---

func TestDefaultTimeout(t *testing.T) {
	ex := New(&config.Config{Services: map[string]config.ServiceConfig{}})
	if ex.client.Timeout != defaultTimeout {
		t.Errorf("http.Client.Timeout = %v, want %v", ex.client.Timeout, defaultTimeout)
	}
}

// --- doWithRetry ---

func TestDoWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	ex := newTestExecutor(t, srv.URL)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	resp, body, err := ex.doWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q, want {\"ok\":true}", body)
	}
}

func TestDoWithRetry_RetriesOnEOF(t *testing.T) {
	var attempts atomic.Int32

	// First two connections: close immediately (simulates EOF).
	// Third connection: returns 200.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			// Hijack and close without sending a response → client sees EOF.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Errorf("server does not support hijacking")
				return
			}
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"ok"`))
	}))
	defer srv.Close()

	ex := newTestExecutor(t, srv.URL)
	// Use a short retry delay so the test doesn't take seconds.
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/retry", nil)
	resp, _, err := ex.doWithRetry(req)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestDoWithRetry_NoRetryOnPOST(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	ex := newTestExecutor(t, srv.URL)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/no-retry", strings.NewReader(`{}`))
	_, _, err := ex.doWithRetry(req)
	if err == nil {
		t.Fatal("expected error for closed connection")
	}
	// POST must NOT be retried — exactly 1 attempt.
	if attempts.Load() != 1 {
		t.Errorf("attempts = %d, want 1 (POST must not be retried)", attempts.Load())
	}
}

func TestDoWithRetry_ContextCancelledMidRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Cancel the context before any successful response.
		cancel()
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	ex := newTestExecutor(t, srv.URL)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/cancel", nil)
	_, _, err := ex.doWithRetry(req)
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}

// --- Execute integration (happy path) ---

func TestExecute_ConfigError_MissingService(t *testing.T) {
	ex := New(&config.Config{Services: map[string]config.ServiceConfig{}})
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "unknown",
		Method:       http.MethodGet,
		PathTemplate: "/x",
	})
	if err == nil {
		t.Fatal("expected config error for unknown service")
	}
	asCLIError(t, err)
}

func TestExecute_ConfigError_MissingBaseURL(t *testing.T) {
	ex := New(&config.Config{
		Services: map[string]config.ServiceConfig{
			"svc": {BaseURL: ""},
		},
	})
	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodGet,
		PathTemplate: "/x",
	})
	if err == nil {
		t.Fatal("expected config error for empty base_url")
	}
}

func TestExecute_ClientTimeout(t *testing.T) {
	// Server that sleeps longer than the client timeout.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ex := newTestExecutor(t, srv.URL)
	// Override to a very short timeout to keep the test fast.
	ex.client.Timeout = 100 * time.Millisecond

	err := ex.Execute(context.Background(), &Request{
		ServiceName:  "svc",
		Method:       http.MethodGet,
		PathTemplate: "/slow",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	cliErr := asCLIError(t, err)
	if !strings.Contains(cliErr.Msg, "timed out") {
		t.Errorf("expected timeout message, got %q", cliErr.Msg)
	}
}

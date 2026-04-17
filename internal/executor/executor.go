package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cncf/cora/internal/auth"
	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/internal/log"
	"github.com/cncf/cora/internal/output"
	"github.com/cncf/cora/pkg/errs"
)

// Request is the input to a single HTTP API call.
type Request struct {
	ServiceName  string
	PathTemplate string            // e.g. "/posts/{id}.json"
	Method       string            // "GET", "POST", …
	PathParams   map[string]string // {id} → "123"
	QueryParams  map[string]string
	Body         map[string]interface{}
	Format       string // "table" | "json"
	DryRun       bool
}

// Executor executes API requests against configured backend services.
type Executor struct {
	cfg    *config.Config
	client *http.Client
}

// New creates an Executor backed by the given config.
func New(cfg *config.Config) *Executor {
	return &Executor{cfg: cfg, client: &http.Client{}}
}

// Execute performs the HTTP request described by req, formats the response,
// and writes it to stdout.  Errors are returned as CLIErrors.
func (e *Executor) Execute(ctx context.Context, req *Request) error {
	svcCfg, ok := e.cfg.Services[req.ServiceName]
	if !ok {
		return errs.NewConfigError(fmt.Sprintf("service %q not found in config", req.ServiceName))
	}

	baseURL := strings.TrimRight(svcCfg.BaseURL, "/")
	if baseURL == "" {
		return errs.NewConfigError(fmt.Sprintf("service %q: base_url is not set", req.ServiceName))
	}

	// Substitute path parameters: /posts/{id}.json → /posts/123.json
	path := req.PathTemplate
	for k, v := range req.PathParams {
		path = strings.ReplaceAll(path, "{"+k+"}", url.PathEscape(v))
	}

	// Build full URL with query string
	fullURL := baseURL + path
	if len(req.QueryParams) > 0 {
		q := url.Values{}
		for k, v := range req.QueryParams {
			q.Set(k, v)
		}
		fullURL += "?" + q.Encode()
	}

	// Serialise request body
	var bodyReader io.Reader
	if len(req.Body) > 0 {
		b, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	// Build HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if bodyReader != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Accept", "application/json")

	// Inject auth credentials (Discourse: headers; Etherpad: ?apikey= query param).
	// Done before dry-run output so the printed URL reflects the actual request.
	auth.InjectAuth(httpReq, svcCfg, req.ServiceName)

	// Log the outgoing request (after auth injection so the masked URL is accurate).
	bodySize := 0
	if len(req.Body) > 0 {
		if b, err2 := json.Marshal(req.Body); err2 == nil {
			bodySize = len(b)
		}
	}
	log.Debug("→ %s %s  [body: %d bytes]", req.Method, log.MaskURL(httpReq.URL.String()), bodySize)

	// --dry-run: print what would be sent and exit
	if req.DryRun {
		fmt.Printf("[dry-run] %s %s\n", req.Method, httpReq.URL.String())
		if len(req.Body) > 0 {
			pretty, _ := json.MarshalIndent(req.Body, "", "  ")
			fmt.Printf("Body:\n%s\n", pretty)
		}
		return nil
	}

	// Execute
	start := time.Now()
	resp, err := e.client.Do(httpReq)
	if err != nil {
		return errs.NewAPIError("request failed", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	elapsed := time.Since(start)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	log.Debug("← %s (%d bytes, %dms)", resp.Status, len(respBytes), elapsed.Milliseconds())
	log.Debug("response body: %s", log.FormatBody(respBytes, 3072))

	// Treat 4xx/5xx as API errors
	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("API error %d", resp.StatusCode)
		if len(respBytes) > 0 {
			msg += ": " + truncate(string(respBytes), 300)
		}
		return errs.NewAPIError(msg, nil)
	}

	// Empty body (e.g. 204 No Content)
	if len(respBytes) == 0 {
		fmt.Println("OK")
		return nil
	}

	format := req.Format
	if format == "" {
		format = "table"
	}
	return output.Print(respBytes, format)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

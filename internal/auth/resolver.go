package auth

import (
	"net/http"

	"github.com/cncf/cora/internal/config"
)

// InjectAuth adds authentication credentials to an outgoing request based on
// the service's configured auth provider.
//
// Discourse: injects Api-Key and Api-Username headers.
// Etherpad:  injects ?apikey= into the request URL's query string.
// GitCode:   injects ?access_token= into the request URL's query string.
//
// All providers inject credentials unconditionally when present; the server
// ignores them for public endpoints and enforces them for protected ones.
func InjectAuth(req *http.Request, svc config.ServiceConfig) {
	if d := svc.Auth.Discourse; d != nil {
		if d.APIKey != "" {
			req.Header.Set("Api-Key", d.APIKey)
		}
		if d.APIUsername != "" {
			req.Header.Set("Api-Username", d.APIUsername)
		}
	}

	if e := svc.Auth.Etherpad; e != nil && e.APIKey != "" {
		q := req.URL.Query()
		q.Set("apikey", e.APIKey)
		req.URL.RawQuery = q.Encode()
	}

	if g := svc.Auth.Gitcode; g != nil && g.AccessToken != "" {
		q := req.URL.Query()
		q.Set("access_token", g.AccessToken)
		req.URL.RawQuery = q.Encode()
	}
}

// IsDiscourseAuthParam reports whether an OpenAPI parameter is one of the
// Discourse auth headers that should be injected automatically (not exposed
// to the user as a CLI flag).
func IsDiscourseAuthParam(name string) bool {
	return name == "Api-Key" || name == "Api-Username"
}

// IsGitcodeAuthParam reports whether an OpenAPI parameter is a GitCode auth
// parameter that should be injected automatically (not exposed as a CLI flag).
// GitCode uses ?access_token= (PAT) and Authorization header (OAuth Bearer).
func IsGitcodeAuthParam(name string) bool {
	return name == "access_token" || name == "Authorization"
}

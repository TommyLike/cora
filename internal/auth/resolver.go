package auth

import (
	"net/http"

	"github.com/cncf/cora/internal/config"
)

// InjectHeaders adds authentication headers to an outgoing request based on
// the service's configured auth provider.
//
// Discourse style: injects Api-Key and Api-Username headers unconditionally
// when credentials are present. The server ignores them for public endpoints
// and enforces them for protected ones.
func InjectHeaders(req *http.Request, svc config.ServiceConfig) {
	if d := svc.Auth.Discourse; d != nil {
		if d.APIKey != "" {
			req.Header.Set("Api-Key", d.APIKey)
		}
		if d.APIUsername != "" {
			req.Header.Set("Api-Username", d.APIUsername)
		}
	}
}

// IsDiscourseAuthParam reports whether an OpenAPI parameter is one of the
// Discourse auth headers that should be injected automatically (not exposed
// to the user as a CLI flag).
func IsDiscourseAuthParam(name string) bool {
	return name == "Api-Key" || name == "Api-Username"
}

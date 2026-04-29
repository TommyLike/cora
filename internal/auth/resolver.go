package auth

import (
	"net/http"

	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/internal/log"
)

// InjectAuth adds authentication credentials to an outgoing request based on
// the service's configured auth provider. svcName is used only for log output.
//
// Discourse: injects Api-Key and Api-Username headers.
// Etherpad:  injects ?apikey= into the request URL's query string.
// GitCode:   injects ?access_token= into the request URL's query string.
// GitHub:    injects Authorization: Bearer <token> header.
//
// All providers inject credentials unconditionally when present; the server
// ignores them for public endpoints and enforces them for protected ones.
func InjectAuth(req *http.Request, svc config.ServiceConfig, svcName string) {
	if d := svc.Auth.Discourse; d != nil {
		if d.APIKey != "" {
			req.Header.Set("Api-Key", d.APIKey)
		}
		if d.APIUsername != "" {
			req.Header.Set("Api-Username", d.APIUsername)
		}
		log.Debug("auth: injecting discourse headers for service %q", svcName)
	}

	if e := svc.Auth.Etherpad; e != nil && e.APIKey != "" {
		q := req.URL.Query()
		q.Set("apikey", e.APIKey)
		req.URL.RawQuery = q.Encode()
		log.Debug("auth: injecting etherpad apikey for service %q", svcName)
	}

	if g := svc.Auth.Gitcode; g != nil && g.AccessToken != "" {
		q := req.URL.Query()
		q.Set("access_token", g.AccessToken)
		req.URL.RawQuery = q.Encode()
		log.Debug("auth: injecting gitcode access_token for service %q", svcName)
	}

	if h := svc.Auth.Github; h != nil && h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
		// GitHub recommends sending an explicit API version header; this also
		// pins responses to a stable shape regardless of server-side rollouts.
		if req.Header.Get("X-GitHub-Api-Version") == "" {
			req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		}
		log.Debug("auth: injecting github bearer token for service %q", svcName)
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

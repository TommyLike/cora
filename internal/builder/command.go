// Package builder converts an OpenAPI 3 spec into a Cobra command tree.
//
// Mapping rules:
//   - Resource  = first operation tag, normalised to lowercase kebab-case.
//   - Verb      = known prefix from operationId (list/get/create/update/delete/patch),
//     falling back to HTTP-method inference.
//   - URL .json suffix is stripped for command names but kept in HTTP requests.
//   - Discourse auth header params (Api-Key, Api-Username) are silently skipped
//     from flag generation; they are injected by the executor.
package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cobra"

	"github.com/cncf/cora/internal/auth"
	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/internal/executor"
	"github.com/cncf/cora/internal/view"
)

// Build returns a *cobra.Command for the given service, populated with one
// sub-command per resource and one leaf command per operation in the spec.
func Build(
	svcName string,
	spec *openapi3.T,
	cfg *config.Config,
	exec *executor.Executor,
	viewReg *view.Registry,
) *cobra.Command {
	title := ""
	if spec.Info != nil {
		title = spec.Info.Title
	}
	svcCmd := &cobra.Command{
		Use:   svcName,
		Short: title,
	}

	// --- Group operations by resource ---
	type opEntry struct {
		path   string
		method string
		op     *openapi3.Operation
	}
	resources := map[string][]opEntry{}

	if spec.Paths != nil {
		for path, pathItem := range spec.Paths.Map() {
			if pathItem == nil {
				continue
			}
			ops := pathItem.Operations()
			// When a path exposes both GET and POST with the same operationId
			// base (e.g. Etherpad's "Using{METHOD}" pattern), keep only GET to
			// avoid duplicate commands. We detect this by checking whether all
			// HTTP methods on the path share the same stripped operationId.
			if shouldDeduplicateMethods(ops) {
				if getOp, ok := ops[http.MethodGet]; ok && getOp != nil {
					res := resourceName(getOp, path)
					resources[res] = append(resources[res], opEntry{path, http.MethodGet, getOp})
					continue
				}
			}
			for method, op := range ops {
				if op == nil {
					continue
				}
				res := resourceName(op, path)
				resources[res] = append(resources[res], opEntry{path, method, op})
			}
		}
	}

	// --- Build one sub-command per resource ---
	for res, ops := range resources {
		res := res
		resCmd := &cobra.Command{
			Use:   res,
			Short: fmt.Sprintf("Manage %s", res),
		}

		// Sort operations to establish a stable priority for verb assignment:
		//   1. "repo" context first (primary entity type for most APIs).
		//   2. Within the same context, shallower paths first — a shorter path
		//      means a more fundamental operation (e.g. /issues/{n} beats
		//      /issues/comments/{id} for the "get" verb).
		//   3. Alphabetical by path as a final tiebreaker.
		// This ensures the primary get/list operations receive the clean verb,
		// and sub-resource operations receive a disambiguated suffix.
		sort.Slice(ops, func(i, j int) bool {
			ci := pathContext(ops[i].path)
			cj := pathContext(ops[j].path)
			if ci != cj {
				if ci == "repo" {
					return true
				}
				if cj == "repo" {
					return false
				}
				return ci < cj
			}
			// Same context: prefer shallower paths.
			di := strings.Count(ops[i].path, "/")
			dj := strings.Count(ops[j].path, "/")
			if di != dj {
				return di < dj
			}
			return ops[i].path < ops[j].path
		})

		// Track verbs to detect conflicts within the same resource.
		verbSeen := map[string]bool{}

		for _, e := range ops {
			verb := verbName(e.op.OperationID, e.method, e.path)
			if verbSeen[verb] {
				// First try: qualify with path context (e.g. "enterprise", "user").
				ctx := pathContext(e.path)
				if ctx != "" && ctx != "repo" {
					verb = verb + "-" + ctx
				} else {
					verb = verb + "-" + pathSuffix(e.path)
				}
			}
			// Second try: if still conflicts, append path suffix on top.
			if verbSeen[verb] {
				verb = verb + "-" + pathSuffix(e.path)
			}
			verbSeen[verb] = true

			verbCmd := buildLeaf(svcName, res, e.path, e.method, verb, e.op, exec, viewReg)
			resCmd.AddCommand(verbCmd)
		}

		svcCmd.AddCommand(resCmd)
	}

	return svcCmd
}

// ─── Name derivation ─────────────────────────────────────────────────────────

// resourceName returns the CLI resource name for an operation.
// Uses the first tag (e.g. "Posts" → "posts"); falls back to path analysis.
func resourceName(op *openapi3.Operation, path string) string {
	if len(op.Tags) > 0 {
		return strings.ToLower(strings.ReplaceAll(op.Tags[0], " ", "-"))
	}
	return lastPathSegment(path)
}

// lastPathSegment returns the last non-parameter path segment, without .json.
// "/posts/{id}/replies.json" → "replies"
func lastPathSegment(path string) string {
	clean := strings.TrimSuffix(path, ".json")
	segs := strings.Split(strings.Trim(clean, "/"), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if s := segs[i]; s != "" && !strings.HasPrefix(s, "{") {
			return strings.ToLower(s)
		}
	}
	return "root"
}

// pathSuffix returns a short disambiguating label from the path.
// "/posts/{id}/replies.json" → "replies"
func pathSuffix(path string) string {
	return lastPathSegment(path)
}

var knownVerbPrefixes = []string{
	"list", "get", "create", "update", "patch", "delete",
}

// httpMethodSuffixes are the Java/Spring-Boot-style operationId suffixes that
// Etherpad's OpenAPI spec appends, e.g. "getTextUsingGET".
var httpMethodSuffixes = []string{
	"UsingGET", "UsingPOST", "UsingPUT", "UsingPATCH",
	"UsingDELETE", "UsingHEAD", "UsingOPTIONS",
}

// verbName determines the CLI verb for an operation.
//
// Priority:
//  1. operationId ending with "Using{METHOD}" (Etherpad/Spring-Boot style):
//     strip the suffix and convert the remainder to kebab-case.
//     e.g. "getTextUsingGET" → "get-text"
//  2. Known verb prefix in a plain camelCase operationId (REST style):
//     e.g. "listPosts" → "list"
//     Skipped for path-encoded operationIds (GitCode style: get_api_v5_…)
//     because those are better resolved by the HTTP-method fallback.
//  3. Action segment after a path param: /{id}/lock.json → "lock"
//  4. HTTP method + trailing-param check → "get" / "list" / "create" etc.
//     Uses trailing-param detection so /repos/{owner}/{repo}/issues (no
//     trailing param) yields "list", while /repos/{owner}/{repo}/issues/{n}
//     (trailing param) yields "get".
func verbName(opID, method, path string) string {
	// Priority 1: strip Java-style Using{METHOD} suffix and use full kebab name.
	for _, suffix := range httpMethodSuffixes {
		if strings.HasSuffix(opID, suffix) {
			base := opID[:len(opID)-len(suffix)]
			if base != "" {
				return camelToKebab(base)
			}
		}
	}

	// Priority 2: known verb prefix in plain operationId.
	// Skip for path-encoded operationIds (e.g. GitCode's get_api_v5_repos_…)
	// to avoid all such operations collapsing to the same verb.
	if !isPathEncodedOpID(opID) {
		lower := strings.ToLower(opID)
		for _, v := range knownVerbPrefixes {
			if strings.HasPrefix(lower, v) {
				return v
			}
		}
	}

	// Priority 3: action segment after a path param: /posts/{id}/lock.json → "lock"
	// Only applies to shallow paths (at most 2 non-param segments before the
	// {param}/action pair). Deep API paths like /api/v5/repos/{owner}/{repo}/issues
	// have resource names at the end, not actions, and must fall through to Priority 4.
	clean := strings.TrimSuffix(path, ".json")
	segs := strings.Split(strings.Trim(clean, "/"), "/")
	if len(segs) >= 2 {
		last := segs[len(segs)-1]
		secondLast := segs[len(segs)-2]
		if !strings.HasPrefix(last, "{") && strings.HasPrefix(secondLast, "{") {
			// Count non-param segments that appear before the {param} at secondLast.
			paramIdx := len(segs) - 2
			priorNonParam := 0
			for i := 0; i < paramIdx; i++ {
				if segs[i] != "" && !strings.HasPrefix(segs[i], "{") {
					priorNonParam++
				}
			}
			if priorNonParam <= 2 {
				return last // e.g. "replies", "lock"
			}
		}
	}

	// Priority 4: HTTP method fallback.
	// Check whether the LAST path segment is a template param to correctly
	// distinguish a collection (/issues → list) from a single-item fetch
	// (/issues/{number} → get), even when owner/repo params appear mid-path.
	switch strings.ToUpper(method) {
	case "GET":
		if hasTrailingParam(path) {
			return "get"
		}
		return "list"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	}
	return strings.ToLower(method)
}

// isPathEncodedOpID reports whether operationId encodes the full API path
// in the pattern used by GitCode: {method}_api_v{N}_{rest}
// e.g. "get_api_v5_repos_{owner}_{repo}_issues_{number}"
// These ids should bypass the known-verb-prefix heuristic (Priority 2).
func isPathEncodedOpID(opID string) bool {
	lower := strings.ToLower(opID)
	for _, m := range []string{"get_", "post_", "put_", "patch_", "delete_"} {
		if strings.HasPrefix(lower, m) {
			rest := lower[len(m):]
			// Must be followed by "api_v" + a digit, e.g. "api_v5_"
			if len(rest) > 6 && strings.HasPrefix(rest, "api_v") &&
				rest[5] >= '0' && rest[5] <= '9' {
				return true
			}
		}
	}
	return false
}

// hasTrailingParam reports whether the last non-empty path segment is a
// template parameter such as "{id}" or "{number}".
// Unlike strings.Contains(path, "{"), this correctly treats
// /repos/{owner}/{repo}/issues as a collection (no trailing param).
func hasTrailingParam(path string) bool {
	clean := strings.TrimSuffix(path, ".json")
	segs := strings.Split(strings.Trim(clean, "/"), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if segs[i] != "" {
			return strings.HasPrefix(segs[i], "{")
		}
	}
	return false
}

// pathContext returns a short label for the primary entity type in a path,
// used to disambiguate commands from the same tag but different API scopes.
//
//	/api/v5/repos/{owner}/{repo}/issues/{n}      → "repo"
//	/api/v5/enterprises/{enterprise}/issues/{n}  → "enterprise"
//	/api/v5/user/issues                          → "user"
//	/api/v5/orgs/{org}/issues                    → "org"
func pathContext(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for _, s := range segs {
		sl := strings.ToLower(s)
		if sl == "" || sl == "api" || strings.HasPrefix(sl, "{") {
			continue
		}
		// Skip version segments: "v5", "v1", "v3", …
		if len(sl) >= 2 && sl[0] == 'v' {
			rest := sl[1:]
			onlyDigitsOrDot := true
			for _, c := range rest {
				if (c < '0' || c > '9') && c != '.' {
					onlyDigitsOrDot = false
					break
				}
			}
			if onlyDigitsOrDot {
				continue
			}
		}
		// Strip trailing 's' to get singular form: "repos"→"repo", "enterprises"→"enterprise"
		if strings.HasSuffix(sl, "s") && len(sl) > 2 {
			return sl[:len(sl)-1]
		}
		return sl
	}
	return ""
}

// ─── Leaf command builder ─────────────────────────────────────────────────────

type paramRecord struct {
	specName string // original name in spec, e.g. "topic_id"
	flagName string // kebab CLI flag, e.g. "topic-id"
	location string // "path" | "query"
	valPtr   interface{}
}

type bodyRecord struct {
	specName string
	valPtr   interface{}
}

// buildLeaf creates the innermost cobra.Command for a single API operation.
func buildLeaf(
	svcName, resource, pathTpl, method, verb string,
	op *openapi3.Operation,
	exec *executor.Executor,
	viewReg *view.Registry,
) *cobra.Command {
	desc := op.Summary
	if op.Description != "" {
		desc = op.Description
	}

	cmd := &cobra.Command{
		Use:   verb,
		Short: op.Summary,
		Long:  desc,
	}

	var params []paramRecord
	var bodyFields []bodyRecord
	dataFlag := new(string)
	// registeredFlags tracks flag names already registered on this command to
	// prevent panics when a spec declares the same parameter in both query and
	// request body (e.g. GitCode's /oauth/token client_secret).
	registeredFlags := map[string]bool{}

	// ── Path & query params ───────────────────────────────────────────────────
	for _, ref := range op.Parameters {
		if ref == nil || ref.Value == nil {
			continue
		}
		p := ref.Value

		// Skip auth params that are injected automatically by the executor.
		if p.In == "header" && isDiscourseAuthParam(p.Name) {
			continue
		}
		if auth.IsGitcodeAuthParam(p.Name) {
			continue
		}
		if p.In != "path" && p.In != "query" {
			continue
		}

		fn := toFlagName(p.Name)
		if registeredFlags[fn] {
			continue
		}
		pr := paramRecord{specName: p.Name, flagName: fn, location: p.In}

		switch schemaType(p.Schema) {
		case "integer":
			v := new(int)
			pr.valPtr = v
			cmd.Flags().IntVar(v, fn, 0, p.Description)
		case "boolean":
			v := new(bool)
			pr.valPtr = v
			cmd.Flags().BoolVar(v, fn, false, p.Description)
		default:
			v := new(string)
			pr.valPtr = v
			cmd.Flags().StringVar(v, fn, "", p.Description)
		}

		registeredFlags[fn] = true
		if p.Required {
			_ = cmd.MarkFlagRequired(fn)
		}
		params = append(params, pr)
	}

	// ── Request body ──────────────────────────────────────────────────────────
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		for _, mt := range op.RequestBody.Value.Content {
			if mt == nil || mt.Schema == nil || mt.Schema.Value == nil {
				continue
			}
			schema := mt.Schema.Value

			expanded := 0
			for propName, propRef := range schema.Properties {
				if expanded >= 5 {
					break
				}
				if propRef == nil || propRef.Value == nil {
					continue
				}
				// Skip nested objects — let --data handle those
				if schemaType(propRef) == "object" {
					continue
				}

				fn := toFlagName(propName)
				// Skip if a same-named query/path flag is already registered.
				if registeredFlags[fn] {
					continue
				}
				desc := propRef.Value.Description
				isReq := contains(schema.Required, propName)

				br := bodyRecord{specName: propName}
				switch schemaType(propRef) {
				case "integer":
					v := new(int)
					br.valPtr = v
					cmd.Flags().IntVar(v, fn, 0, desc)
				case "boolean":
					v := new(bool)
					br.valPtr = v
					cmd.Flags().BoolVar(v, fn, false, desc)
				default:
					v := new(string)
					br.valPtr = v
					cmd.Flags().StringVar(v, fn, "", desc)
				}

				registeredFlags[fn] = true
				if isReq {
					_ = cmd.MarkFlagRequired(fn)
				}
				bodyFields = append(bodyFields, br)
				expanded++
			}

			// Always offer --data as a full-body override
			cmd.Flags().StringVar(dataFlag, "data", "", "request body as JSON (overrides individual body flags)")
			break // only process first content type
		}
	}

	// ── RunE closure ─────────────────────────────────────────────────────────
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		// Read global persistent flags from root
		format, _ := cmd.Root().PersistentFlags().GetString("format")
		dryRun, _ := cmd.Root().PersistentFlags().GetBool("dry-run")

		// Collect path / query values
		pathParams := map[string]string{}
		queryParams := map[string]string{}
		for _, pr := range params {
			v := derefString(pr.valPtr)
			switch pr.location {
			case "path":
				pathParams[pr.specName] = v
			case "query":
				if v != "" {
					queryParams[pr.specName] = v
				}
			}
		}

		// Build request body
		body := map[string]interface{}{}
		if *dataFlag != "" {
			if err := json.Unmarshal([]byte(*dataFlag), &body); err != nil {
				return fmt.Errorf("invalid --data JSON: %w", err)
			}
		} else {
			for _, br := range bodyFields {
				switch v := br.valPtr.(type) {
				case *string:
					if *v != "" {
						body[br.specName] = *v
					}
				case *int:
					if *v != 0 {
						body[br.specName] = *v
					}
				case *bool:
					body[br.specName] = *v
				}
			}
		}

		// Look up view config; nil means generic fallback rendering.
		viewCfg := viewReg.Lookup(svcName, resource, verb)

		return exec.Execute(context.Background(), &executor.Request{
			ServiceName:  svcName,
			PathTemplate: pathTpl,
			Method:       strings.ToUpper(method),
			PathParams:   pathParams,
			QueryParams:  queryParams,
			Body:         body,
			Format:       format,
			DryRun:       dryRun,
			ViewConfig:   viewCfg,
		})
	}

	return cmd
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func isDiscourseAuthParam(name string) bool {
	return name == "Api-Key" || name == "Api-Username"
}

// toFlagName converts parameter names to kebab-case CLI flag names.
// Handles both snake_case ("topic_id" → "topic-id") and camelCase
// ("padID" → "pad-id", "validUntil" → "valid-until").
func toFlagName(name string) string {
	return camelToKebab(strings.ReplaceAll(name, "_", "-"))
}

// camelToKebab converts a camelCase or PascalCase string to kebab-case.
// Consecutive uppercase sequences (e.g. "ID", "URL") are kept together.
//
//	"padID"       → "pad-id"
//	"validUntil"  → "valid-until"
//	"already-ok"  → "already-ok"
func camelToKebab(s string) string {
	runes := []rune(s)
	var buf strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			// Insert hyphen when transitioning from lower/digit to upper.
			if unicode.IsLower(prev) || unicode.IsDigit(prev) {
				buf.WriteByte('-')
			} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				// Insert hyphen before a new word that starts uppercase
				// inside an all-caps run: "HTMLParser" → "html-parser".
				buf.WriteByte('-')
			}
		}
		buf.WriteRune(unicode.ToLower(r))
	}
	return buf.String()
}

// schemaType returns the primary (non-null) JSON Schema type for a SchemaRef.
func schemaType(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil || ref.Value.Type == nil {
		return "string"
	}
	for _, t := range *ref.Value.Type {
		if t != "null" {
			return t
		}
	}
	return "string"
}

// derefString turns *string / *int / *bool into a string representation.
func derefString(v interface{}) string {
	switch p := v.(type) {
	case *string:
		return *p
	case *int:
		if *p == 0 {
			return ""
		}
		return fmt.Sprintf("%d", *p)
	case *bool:
		if *p {
			return "true"
		}
		return ""
	}
	return ""
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// shouldDeduplicateMethods returns true when all HTTP methods on a path share
// the same base operationId after stripping Using{METHOD} suffixes.
// This identifies Etherpad/Spring-Boot specs where GET and POST expose the
// same operation under different HTTP verbs (e.g. getTextUsingGET / getTextUsingPOST).
func shouldDeduplicateMethods(ops map[string]*openapi3.Operation) bool {
	if len(ops) < 2 {
		return false
	}
	var baseID string
	for _, op := range ops {
		if op == nil {
			continue
		}
		stripped := op.OperationID
		for _, suffix := range httpMethodSuffixes {
			if strings.HasSuffix(stripped, suffix) {
				stripped = stripped[:len(stripped)-len(suffix)]
				break
			}
		}
		if stripped == "" {
			return false
		}
		if baseID == "" {
			baseID = stripped
			continue
		}
		if baseID != stripped {
			return false
		}
	}
	return baseID != ""
}

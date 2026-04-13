// Package builder converts an OpenAPI 3 spec into a Cobra command tree.
//
// Mapping rules:
//   - Resource  = first operation tag, normalised to lowercase kebab-case.
//   - Verb      = known prefix from operationId (list/get/create/update/delete/patch),
//                 falling back to HTTP-method inference.
//   - URL .json suffix is stripped for command names but kept in HTTP requests.
//   - Discourse auth header params (Api-Key, Api-Username) are silently skipped
//     from flag generation; they are injected by the executor.
package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cobra"

	"github.com/cncf/community-cli/internal/config"
	"github.com/cncf/community-cli/internal/executor"
)

// Build returns a *cobra.Command for the given service, populated with one
// sub-command per resource and one leaf command per operation in the spec.
func Build(
	svcName string,
	spec *openapi3.T,
	cfg *config.Config,
	exec *executor.Executor,
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
			for method, op := range pathItem.Operations() {
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

		// Track verbs to detect conflicts within the same resource.
		verbSeen := map[string]bool{}

		for _, e := range ops {
			verb := verbName(e.op.OperationID, e.method, e.path)
			if verbSeen[verb] {
				// Disambiguate by appending a path suffix (e.g. "get-replies")
				verb = verb + "-" + pathSuffix(e.path)
			}
			verbSeen[verb] = true

			verbCmd := buildLeaf(svcName, e.path, e.method, verb, e.op, exec)
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

// verbName determines the CLI verb for an operation.
//
// Priority:
//  1. Known verb prefix in operationId  (listPosts → "list")
//  2. Action segment: /{id}/lock.json → "lock"
//  3. HTTP method + path-param presence → "get" / "list" / "create" etc.
func verbName(opID, method, path string) string {
	lower := strings.ToLower(opID)
	for _, v := range knownVerbPrefixes {
		if strings.HasPrefix(lower, v) {
			return v
		}
	}

	// Check for an action segment after a path param: /posts/{id}/lock.json → lock
	clean := strings.TrimSuffix(path, ".json")
	segs := strings.Split(strings.Trim(clean, "/"), "/")
	if len(segs) >= 2 {
		last := segs[len(segs)-1]
		secondLast := segs[len(segs)-2]
		if !strings.HasPrefix(last, "{") && strings.HasPrefix(secondLast, "{") {
			return last // e.g. "replies", "lock"
		}
	}

	// Fallback: HTTP method
	hasParam := strings.Contains(path, "{")
	switch strings.ToUpper(method) {
	case "GET":
		if hasParam {
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
	svcName, pathTpl, method, verb string,
	op *openapi3.Operation,
	exec *executor.Executor,
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

	// ── Path & query params ───────────────────────────────────────────────────
	for _, ref := range op.Parameters {
		if ref == nil || ref.Value == nil {
			continue
		}
		p := ref.Value

		// Skip Discourse auth headers — injected automatically.
		if p.In == "header" && isDiscourseAuthParam(p.Name) {
			continue
		}
		if p.In != "path" && p.In != "query" {
			continue
		}

		fn := toFlagName(p.Name)
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

		return exec.Execute(context.Background(), &executor.Request{
			ServiceName:  svcName,
			PathTemplate: pathTpl,
			Method:       strings.ToUpper(method),
			PathParams:   pathParams,
			QueryParams:  queryParams,
			Body:         body,
			Format:       format,
			DryRun:       dryRun,
		})
	}

	return cmd
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func isDiscourseAuthParam(name string) bool {
	return name == "Api-Key" || name == "Api-Username"
}

// toFlagName converts "topic_id" → "topic-id" for CLI flag names.
func toFlagName(name string) string {
	return strings.ReplaceAll(name, "_", "-")
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

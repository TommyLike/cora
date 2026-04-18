package builder

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/cncf/cora/internal/config"
	"github.com/cncf/cora/internal/executor"
	"github.com/cncf/cora/internal/view"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func minimalSpec(title string, paths map[string]*openapi3.PathItem) *openapi3.T {
	p := openapi3.NewPaths()
	for path, item := range paths {
		p.Set(path, item)
	}
	return &openapi3.T{
		Info:  &openapi3.Info{Title: title},
		Paths: p,
	}
}

func op(opID string, tags []string) *openapi3.Operation {
	o := openapi3.NewOperation()
	o.OperationID = opID
	o.Summary = opID
	o.Tags = tags
	return o
}

func opWithParam(opID string, tags []string, paramName, in string, required bool) *openapi3.Operation {
	o := op(opID, tags)
	p := &openapi3.Parameter{
		Name:     paramName,
		In:       in,
		Required: required,
		Schema:   openapi3.NewSchemaRef("", openapi3.NewStringSchema()),
	}
	o.Parameters = openapi3.Parameters{&openapi3.ParameterRef{Value: p}}
	return o
}

// newTestCfg builds a config pointing to the given server URL.
func newTestCfg(serverURL string) *config.Config {
	return &config.Config{
		Services: map[string]config.ServiceConfig{
			"svc": {BaseURL: serverURL},
		},
	}
}

// ── Build: command tree structure ────────────────────────────────────────────

func TestBuild_ReturnsServiceCommand(t *testing.T) {
	spec := minimalSpec("Test API", map[string]*openapi3.PathItem{
		"/issues": {Get: op("listIssues", []string{"issues"})},
	})
	cfg := newTestCfg("http://localhost")
	cmd := Build("svc", spec, cfg, executor.New(cfg), view.NewRegistry())

	if cmd == nil {
		t.Fatal("Build returned nil")
	}
	if cmd.Use != "svc" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "svc")
	}
	if cmd.Short != "Test API" {
		t.Errorf("cmd.Short = %q, want %q", cmd.Short, "Test API")
	}
}

func TestBuild_CreatesResourceSubcommand(t *testing.T) {
	spec := minimalSpec("API", map[string]*openapi3.PathItem{
		"/issues":      {Get: op("listIssues", []string{"issues"})},
		"/issues/{id}": {Get: op("getIssue", []string{"issues"})},
	})
	cfg := newTestCfg("http://localhost")
	cmd := Build("svc", spec, cfg, executor.New(cfg), view.NewRegistry())

	// Must have an "issues" subcommand.
	var issuesCmd bool
	for _, sub := range cmd.Commands() {
		if sub.Use == "issues" {
			issuesCmd = true
			// Must have "list" and "get" leaf commands.
			var hasGet, hasList bool
			for _, leaf := range sub.Commands() {
				switch leaf.Use {
				case "list":
					hasList = true
				case "get":
					hasGet = true
				}
			}
			if !hasGet {
				t.Error("issues subcommand missing 'get' leaf")
			}
			if !hasList {
				t.Error("issues subcommand missing 'list' leaf")
			}
		}
	}
	if !issuesCmd {
		t.Error("Build did not create 'issues' resource subcommand")
	}
}

func TestBuild_NilSpec_ReturnsEmptyService(t *testing.T) {
	spec := &openapi3.T{Info: &openapi3.Info{Title: "Empty"}, Paths: nil}
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"svc": {BaseURL: "http://x"}}}
	cmd := Build("svc", spec, cfg, executor.New(cfg), view.NewRegistry())

	if cmd == nil {
		t.Fatal("Build with nil Paths returned nil")
	}
	if len(cmd.Commands()) != 0 {
		t.Errorf("expected 0 subcommands for nil Paths, got %d", len(cmd.Commands()))
	}
}

func TestBuild_PathParamBecomesFlag(t *testing.T) {
	spec := minimalSpec("API", map[string]*openapi3.PathItem{
		"/issues/{number}": {
			Get: opWithParam("getIssue", []string{"issues"}, "number", "path", true),
		},
	})
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"svc": {BaseURL: "http://x"}}}
	cmd := Build("svc", spec, cfg, executor.New(cfg), view.NewRegistry())

	var getCmd interface {
		Flags() interface{ Lookup(string) interface{} }
	}
	_ = getCmd
	for _, resCmd := range cmd.Commands() {
		for _, leaf := range resCmd.Commands() {
			if leaf.Use == "get" {
				f := leaf.Flags().Lookup("number")
				if f == nil {
					t.Error("'get' leaf missing --number flag derived from path param")
				}
				return
			}
		}
	}
	t.Error("could not find 'get' leaf command")
}

func TestBuild_QueryParamBecomesFlag(t *testing.T) {
	spec := minimalSpec("API", map[string]*openapi3.PathItem{
		"/issues": {
			Get: opWithParam("listIssues", []string{"issues"}, "state", "query", false),
		},
	})
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"svc": {BaseURL: "http://x"}}}
	cmd := Build("svc", spec, cfg, executor.New(cfg), view.NewRegistry())

	for _, resCmd := range cmd.Commands() {
		for _, leaf := range resCmd.Commands() {
			if leaf.Use == "list" {
				f := leaf.Flags().Lookup("state")
				if f == nil {
					t.Error("'list' leaf missing --state flag from query param")
				}
				return
			}
		}
	}
	t.Error("could not find 'list' leaf command")
}

func TestBuild_DiscourseAuthParamsSkipped(t *testing.T) {
	spec := minimalSpec("Forum", map[string]*openapi3.PathItem{
		"/posts": {
			Get: func() *openapi3.Operation {
				o := op("listPosts", []string{"posts"})
				addHeader := func(name string) *openapi3.ParameterRef {
					return &openapi3.ParameterRef{Value: &openapi3.Parameter{
						Name:   name,
						In:     "header",
						Schema: openapi3.NewSchemaRef("", openapi3.NewStringSchema()),
					}}
				}
				o.Parameters = openapi3.Parameters{
					addHeader("Api-Key"),
					addHeader("Api-Username"),
					addHeader("X-Custom"),
				}
				return o
			}(),
		},
	})
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"svc": {BaseURL: "http://x"}}}
	cmd := Build("svc", spec, cfg, executor.New(cfg), view.NewRegistry())

	for _, resCmd := range cmd.Commands() {
		for _, leaf := range resCmd.Commands() {
			if leaf.Use == "list" {
				if leaf.Flags().Lookup("api-key") != nil {
					t.Error("Api-Key should be filtered out (injected automatically)")
				}
				if leaf.Flags().Lookup("api-username") != nil {
					t.Error("Api-Username should be filtered out")
				}
				// X-Custom header is not a path/query param so it's also skipped by the location filter.
				return
			}
		}
	}
	t.Error("could not find 'list' leaf")
}

func TestBuild_MultipleResources(t *testing.T) {
	spec := minimalSpec("API", map[string]*openapi3.PathItem{
		"/issues": {Get: op("listIssues", []string{"issues"})},
		"/repos":  {Get: op("listRepos", []string{"repos"})},
		"/users":  {Get: op("listUsers", []string{"users"})},
	})
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"svc": {BaseURL: "http://x"}}}
	cmd := Build("svc", spec, cfg, executor.New(cfg), view.NewRegistry())

	resourceNames := map[string]bool{}
	for _, sub := range cmd.Commands() {
		resourceNames[sub.Use] = true
	}
	for _, want := range []string{"issues", "repos", "users"} {
		if !resourceNames[want] {
			t.Errorf("expected resource %q in command tree, got: %v", want, resourceNames)
		}
	}
}

// ── schemaType ────────────────────────────────────────────────────────────────

func TestSchemaType(t *testing.T) {
	cases := []struct {
		schema *openapi3.SchemaRef
		want   string
	}{
		{openapi3.NewSchemaRef("", openapi3.NewStringSchema()), "string"},
		{openapi3.NewSchemaRef("", openapi3.NewIntegerSchema()), "integer"},
		{openapi3.NewSchemaRef("", openapi3.NewBoolSchema()), "boolean"},
		{openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"object"}}), "object"},
		{nil, "string"},
		{&openapi3.SchemaRef{Value: nil}, "string"},
	}
	for _, tc := range cases {
		got := schemaType(tc.schema)
		if got != tc.want {
			t.Errorf("schemaType(%v) = %q, want %q", tc.schema, got, tc.want)
		}
	}
}

// ── truncate (executor helper, exposed via Build path) ───────────────────────

func TestBuild_NoInfoTitle_EmptyShort(t *testing.T) {
	spec := &openapi3.T{Info: nil, Paths: openapi3.NewPaths()}
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"svc": {BaseURL: "http://x"}}}
	cmd := Build("svc", spec, cfg, executor.New(cfg), view.NewRegistry())
	if cmd.Short != "" {
		t.Errorf("nil Info should give empty Short, got %q", cmd.Short)
	}
}

// ── pathSuffix ────────────────────────────────────────────────────────────────

func TestPathSuffix(t *testing.T) {
	cases := []struct{ path, want string }{
		{"/posts/{id}/replies.json", "replies"},
		{"/posts.json", "posts"},
		{"/api/v5/repos/{owner}/{repo}/issues/{number}", "issues"},
	}
	for _, tc := range cases {
		if got := pathSuffix(tc.path); got != tc.want {
			t.Errorf("pathSuffix(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// ── Build: Etherpad deduplication ────────────────────────────────────────────

func TestBuild_EtherpadStyle_DeduplicatesGetPost(t *testing.T) {
	getOp := openapi3.NewOperation()
	getOp.OperationID = "getTextUsingGET"
	getOp.Tags = []string{"pads"}
	getOp.Summary = "Get text"

	postOp := openapi3.NewOperation()
	postOp.OperationID = "getTextUsingPOST"
	postOp.Tags = []string{"pads"}
	postOp.Summary = "Get text (post)"

	spec := minimalSpec("Etherpad", map[string]*openapi3.PathItem{
		"/getText": {Get: getOp, Post: postOp},
	})
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"svc": {BaseURL: "http://x"}}}
	cmd := Build("svc", spec, cfg, executor.New(cfg), view.NewRegistry())

	for _, resCmd := range cmd.Commands() {
		if resCmd.Use == "pads" {
			if len(resCmd.Commands()) != 1 {
				t.Errorf("expected 1 deduplicated command, got %d", len(resCmd.Commands()))
			}
			leaf := resCmd.Commands()[0]
			if !strings.HasPrefix(leaf.Use, "get-text") {
				t.Errorf("expected 'get-text' command, got %q", leaf.Use)
			}
			return
		}
	}
	t.Error("pads resource not found")
}

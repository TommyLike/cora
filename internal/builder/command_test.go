package builder

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// newOp builds a minimal openapi3.Operation with the given tags.
func newOp(tags []string) *openapi3.Operation {
	op := openapi3.NewOperation()
	op.Tags = tags
	return op
}

// --- resourceName ---

func TestResourceName_usesFirstTag(t *testing.T) {
	tests := []struct {
		tags []string
		path string
		want string
	}{
		{[]string{"Posts"}, "/posts.json", "posts"},
		{[]string{"Mail Threads"}, "/threads", "mail-threads"},
		{[]string{"ISSUES"}, "/issues", "issues"},
	}
	for _, tc := range tests {
		got := resourceName(newOp(tc.tags), tc.path)
		if got != tc.want {
			t.Errorf("resourceName(tags=%v, path=%q) = %q, want %q", tc.tags, tc.path, got, tc.want)
		}
	}
}

func TestResourceName_fallsBackToPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/posts/{id}.json", "posts"},
		{"/posts/{id}/replies.json", "replies"},
	}
	for _, tc := range tests {
		got := resourceName(newOp(nil), tc.path)
		if got != tc.want {
			t.Errorf("resourceName(nil tags, path=%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// --- lastPathSegment ---

func TestLastPathSegment(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/posts.json", "posts"},
		{"/posts/{id}.json", "posts"},
		{"/posts/{id}/replies.json", "replies"},
		{"/", "root"},
		{"/{id}", "root"},
		{"/v1/topics/{id}/posts", "posts"},
	}
	for _, tc := range tests {
		got := lastPathSegment(tc.path)
		if got != tc.want {
			t.Errorf("lastPathSegment(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// --- verbName ---

func TestVerbName_knownPrefix(t *testing.T) {
	tests := []struct {
		opID   string
		method string
		path   string
		want   string
	}{
		{"listPosts", "GET", "/posts.json", "list"},
		{"getPosts", "GET", "/posts/{id}.json", "get"},
		{"createPost", "POST", "/posts.json", "create"},
		{"updatePost", "PUT", "/posts/{id}.json", "update"},
		{"deletePost", "DELETE", "/posts/{id}.json", "delete"},
		{"patchPost", "PATCH", "/posts/{id}.json", "patch"},
	}
	for _, tc := range tests {
		got := verbName(tc.opID, tc.method, tc.path)
		if got != tc.want {
			t.Errorf("verbName(%q, %q, %q) = %q, want %q", tc.opID, tc.method, tc.path, got, tc.want)
		}
	}
}

func TestVerbName_actionSegment(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/posts/{id}/lock.json", "lock"},
		{"/posts/{id}/replies.json", "replies"},
	}
	for _, tc := range tests {
		got := verbName("", "POST", tc.path)
		if got != tc.want {
			t.Errorf("verbName(action segment, path=%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestVerbName_HTTPMethodFallback(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{"GET", "/posts.json", "list"},
		{"GET", "/posts/{id}.json", "get"},
		{"POST", "/posts.json", "create"},
		{"PUT", "/posts/{id}.json", "update"},
		{"PATCH", "/posts/{id}.json", "update"},
		{"DELETE", "/posts/{id}.json", "delete"},
		// GitCode-style paths: mid-path params should not trigger "get" for collections.
		{"GET", "/api/v5/repos/{owner}/{repo}/issues", "list"},
		{"GET", "/api/v5/repos/{owner}/{repo}/issues/{number}", "get"},
	}
	for _, tc := range tests {
		got := verbName("", tc.method, tc.path)
		if got != tc.want {
			t.Errorf("verbName(method=%q, path=%q) = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

// --- isPathEncodedOpID ---

func TestIsPathEncodedOpID(t *testing.T) {
	yes := []string{
		"get_api_v5_repos_{owner}_{repo}_issues_{number}",
		"post_api_v5_repos_{owner}_{repo}_issues",
		"delete_api_v3_users_{id}",
	}
	no := []string{
		"listPosts", "getPost", "createPost",
		"getTextUsingGET",
		"",
		"get_something_without_api_prefix",
	}
	for _, opID := range yes {
		if !isPathEncodedOpID(opID) {
			t.Errorf("isPathEncodedOpID(%q) should be true", opID)
		}
	}
	for _, opID := range no {
		if isPathEncodedOpID(opID) {
			t.Errorf("isPathEncodedOpID(%q) should be false", opID)
		}
	}
}

// --- hasTrailingParam ---

func TestHasTrailingParam(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v5/repos/{owner}/{repo}/issues/{number}", true},
		{"/api/v5/repos/{owner}/{repo}/issues", false},
		{"/posts/{id}.json", true},
		{"/posts.json", false},
		{"/api/v5/user/issues", false},
		{"/api/v5/enterprises/{enterprise}/issues/{number}", true},
	}
	for _, tc := range tests {
		got := hasTrailingParam(tc.path)
		if got != tc.want {
			t.Errorf("hasTrailingParam(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// --- pathContext ---

func TestPathContext(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/v5/repos/{owner}/{repo}/issues/{number}", "repo"},
		{"/api/v5/enterprises/{enterprise}/issues/{number}", "enterprise"},
		{"/api/v5/user/issues", "user"},
		{"/api/v5/orgs/{org}/issues", "org"},
		{"/posts/{id}.json", "post"},
	}
	for _, tc := range tests {
		got := pathContext(tc.path)
		if got != tc.want {
			t.Errorf("pathContext(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// --- toFlagName ---

func TestToFlagName(t *testing.T) {
	tests := []struct{ in, want string }{
		// snake_case (existing behaviour)
		{"topic_id", "topic-id"},
		{"api_key", "api-key"},
		{"id", "id"},
		{"already-kebab", "already-kebab"},
		// camelCase (Etherpad-style parameters)
		{"padID", "pad-id"},
		{"groupID", "group-id"},
		{"validUntil", "valid-until"},
		{"authorMapper", "author-mapper"},
		{"startRev", "start-rev"},
	}
	for _, tc := range tests {
		got := toFlagName(tc.in)
		if got != tc.want {
			t.Errorf("toFlagName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- verbName with Using{METHOD} suffix ---

func TestVerbName_stripsUsingMethodSuffix(t *testing.T) {
	tests := []struct {
		opID   string
		method string
		path   string
		want   string
	}{
		{"getTextUsingGET", "GET", "/getText", "get-text"},
		{"createGroupUsingPOST", "POST", "/createGroup", "create-group"},
		{"deleteGroupUsingGET", "GET", "/deleteGroup", "delete-group"},
		{"listAllGroupsUsingGET", "GET", "/listAllGroups", "list-all-groups"},
		{"createGroupIfNotExistsForUsingGET", "GET", "/createGroupIfNotExistsFor", "create-group-if-not-exists-for"},
	}
	for _, tc := range tests {
		got := verbName(tc.opID, tc.method, tc.path)
		if got != tc.want {
			t.Errorf("verbName(%q, %q, %q) = %q, want %q", tc.opID, tc.method, tc.path, got, tc.want)
		}
	}
}

// --- isDiscourseAuthParam ---

func TestIsDiscourseAuthParam(t *testing.T) {
	yes := []string{"Api-Key", "Api-Username"}
	no := []string{"id", "username", "api-key", "Api-key"}

	for _, name := range yes {
		if !isDiscourseAuthParam(name) {
			t.Errorf("isDiscourseAuthParam(%q) should be true", name)
		}
	}
	for _, name := range no {
		if isDiscourseAuthParam(name) {
			t.Errorf("isDiscourseAuthParam(%q) should be false", name)
		}
	}
}

// --- derefString ---

func TestDerefString(t *testing.T) {
	s := "hello"
	if got := derefString(&s); got != "hello" {
		t.Errorf("*string: got %q, want %q", got, "hello")
	}

	n := 42
	if got := derefString(&n); got != "42" {
		t.Errorf("*int(42): got %q, want %q", got, "42")
	}

	zero := 0
	if got := derefString(&zero); got != "" {
		t.Errorf("*int(0): got %q, want empty string", got)
	}

	b := true
	if got := derefString(&b); got != "true" {
		t.Errorf("*bool(true): got %q, want %q", got, "true")
	}

	bf := false
	if got := derefString(&bf); got != "" {
		t.Errorf("*bool(false): got %q, want empty string", got)
	}
}

// --- shouldDeduplicateMethods ---

func TestShouldDeduplicateMethods(t *testing.T) {
	makeOp := func(opID string) *openapi3.Operation {
		op := openapi3.NewOperation()
		op.OperationID = opID
		return op
	}

	// Etherpad-style: GET and POST share same base id → deduplicate
	same := map[string]*openapi3.Operation{
		"get":  makeOp("getTextUsingGET"),
		"post": makeOp("getTextUsingPOST"),
	}
	if !shouldDeduplicateMethods(same) {
		t.Error("same base id: expected true")
	}

	// Different base ids → do not deduplicate
	diff := map[string]*openapi3.Operation{
		"get":  makeOp("listPostsUsingGET"),
		"post": makeOp("createPostUsingPOST"),
	}
	if shouldDeduplicateMethods(diff) {
		t.Error("different base ids: expected false")
	}

	// Single method → do not deduplicate
	single := map[string]*openapi3.Operation{
		"get": makeOp("getTextUsingGET"),
	}
	if shouldDeduplicateMethods(single) {
		t.Error("single method: expected false")
	}

	// No Using{METHOD} suffix → do not deduplicate (plain REST spec)
	plain := map[string]*openapi3.Operation{
		"get":  makeOp("listPosts"),
		"post": makeOp("createPost"),
	}
	if shouldDeduplicateMethods(plain) {
		t.Error("plain operationIds with no suffix: expected false")
	}
}

// --- contains ---

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}
	if !contains(slice, "b") {
		t.Error("contains should return true for 'b'")
	}
	if contains(slice, "z") {
		t.Error("contains should return false for 'z'")
	}
	if contains(nil, "a") {
		t.Error("contains on nil slice should return false")
	}
}

package view

import "testing"

// ── Registry.Register & Lookup ───────────────────────────────────────────────

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	cfg := ViewConfig{
		Columns: []ViewColumn{{Field: "id", Label: "ID"}},
	}
	r.Register("gitcode", "issues/get", cfg)

	got := r.Lookup("gitcode", "issues", "get")
	if got == nil {
		t.Fatal("expected non-nil ViewConfig")
	}
	if len(got.Columns) != 1 || got.Columns[0].Field != "id" {
		t.Errorf("unexpected columns: %v", got.Columns)
	}
}

func TestRegistry_Lookup_UnknownService_ReturnsNil(t *testing.T) {
	r := NewRegistry()
	if got := r.Lookup("unknown", "issues", "get"); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestRegistry_Lookup_UnknownOperation_ReturnsNil(t *testing.T) {
	r := NewRegistry()
	r.Register("gitcode", "issues/get", ViewConfig{})

	if got := r.Lookup("gitcode", "issues", "list"); got != nil {
		t.Errorf("expected nil for unregistered op, got %+v", got)
	}
}

func TestRegistry_UserOverridesBuiltin(t *testing.T) {
	r := NewRegistry()
	builtin := ViewConfig{Columns: []ViewColumn{{Field: "id"}}}
	user := ViewConfig{Columns: []ViewColumn{{Field: "number", Label: "No."}}}

	r.Register("gitcode", "issues/list", builtin)
	r.Register("gitcode", "issues/list", user) // override

	got := r.Lookup("gitcode", "issues", "list")
	if got == nil {
		t.Fatal("expected non-nil ViewConfig")
	}
	if got.Columns[0].Field != "number" {
		t.Errorf("expected user override (number), got %q", got.Columns[0].Field)
	}
}

func TestRegistry_OpKeyFormat(t *testing.T) {
	r := NewRegistry()
	r.Register("svc", "repos/get", ViewConfig{RootField: "data"})

	// Lookup uses resource + "/" + verb internally.
	got := r.Lookup("svc", "repos", "get")
	if got == nil {
		t.Fatal("expected ViewConfig for repos/get")
	}
	if got.RootField != "data" {
		t.Errorf("RootField = %q, want %q", got.RootField, "data")
	}
}

func TestRegistry_MultipleServices(t *testing.T) {
	r := NewRegistry()
	r.Register("gitcode", "issues/list", ViewConfig{RootField: "issues"})
	r.Register("forum", "topics/list", ViewConfig{RootField: "topics"})

	gc := r.Lookup("gitcode", "issues", "list")
	if gc == nil || gc.RootField != "issues" {
		t.Errorf("gitcode issues/list: %+v", gc)
	}
	fm := r.Lookup("forum", "topics", "list")
	if fm == nil || fm.RootField != "topics" {
		t.Errorf("forum topics/list: %+v", fm)
	}
	// Cross-service: gitcode op not visible in forum.
	if r.Lookup("forum", "issues", "list") != nil {
		t.Error("expected nil for gitcode op looked up in forum")
	}
}

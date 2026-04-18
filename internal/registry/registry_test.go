package registry

import (
	"sort"
	"testing"
)

// newTestRegistry creates a Registry with a few hand-built entries (no real
// spec loading needed for lookup/alias tests).
func newTestRegistry() *Registry {
	r := &Registry{
		entries: make(map[string]*Entry),
		aliases: make(map[string]string),
	}
	return r
}

func entry(name string, aliases ...string) *Entry {
	return &Entry{Name: name, Aliases: aliases}
}

// ── Register & Lookup ────────────────────────────────────────────────────────

func TestRegisterAndLookupByName(t *testing.T) {
	r := newTestRegistry()
	r.Register(entry("gitcode"))

	e, err := r.Lookup("gitcode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Name != "gitcode" {
		t.Errorf("Name = %q, want %q", e.Name, "gitcode")
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	r := newTestRegistry()
	r.Register(entry("gitcode"))

	for _, name := range []string{"GITCODE", "GitCode", "gitcode"} {
		e, err := r.Lookup(name)
		if err != nil {
			t.Errorf("Lookup(%q) error: %v", name, err)
			continue
		}
		if e.Name != "gitcode" {
			t.Errorf("Lookup(%q).Name = %q, want %q", name, e.Name, "gitcode")
		}
	}
}

func TestLookup_Unknown_ReturnsError(t *testing.T) {
	r := newTestRegistry()
	_, err := r.Lookup("nope")
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
}

// ── Aliases ──────────────────────────────────────────────────────────────────

func TestLookupByAlias(t *testing.T) {
	r := newTestRegistry()
	r.Register(entry("forum", "discourse", "Discourse"))

	for _, alias := range []string{"discourse", "Discourse", "DISCOURSE"} {
		e, err := r.Lookup(alias)
		if err != nil {
			t.Errorf("Lookup(%q) error: %v", alias, err)
			continue
		}
		if e.Name != "forum" {
			t.Errorf("Lookup(%q).Name = %q, want %q", alias, e.Name, "forum")
		}
	}
}

func TestRegister_OverwritesExistingEntry(t *testing.T) {
	r := newTestRegistry()
	r.Register(&Entry{Name: "svc", BaseURL: "http://old.example.com"})
	r.Register(&Entry{Name: "svc", BaseURL: "http://new.example.com"})

	e, err := r.Lookup("svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.BaseURL != "http://new.example.com" {
		t.Errorf("BaseURL = %q, want %q", e.BaseURL, "http://new.example.com")
	}
}

// ── Names & Entries ──────────────────────────────────────────────────────────

func TestNames_ReturnsAllRegistered(t *testing.T) {
	r := newTestRegistry()
	r.Register(entry("gitcode"))
	r.Register(entry("forum"))
	r.Register(entry("etherpad"))

	names := r.Names()
	sort.Strings(names)

	want := []string{"etherpad", "forum", "gitcode"}
	if len(names) != len(want) {
		t.Fatalf("Names() = %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("names[%d] = %q, want %q", i, names[i], n)
		}
	}
}

func TestEntries_ReturnsAllRegistered(t *testing.T) {
	r := newTestRegistry()
	r.Register(entry("svc1"))
	r.Register(entry("svc2"))

	entries := r.Entries()
	if len(entries) != 2 {
		t.Errorf("Entries() = %d items, want 2", len(entries))
	}
}

func TestNames_EmptyRegistry(t *testing.T) {
	r := newTestRegistry()
	if names := r.Names(); len(names) != 0 {
		t.Errorf("Names() = %v, want empty", names)
	}
}

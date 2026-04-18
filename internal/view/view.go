// Package view provides the result-display customisation layer.
//
// A ViewConfig declares which fields to extract from an API response and how
// to render each one. Configs are looked up by (service, resource/verb) and
// fall back to a generic table when nothing matches.
package view

// ColumnFormat controls how a field value is rendered in the output.
type ColumnFormat string

const (
	// FormatText is the default: convert to string, strip newlines, apply Truncate.
	FormatText ColumnFormat = "text"
	// FormatJSON renders the raw JSON fragment (compact single-line, or indented when Indent=true).
	FormatJSON ColumnFormat = "json"
	// FormatDate parses an ISO-8601 timestamp and reformats it using DateFmt.
	FormatDate ColumnFormat = "date"
	// FormatMultiline is like text but preserves newline characters.
	FormatMultiline ColumnFormat = "multiline"
)

// ViewColumn describes one output column (list mode) or one row (object mode).
type ViewColumn struct {
	Field    string       `yaml:"field"`    // dot-separated JSON path, e.g. "user.login"
	Label    string       `yaml:"label"`    // table header / KV key; auto-derived when empty
	Format   ColumnFormat `yaml:"format"`   // rendering mode; defaults to FormatText
	Truncate int          `yaml:"truncate"` // max rune count before "…" (0 = unlimited)
	Width    int          `yaml:"width"`    // fixed column width in list mode (0 = auto)
	DateFmt  string       `yaml:"date_fmt"` // Go time format for FormatDate; default "2006-01-02"
	Indent   bool         `yaml:"indent"`   // FormatJSON only: use indented pretty-print
}

// ViewConfig is the complete display configuration for one API operation.
type ViewConfig struct {
	// RootField is the JSON key whose value should be treated as the response
	// root. Useful when the API wraps a list inside {"items":[...]}.
	// Empty string means use the response root directly.
	RootField string       `yaml:"root_field"`
	Columns   []ViewColumn `yaml:"columns"`
}

// Registry maps (service → resource/verb) to a ViewConfig.
type Registry struct {
	views map[string]map[string]ViewConfig
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{views: make(map[string]map[string]ViewConfig)}
}

// Register adds or replaces a ViewConfig for the given service and operation key.
// opKey format: "resource/verb", e.g. "issues/get".
// A user-supplied call always overwrites a built-in one.
func (r *Registry) Register(service, opKey string, cfg ViewConfig) {
	if r.views[service] == nil {
		r.views[service] = make(map[string]ViewConfig)
	}
	r.views[service][opKey] = cfg
}

// Lookup returns the ViewConfig for (service, resource, verb), or nil when none
// is registered. The caller should fall back to generic table rendering on nil.
func (r *Registry) Lookup(service, resource, verb string) *ViewConfig {
	key := resource + "/" + verb
	if svcViews, ok := r.views[service]; ok {
		if cfg, ok := svcViews[key]; ok {
			return &cfg
		}
	}
	return nil
}

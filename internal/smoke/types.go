package smoke

import "time"

// Scenario is loaded from a single YAML file.
type Scenario struct {
	Name       string      `yaml:"name"`
	Service    string      `yaml:"service"`
	Args       []string    `yaml:"args"`
	Format     string      `yaml:"format"`     // "table" | "json" | "yaml"; default "table"
	TimeoutMs  int         `yaml:"timeout_ms"` // default 10000
	Skip       bool        `yaml:"skip"`
	SkipReason string      `yaml:"skip_reason"`
	Assertions []Assertion `yaml:"assertions"`
	FilePath   string      `yaml:"-"` // populated by loader
}

// Assertion is one check within a Scenario.
type Assertion struct {
	Type   string   `yaml:"type"`
	Value  int      `yaml:"value"`  // exit_code, response_time_lt, table_row_count_gte
	Str    string   `yaml:"str"`    // stdout_contains, stderr_not_contains, json_key_not_empty key name
	Values []string `yaml:"values"` // table_has_columns, json_has_keys
}

// UnmarshalYAML handles the dual-type "value" field (int or string)
// and the "key" alias used by json_key_not_empty.
func (a *Assertion) UnmarshalYAML(unmarshal func(any) error) error {
	raw := map[string]any{}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	a.Type, _ = raw["type"].(string)

	// "values" → []string
	if v, ok := raw["values"]; ok {
		if slice, ok := v.([]any); ok {
			a.Values = make([]string, len(slice))
			for i, s := range slice {
				a.Values[i], _ = s.(string)
			}
		}
	}

	// "value" can be int (numeric assertions) or string (contains checks).
	switch v := raw["value"].(type) {
	case int:
		a.Value = v
	case float64:
		a.Value = int(v) // YAML numbers decode as float64
	case string:
		a.Str = v
	}

	// "key" is an alias for Str used in json_key_not_empty.
	if k, ok := raw["key"].(string); ok && a.Str == "" {
		a.Str = k
	}

	// "value_str" explicit string override.
	if s, ok := raw["value_str"].(string); ok && a.Str == "" {
		a.Str = s
	}

	return nil
}

// Status is the outcome of a ScenarioResult.
type Status string

const (
	StatusPass    Status = "PASS"
	StatusFail    Status = "FAIL"
	StatusSkip    Status = "SKIP"
	StatusTimeout Status = "TIMEOUT"
	StatusError   Status = "ERROR"
)

// ScenarioResult is the full output of executing one Scenario.
type ScenarioResult struct {
	Scenario         Scenario
	Status           Status
	DurationMs       int64
	Stdout           string
	Stderr           string
	ExitCode         int
	AssertionResults []AssertionResult
	Err              string // non-assertion error (subprocess failed to start, etc.)
}

// AssertionResult is the outcome of one Assertion evaluation.
type AssertionResult struct {
	Assertion Assertion
	Passed    bool
	Actual    string // actual value observed, for report display
	Message   string // failure description
}

// RunReport aggregates all results for a full smoke run.
type RunReport struct {
	GeneratedAt     string
	ConfigPath      string
	TotalDurationMs int64
	Results         []ScenarioResult
}

// Passed returns the count of PASS results.
func (r *RunReport) Passed() int {
	n := 0
	for _, res := range r.Results {
		if res.Status == StatusPass {
			n++
		}
	}
	return n
}

// Failed returns the count of FAIL + TIMEOUT + ERROR results.
func (r *RunReport) Failed() int {
	n := 0
	for _, res := range r.Results {
		if res.Status == StatusFail || res.Status == StatusTimeout || res.Status == StatusError {
			n++
		}
	}
	return n
}

// Skipped returns the count of SKIP results.
func (r *RunReport) Skipped() int {
	n := 0
	for _, res := range r.Results {
		if res.Status == StatusSkip {
			n++
		}
	}
	return n
}

// TotalDuration returns the total run duration as a Duration.
func (r *RunReport) TotalDuration() time.Duration {
	return time.Duration(r.TotalDurationMs) * time.Millisecond
}

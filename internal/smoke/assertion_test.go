package smoke

import "testing"

func eval(t *testing.T, a Assertion, stdout, stderr string, exitCode int, durationMs int64) AssertionResult {
	t.Helper()
	return EvaluateAssertion(a, stdout, stderr, exitCode, durationMs)
}

func assertPass(t *testing.T, ar AssertionResult) {
	t.Helper()
	if !ar.Passed {
		t.Errorf("expected PASS, got FAIL: %s", ar.Message)
	}
}

func assertFail(t *testing.T, ar AssertionResult) {
	t.Helper()
	if ar.Passed {
		t.Errorf("expected FAIL, got PASS")
	}
}

func TestAssertion_ExitCode_Pass(t *testing.T) {
	assertPass(t, eval(t, Assertion{Type: "exit_code", Value: 0}, "", "", 0, 0))
}

func TestAssertion_ExitCode_Fail(t *testing.T) {
	ar := eval(t, Assertion{Type: "exit_code", Value: 0}, "", "", 1, 0)
	assertFail(t, ar)
	if ar.Message == "" {
		t.Error("expected non-empty failure message")
	}
}

func TestAssertion_ResponseTimeLt_Pass(t *testing.T) {
	assertPass(t, eval(t, Assertion{Type: "response_time_lt", Value: 3000}, "", "", 0, 800))
}

func TestAssertion_ResponseTimeLt_Fail(t *testing.T) {
	assertFail(t, eval(t, Assertion{Type: "response_time_lt", Value: 3000}, "", "", 0, 5000))
}

func TestAssertion_StdoutNotEmpty_Pass(t *testing.T) {
	assertPass(t, eval(t, Assertion{Type: "stdout_not_empty"}, "hello", "", 0, 0))
}

func TestAssertion_StdoutNotEmpty_Fail(t *testing.T) {
	assertFail(t, eval(t, Assertion{Type: "stdout_not_empty"}, "   ", "", 0, 0))
}

func TestAssertion_StdoutContains_Pass(t *testing.T) {
	assertPass(t, eval(t, Assertion{Type: "stdout_contains", Str: "Title"}, "| Title | State |", "", 0, 0))
}

func TestAssertion_StdoutContains_Fail(t *testing.T) {
	assertFail(t, eval(t, Assertion{Type: "stdout_contains", Str: "Title"}, "no match here", "", 0, 0))
}

func TestAssertion_StdoutNotContains_Pass(t *testing.T) {
	assertPass(t, eval(t, Assertion{Type: "stdout_not_contains", Str: "ERROR"}, "all good", "", 0, 0))
}

func TestAssertion_StdoutNotContains_Fail(t *testing.T) {
	assertFail(t, eval(t, Assertion{Type: "stdout_not_contains", Str: "ERROR"}, "ERROR occurred", "", 0, 0))
}

func TestAssertion_StderrNotContains_Pass(t *testing.T) {
	assertPass(t, eval(t, Assertion{Type: "stderr_not_contains", Str: "ERROR"}, "", "", 0, 0))
}

func TestAssertion_StderrNotContains_Fail(t *testing.T) {
	assertFail(t, eval(t, Assertion{Type: "stderr_not_contains", Str: "ERROR"}, "", "[ERROR] something broke", 0, 0))
}

func TestAssertion_TableHasColumns_Pass(t *testing.T) {
	table := "+--------+-------+-------+\n| NUMBER | TITLE | STATE |\n+--------+-------+-------+\n| 1      | foo   | open  |\n+--------+-------+-------+\n"
	a := Assertion{Type: "table_has_columns", Values: []string{"Number", "Title"}}
	assertPass(t, eval(t, a, table, "", 0, 0))
}

func TestAssertion_TableHasColumns_Fail(t *testing.T) {
	table := "+--------+-------+\n| NUMBER | TITLE |\n+--------+-------+\n| 1      | foo   |\n+--------+-------+\n"
	a := Assertion{Type: "table_has_columns", Values: []string{"Number", "Title", "State"}}
	assertFail(t, eval(t, a, table, "", 0, 0))
}

func TestAssertion_TableRowCountGte_Pass(t *testing.T) {
	table := "+----+-----+\n| ID | VAL |\n+----+-----+\n| 1  | a   |\n| 2  | b   |\n+----+-----+\n"
	assertPass(t, eval(t, Assertion{Type: "table_row_count_gte", Value: 2}, table, "", 0, 0))
}

func TestAssertion_TableRowCountGte_Fail(t *testing.T) {
	// No data rows between separators
	table := "+----+-----+\n| ID | VAL |\n+----+-----+\n+----+-----+\n"
	assertFail(t, eval(t, Assertion{Type: "table_row_count_gte", Value: 1}, table, "", 0, 0))
}

func TestAssertion_JSONHasKeys_Pass(t *testing.T) {
	j := `{"title":"foo","state":"open","number":1}`
	a := Assertion{Type: "json_has_keys", Values: []string{"title", "state"}}
	assertPass(t, eval(t, a, j, "", 0, 0))
}

func TestAssertion_JSONHasKeys_Fail(t *testing.T) {
	j := `{"title":"foo"}`
	a := Assertion{Type: "json_has_keys", Values: []string{"title", "state"}}
	assertFail(t, eval(t, a, j, "", 0, 0))
}

func TestAssertion_JSONKeyNotEmpty_Pass(t *testing.T) {
	j := `{"title":"hello","state":"open"}`
	assertPass(t, eval(t, Assertion{Type: "json_key_not_empty", Str: "title"}, j, "", 0, 0))
}

func TestAssertion_JSONKeyNotEmpty_Fail_EmptyString(t *testing.T) {
	j := `{"title":"","state":"open"}`
	assertFail(t, eval(t, Assertion{Type: "json_key_not_empty", Str: "title"}, j, "", 0, 0))
}

func TestAssertion_JSONKeyNotEmpty_Fail_Null(t *testing.T) {
	j := `{"title":null}`
	assertFail(t, eval(t, Assertion{Type: "json_key_not_empty", Str: "title"}, j, "", 0, 0))
}

func TestAssertion_UnknownType_Fail(t *testing.T) {
	ar := eval(t, Assertion{Type: "nonexistent_type"}, "", "", 0, 0)
	assertFail(t, ar)
}

package errs_test

import (
	"errors"
	"testing"

	"github.com/cncf/community-cli/pkg/errs"
)

// --- CLIError.Error() ---

func TestCLIError_Error_withoutCause(t *testing.T) {
	err := &errs.CLIError{Code: errs.ExitConfig, Msg: "service not found"}
	if got := err.Error(); got != "service not found" {
		t.Errorf("expected %q, got %q", "service not found", got)
	}
}

func TestCLIError_Error_withCause(t *testing.T) {
	cause := errors.New("dial tcp: connection refused")
	err := &errs.CLIError{Code: errs.ExitAPI, Msg: "request failed", Cause: cause}
	want := "request failed: dial tcp: connection refused"
	if got := err.Error(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCLIError_Unwrap_returnsCause(t *testing.T) {
	cause := errors.New("root cause")
	err := &errs.CLIError{Code: errs.ExitAPI, Msg: "wrapper", Cause: cause}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
}

// --- constructors ---

func TestNewAPIError_setsCode(t *testing.T) {
	err := errs.NewAPIError("bad gateway", nil)
	if err.Code != errs.ExitAPI {
		t.Errorf("expected ExitAPI(%d), got %d", errs.ExitAPI, err.Code)
	}
}

func TestNewAuthError_setsHint(t *testing.T) {
	err := errs.NewAuthError("forum")
	if err.Code != errs.ExitAuth {
		t.Errorf("expected ExitAuth(%d), got %d", errs.ExitAuth, err.Code)
	}
	if err.Hint == "" {
		t.Error("expected a non-empty hint for auth errors")
	}
}

func TestNewInputError_setsCode(t *testing.T) {
	err := errs.NewInputError("missing --id flag")
	if err.Code != errs.ExitInput {
		t.Errorf("expected ExitInput(%d), got %d", errs.ExitInput, err.Code)
	}
}

func TestNewSpecError_setsHint(t *testing.T) {
	cause := errors.New("no such file")
	err := errs.NewSpecError("file:///missing.json", cause)
	if err.Code != errs.ExitSpec {
		t.Errorf("expected ExitSpec(%d), got %d", errs.ExitSpec, err.Code)
	}
	if err.Hint == "" {
		t.Error("expected a non-empty hint for spec errors")
	}
}

func TestNewConfigError_setsCode(t *testing.T) {
	err := errs.NewConfigError("service \"x\" not found")
	if err.Code != errs.ExitConfig {
		t.Errorf("expected ExitConfig(%d), got %d", errs.ExitConfig, err.Code)
	}
}

// --- GetExitCode ---

func TestGetExitCode_fromCLIError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want errs.ExitCode
	}{
		{"api error", errs.NewAPIError("fail", nil), errs.ExitAPI},
		{"auth error", errs.NewAuthError("svc"), errs.ExitAuth},
		{"input error", errs.NewInputError("bad flag"), errs.ExitInput},
		{"spec error", errs.NewSpecError("url", nil), errs.ExitSpec},
		{"config error", errs.NewConfigError("missing"), errs.ExitConfig},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := errs.GetExitCode(tc.err); got != tc.want {
				t.Errorf("GetExitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestGetExitCode_nonCLIError_returnsUnknown(t *testing.T) {
	err := errors.New("some generic error")
	if got := errs.GetExitCode(err); got != errs.ExitUnknown {
		t.Errorf("expected ExitUnknown(%d), got %d", errs.ExitUnknown, got)
	}
}

func TestGetExitCode_wrappedCLIError(t *testing.T) {
	inner := errs.NewAPIError("upstream down", nil)
	wrapped := errors.Join(errors.New("outer"), inner)
	if got := errs.GetExitCode(wrapped); got != errs.ExitAPI {
		t.Errorf("expected ExitAPI(%d), got %d", errs.ExitAPI, got)
	}
}

package errs

import (
	"errors"
	"fmt"
)

// ExitCode maps to structured CLI exit codes (ADR-0005).
type ExitCode int

const (
	ExitOK      ExitCode = 0
	ExitAPI     ExitCode = 1 // 4xx/5xx from backend service
	ExitAuth    ExitCode = 2 // unauthenticated / token expired
	ExitInput   ExitCode = 3 // bad user input / flag validation
	ExitSpec    ExitCode = 4 // failed to load OpenAPI spec
	ExitConfig  ExitCode = 5 // misconfigured service entry
	ExitUnknown ExitCode = 127
)

// CLIError is the canonical error type for all CLI failures.
// Code drives the process exit code; Hint is printed to stderr as a fix suggestion.
type CLIError struct {
	Code  ExitCode
	Msg   string
	Hint  string
	Cause error
}

func (e *CLIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Cause)
	}
	return e.Msg
}

func (e *CLIError) Unwrap() error { return e.Cause }

// --- constructors ---

func NewAPIError(msg string, cause error) *CLIError {
	return &CLIError{Code: ExitAPI, Msg: msg, Cause: cause}
}

func NewAuthError(service string) *CLIError {
	return &CLIError{
		Code: ExitAuth,
		Msg:  fmt.Sprintf("authentication required for service %q", service),
		Hint: fmt.Sprintf("community auth login --service %s", service),
	}
}

func NewInputError(msg string) *CLIError {
	return &CLIError{Code: ExitInput, Msg: msg}
}

func NewSpecError(specURL string, cause error) *CLIError {
	return &CLIError{
		Code:  ExitSpec,
		Msg:   fmt.Sprintf("failed to load OpenAPI spec from %s", specURL),
		Hint:  "check network/file path and spec_url in config",
		Cause: cause,
	}
}

func NewConfigError(msg string) *CLIError {
	return &CLIError{Code: ExitConfig, Msg: msg}
}

// GetExitCode extracts the exit code from an error, defaulting to ExitUnknown.
func GetExitCode(err error) ExitCode {
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr.Code
	}
	return ExitUnknown
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/felix021/agentcall/internal/runner"
)

type errWriter struct {
	err error
}

func (w errWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

func TestParseRunArgsRequiresCommand(t *testing.T) {
	_, err := parseRunArgs([]string{"--timeout", "5s"}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("parseRunArgs() error = nil, want non-nil")
	}
}

func TestParseRunArgsRejectsInvalidTimeout(t *testing.T) {
	_, err := parseRunArgs([]string{"--timeout", "nope", "--", "claude"}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("parseRunArgs() error = nil, want non-nil")
	}
}

func TestParseRunArgsRejectsNonPositiveTimeout(t *testing.T) {
	for _, raw := range []string{"0s", "-5s"} {
		_, err := parseRunArgs([]string{"--timeout", raw, "--", "claude"}, &bytes.Buffer{})
		if err == nil {
			t.Fatalf("parseRunArgs(%q) error = nil, want non-nil", raw)
		}
	}
}

func TestParseRunArgsEnablesAutoTrustWhenRequested(t *testing.T) {
	got, err := parseRunArgs([]string{"--auto-trust", "--", "claude"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseRunArgs() error = %v", err)
	}
	if !got.AutoTrust {
		t.Fatal("AutoTrust = false, want true")
	}
}

func TestParseRunArgsCarriesPromptWhenProvided(t *testing.T) {
	got, err := parseRunArgs([]string{"--prompt", "review this diff", "--", "claude"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseRunArgs() error = %v", err)
	}
	if got.Prompt != "review this diff" {
		t.Fatalf("Prompt = %q, want %q", got.Prompt, "review this diff")
	}
}

func TestParseRunArgsAppliesDefaultHeartbeatSettings(t *testing.T) {
	got, err := parseRunArgs([]string{"--", "claude"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseRunArgs() error = %v", err)
	}
	if got.HeartbeatPeriod != time.Second {
		t.Fatalf("HeartbeatPeriod = %v, want %v", got.HeartbeatPeriod, time.Second)
	}
	if got.HeartbeatPeriodSet {
		t.Fatal("HeartbeatPeriodSet = true, want false")
	}
	if got.Verbose != 1 {
		t.Fatalf("Verbose = %d, want 1", got.Verbose)
	}
	if got.VerboseSet {
		t.Fatal("VerboseSet = true, want false")
	}
}

func TestParseRunArgsRejectsInvalidHeartbeatPeriod(t *testing.T) {
	_, err := parseRunArgs([]string{"--heartbeat-period", "nope", "--", "claude"}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("parseRunArgs() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "invalid duration") {
		t.Fatalf("parseRunArgs() error = %v, want invalid duration", err)
	}
}

func TestParseRunArgsRejectsNonPositiveHeartbeatPeriod(t *testing.T) {
	for _, raw := range []string{"0s", "-1s"} {
		_, err := parseRunArgs([]string{"--heartbeat-period", raw, "--", "claude"}, &bytes.Buffer{})
		if err == nil {
			t.Fatalf("parseRunArgs(%q) error = nil, want non-nil", raw)
		}
		if !strings.Contains(err.Error(), "heartbeat-period must be greater than zero") {
			t.Fatalf("parseRunArgs(%q) error = %v, want heartbeat-period validation", raw, err)
		}
	}
}

func TestParseRunArgsCarriesExplicitVerboseLevel(t *testing.T) {
	got, err := parseRunArgs([]string{"--verbose", "0", "--", "claude"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseRunArgs() error = %v", err)
	}
	if got.Verbose != 0 {
		t.Fatalf("Verbose = %d, want 0", got.Verbose)
	}
	if !got.VerboseSet {
		t.Fatal("VerboseSet = false, want true")
	}
}

func TestParseRunArgsRejectsNegativeVerboseLevel(t *testing.T) {
	_, err := parseRunArgs([]string{"--verbose", "-1", "--", "claude"}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("parseRunArgs() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "verbose must be greater than or equal to zero") {
		t.Fatalf("parseRunArgs() error = %v, want verbose validation", err)
	}
}

func TestRunCLIReportsInvalidTimeout(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"run", "--timeout", "nope", "--", "claude"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("runCLI() exitCode = %d, want 1", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("runCLI() wrote stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid duration") {
		t.Fatalf("runCLI() stderr = %q, want invalid duration error", stderr.String())
	}
}

func TestRunCLIUsageWhenSubcommandMissingOrWrong(t *testing.T) {
	tests := [][]string{
		nil,
		{"wrong"},
	}

	for _, args := range tests {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		exitCode := runCLI(args, &stdout, &stderr)

		if exitCode != 1 {
			t.Fatalf("runCLI(%v) exitCode = %d, want 1", args, exitCode)
		}
		if stdout.Len() != 0 {
			t.Fatalf("runCLI(%v) wrote stdout = %q, want empty", args, stdout.String())
		}
		if !strings.Contains(stderr.String(), "usage: agentcall run -- <command>") {
			t.Fatalf("runCLI(%v) stderr = %q, want usage", args, stderr.String())
		}
	}
}

func TestRunCLIReportsFlagParseErrorsToProvidedStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"run", "--bogus"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("runCLI() exitCode = %d, want 1", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("runCLI() wrote stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("runCLI() stderr = %q, want flag parse error", stderr.String())
	}
}

func TestRunCLIEmitsJSONAndExitCodeOnSuccess(t *testing.T) {
	origRunRunner := runRunner
	t.Cleanup(func() {
		runRunner = origRunRunner
	})

	runRunner = func(_ context.Context, in runner.RunInput) (runner.ResultEnvelope, error) {
		if got, want := in.Command, []string{"claude"}; len(got) != len(want) || got[0] != want[0] {
			t.Fatalf("run input command = %v, want %v", got, want)
		}
		if got, want := in.Prompt, "review this diff"; got != want {
			t.Fatalf("run input prompt = %q, want %q", got, want)
		}
		return runner.ResultEnvelope{
			RunID:    "latest",
			State:    runner.StatusCallbackRecv,
			Status:   string(runner.CallbackStatusNeedsInput),
			ExitCode: 2,
			Content:  "waiting",
		}, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"run", "--prompt", "review this diff", "--", "claude"}, &stdout, &stderr)

	if exitCode != 2 {
		t.Fatalf("runCLI() exitCode = %d, want 2", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("runCLI() wrote stderr = %q, want empty", stderr.String())
	}

	var got runner.ResultEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout JSON unmarshal error = %v", err)
	}
	if got.ExitCode != 2 || got.Status != string(runner.CallbackStatusNeedsInput) || got.Content != "waiting" {
		t.Fatalf("stdout JSON = %+v, want exit_code=2 status=%q content=%q", got, runner.CallbackStatusNeedsInput, "waiting")
	}
}

func TestRunCLIReturnsErrorWhenJSONWriteFails(t *testing.T) {
	origRunRunner := runRunner
	t.Cleanup(func() {
		runRunner = origRunRunner
	})

	runRunner = func(_ context.Context, _ runner.RunInput) (runner.ResultEnvelope, error) {
		return runner.ResultEnvelope{
			RunID:    "latest",
			State:    runner.StatusCallbackRecv,
			Status:   string(runner.CallbackStatusOK),
			ExitCode: 0,
		}, nil
	}

	var stderr bytes.Buffer
	exitCode := runCLI([]string{"run", "--", "claude"}, errWriter{err: errors.New("write failed")}, &stderr)

	if exitCode != 1 {
		t.Fatalf("runCLI() exitCode = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "write failed") {
		t.Fatalf("runCLI() stderr = %q, want write failure", stderr.String())
	}
}

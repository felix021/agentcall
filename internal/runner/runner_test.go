package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/felix021/agentcall/internal/callback"
)

func TestMain(m *testing.M) {
	bootstrapRoot := "/tmp/go1.24-bootstrap/go"
	bootstrapBin := filepath.Join(bootstrapRoot, "bin")

	if info, err := os.Stat(filepath.Join(bootstrapBin, "go")); err == nil && !info.IsDir() {
		_ = os.Setenv("GOROOT", bootstrapRoot)
		_ = os.Setenv("PATH", bootstrapBin+string(os.PathListSeparator)+os.Getenv("PATH"))
		_ = os.Setenv("GOPROXY", "https://goproxy.cn,direct")
		_ = os.Setenv("GOSUMDB", "off")
	}

	os.Exit(m.Run())
}

func TestRunReturnsSuccessEnvelopeFromCallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: fakeAgentCommand(t, "success"),
		Prompt:  "review this diff",
		Timeout: 5 * time.Second,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "ok" || res.ExitCode != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunReturnsNeedsInputEnvelope(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: fakeAgentCommand(t, "needs-input"),
		Prompt:  "review this diff",
		Timeout: 5 * time.Second,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "needs_input" || res.ExitCode != 2 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunMarksCallbackMissingWhenProcessExitsWithoutPayload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: fakeAgentCommand(t, "no-callback"),
		Prompt:  "review this diff",
		Timeout: 2 * time.Second,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "callback_missing" || res.ExitCode != 4 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunReturnsTimedOutEnvelopeWhenProcessBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: []string{"sh", "-c", "sleep 10"},
		Prompt:  "review this diff",
		Timeout: 500 * time.Millisecond,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "timed_out" || res.ExitCode != 3 {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(res.Error, "timeout") {
		t.Fatalf("error = %q, want timeout message", res.Error)
	}
}

func TestRunAutoTrustConfirmsRecognizedPrompt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	artifactsDir := t.TempDir()
	res, err := Run(ctx, RunInput{
		Command:      fakeAgentCommand(t, "trust-then-success"),
		Prompt:       "review this diff",
		Timeout:      5 * time.Second,
		ArtifactsDir: artifactsDir,
		StatusFile:   filepath.Join(artifactsDir, "status.json"),
		AutoTrust:    true,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "ok" || res.ExitCode != 0 {
		t.Fatalf("result = %+v", res)
	}

	transcript, err := os.ReadFile(filepath.Join(artifactsDir, "transcript.log"))
	if err != nil {
		t.Fatalf("ReadFile(transcript.log) error = %v", err)
	}
	if !strings.Contains(string(transcript), "auto-trust confirmed") {
		t.Fatalf("transcript missing auto-trust marker: %q", string(transcript))
	}
}

func TestRunLeavesTrustPromptBlockedWithoutAutoTrust(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: fakeAgentCommand(t, "trust-then-success"),
		Prompt:  "review this diff",
		Timeout: 750 * time.Millisecond,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "timed_out" || res.ExitCode != 3 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunSubmitsPromptAfterInjection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: fakeAgentCommand(t, "submit-then-success"),
		Prompt:  "review this diff",
		Timeout: 5 * time.Second,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "ok" || res.ExitCode != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunEmitsHeartbeatJSONToStderr(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	res, err := Run(ctx, RunInput{
		Command:            fakeAgentCommand(t, "slow-success"),
		Prompt:             "review this diff",
		Timeout:            5 * time.Second,
		HeartbeatPeriod:    100 * time.Millisecond,
		HeartbeatPeriodSet: true,
		Verbose:            1,
		VerboseSet:         true,
	}, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("result = %+v", res)
	}
	lines := decodeHeartbeatLines(t, stderr.Bytes())
	if len(lines) < 30 {
		t.Fatalf("heartbeat count = %d, want at least 30 for a deterministic multi-tick window", len(lines))
	}
	for _, line := range lines {
		if line["type"] != "heartbeat" {
			t.Fatalf("heartbeat = %#v", line)
		}
	}
}

func TestRunSuppressesHeartbeatWhenVerboseZero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	_, err := Run(ctx, RunInput{
		Command:            fakeAgentCommand(t, "slow-success"),
		Prompt:             "review this diff",
		Timeout:            5 * time.Second,
		HeartbeatPeriod:    100 * time.Millisecond,
		HeartbeatPeriodSet: true,
		Verbose:            0,
		VerboseSet:         true,
	}, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunVerboseTwoIncludesDiagnosticHeartbeatFields(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	_, err := Run(ctx, RunInput{
		Command:            fakeAgentCommand(t, "slow-success"),
		Prompt:             "review this diff",
		Timeout:            5 * time.Second,
		HeartbeatPeriod:    100 * time.Millisecond,
		HeartbeatPeriodSet: true,
		Verbose:            2,
		VerboseSet:         true,
	}, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	lines := decodeHeartbeatLines(t, stderr.Bytes())
	foundVerboseField := false
	for _, line := range lines {
		if _, ok := line["prompt_pasted"]; ok {
			foundVerboseField = true
			break
		}
	}
	if !foundVerboseField {
		t.Fatalf("heartbeat lines = %#v, want verbose diagnostic fields", lines)
	}
}

func TestOutcomeFromExitPrefersBufferedCallback(t *testing.T) {
	wait := sessionWait{}
	results := make(chan callback.Result, 1)
	results <- callback.Result{Payload: CallbackPayload{
		Token:       "token-1",
		Status:      "ok",
		ContentType: "text/plain",
		Content:     "done",
	}}

	out, consumed := outcomeFromExit(wait, results)
	if !consumed {
		t.Fatal("consumed = false, want true")
	}
	if out.State != StatusCallbackRecv || out.Status != "ok" || out.ExitCode != 0 {
		t.Fatalf("result = %+v", out)
	}
}

func TestRandomTokenReturnsErrorWhenCryptoReadFails(t *testing.T) {
	prev := randomTokenRead
	randomTokenRead = func([]byte) (int, error) {
		return 0, errors.New("entropy source unavailable")
	}
	t.Cleanup(func() {
		randomTokenRead = prev
	})

	_, err := randomToken()
	if err == nil {
		t.Fatal("randomToken() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "entropy source unavailable") {
		t.Fatalf("randomToken() error = %v, want wrapped read error", err)
	}
}

func TestBuildCommandDoesNotRewriteGoBinaryForNonFakeAgent(t *testing.T) {
	got := buildCommand([]string{"go", "version"})

	if got[0] != "go" {
		t.Fatalf("argv[0] = %q, want %q", got[0], "go")
	}
	if len(got) != 2 || got[1] != "version" {
		t.Fatalf("argv = %#v, want original command", got)
	}
}

func TestBuildCommandTreatsFakeAgentLikeAnyOtherCommand(t *testing.T) {
	command := []string{"go", "run", "./internal/fakeagent", "--mode", "success"}

	got := buildCommand(command)
	want := append([]string{}, command...)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildCommand() = %#v, want %#v", got, want)
	}
}

func TestDetectTrustPromptMatchesClaudeStyleSafetyDialog(t *testing.T) {
	text := "Quick safety check: Is this a project you created or one you trust?\n1. Yes, I trust this folder\n2. No, exit\nEnter to confirm\n"
	if !detectTrustPrompt(text) {
		t.Fatal("detectTrustPrompt() = false, want true")
	}
}

func TestDetectTrustPromptMatchesAnsiRenderedSafetyDialog(t *testing.T) {
	text := "\x1b[1CQuick\x1b[1Csafety\x1b[1Ccheck:\x1b[1CIs\x1b[1Cthis\x1b[1Ca\x1b[1Cproject\x1b[1Cyou\x1b[1Ccreated\x1b[1Cor\x1b[1Cone\x1b[1Cyou\x1b[1Ctrust?\r\n\x1b[1C❯\x1b[1C1.\x1b[1CYes,\x1b[1CI\x1b[1Ctrust\x1b[1Cthis\x1b[1Cfolder\r\n\x1b[3C2.\x1b[1CNo,\x1b[1Cexit\r\n\x1b[1CEnter\x1b[1Cto\x1b[1Cconfirm\r\n"
	if !detectTrustPrompt(text) {
		t.Fatal("detectTrustPrompt() = false, want true")
	}
}

func fakeAgentCommand(t *testing.T, mode string) []string {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("exec.LookPath(go) error = %v", err)
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	return []string{goBin, "run", filepath.Join(repoRoot, "internal", "fakeagent"), "--mode", mode}
}

func decodeHeartbeatLines(t *testing.T, raw []byte) []map[string]any {
	t.Helper()

	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}

	var out []map[string]any
	for _, line := range bytes.Split(raw, []byte("\n")) {
		var decoded map[string]any
		if err := json.Unmarshal(line, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", string(line), err)
		}
		out = append(out, decoded)
	}
	return out
}

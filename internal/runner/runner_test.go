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
	"strings"
	"testing"
	"time"

	"github.com/felix021/agentcall/internal/callback"
)

const (
	fakeAgentContextTimeout = 20 * time.Second
	fakeAgentRunTimeout     = 10 * time.Second
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
	ctx, cancel := context.WithTimeout(context.Background(), fakeAgentContextTimeout)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: fakeAgentCommand(t, "success"),
		Prompt:  "review this diff",
		Timeout: fakeAgentRunTimeout,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "ok" || res.ExitCode != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunReturnsNeedsInputEnvelope(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), fakeAgentContextTimeout)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: fakeAgentCommand(t, "needs-input"),
		Prompt:  "review this diff",
		Timeout: fakeAgentRunTimeout,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "needs_input" || res.ExitCode != 2 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunMarksCallbackMissingWhenProcessExitsWithoutPayload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), fakeAgentContextTimeout)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: fakeAgentCommand(t, "no-callback"),
		Prompt:  "review this diff",
		Timeout: 4 * time.Second,
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
	ctx, cancel := context.WithTimeout(context.Background(), fakeAgentContextTimeout)
	defer cancel()

	artifactsDir := t.TempDir()
	res, err := Run(ctx, RunInput{
		Command:      fakeAgentCommand(t, "trust-then-success"),
		Prompt:       "review this diff",
		Timeout:      fakeAgentRunTimeout,
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
	ctx, cancel := context.WithTimeout(context.Background(), fakeAgentContextTimeout)
	defer cancel()

	res, err := Run(ctx, RunInput{
		Command: fakeAgentCommand(t, "submit-then-success"),
		Prompt:  "review this diff",
		Timeout: fakeAgentRunTimeout,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Status != "ok" || res.ExitCode != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunEmitsHeartbeatJSONToStderr(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), fakeAgentContextTimeout)
	defer cancel()

	var stderr bytes.Buffer
	res, err := Run(ctx, RunInput{
		Command:            fakeAgentCommand(t, "slow-success"),
		Prompt:             "review this diff",
		Timeout:            fakeAgentRunTimeout,
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

func TestRunEmitsHeartbeatsWhenPeriodIsBelowControlTick(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), fakeAgentContextTimeout)
	defer cancel()

	var stderr bytes.Buffer
	res, err := Run(ctx, RunInput{
		Command:            fakeAgentCommand(t, "slow-success"),
		Prompt:             "review this diff",
		Timeout:            fakeAgentRunTimeout,
		HeartbeatPeriod:    50 * time.Millisecond,
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
	if len(lines) < 45 {
		t.Fatalf("heartbeat count = %d, want at least 45 when heartbeat period is below control tick", len(lines))
	}

	for i, line := range lines {
		if got := int(line["seq"].(float64)); got != i+1 {
			t.Fatalf("heartbeat[%d].seq = %d, want %d", i, got, i+1)
		}
	}
}

func TestRunSuppressesHeartbeatWhenVerboseZero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), fakeAgentContextTimeout)
	defer cancel()

	var stderr bytes.Buffer
	_, err := Run(ctx, RunInput{
		Command:            fakeAgentCommand(t, "slow-success"),
		Prompt:             "review this diff",
		Timeout:            fakeAgentRunTimeout,
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
	ctx, cancel := context.WithTimeout(context.Background(), fakeAgentContextTimeout)
	defer cancel()

	var stderr bytes.Buffer
	_, err := Run(ctx, RunInput{
		Command:            fakeAgentCommand(t, "slow-success"),
		Prompt:             "review this diff",
		Timeout:            fakeAgentRunTimeout,
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

func TestRefreshRunnerStateRecomputesAwaitingInputBeforeHeartbeatEmit(t *testing.T) {
	now := time.Date(2026, time.May, 26, 12, 0, 0, 0, time.UTC)
	detector := NewDetector(350*time.Millisecond, []string{"clarification", "continue?", "proceed?"})
	detector.Observe("Need clarification about the target branch?", now.Add(-time.Second))

	state, lastSnapshot, promptActivitySeen, screenChanged := refreshRunnerState(
		"Need clarification about the target branch?",
		now,
		detector,
		"Need clarification about the target branch?",
		false,
		false,
		time.Time{},
		false,
		false,
	)

	if state != StatusAwaitingInput {
		t.Fatalf("state = %q, want %q", state, StatusAwaitingInput)
	}
	if lastSnapshot != "Need clarification about the target branch?" {
		t.Fatalf("lastSnapshot = %q, want unchanged snapshot", lastSnapshot)
	}
	if promptActivitySeen {
		t.Fatal("promptActivitySeen = true, want false")
	}
	if screenChanged {
		t.Fatal("screenChanged = true, want false for unchanged snapshot")
	}
}

func TestRefreshRunnerStateMarksScreenChangedAndPromptActivityOnNewSnapshot(t *testing.T) {
	now := time.Date(2026, time.May, 26, 12, 0, 0, 0, time.UTC)
	promptPastedAt := now.Add(-2 * time.Second)
	detector := NewDetector(350*time.Millisecond, []string{"clarification", "continue?", "proceed?"})

	state, lastSnapshot, promptActivitySeen, screenChanged := refreshRunnerState(
		"fakeagent: slow phase 1",
		now,
		detector,
		"",
		true,
		false,
		promptPastedAt,
		false,
		false,
	)

	if state != StatusActive {
		t.Fatalf("state = %q, want %q", state, StatusActive)
	}
	if lastSnapshot != "fakeagent: slow phase 1" {
		t.Fatalf("lastSnapshot = %q, want refreshed snapshot", lastSnapshot)
	}
	if !promptActivitySeen {
		t.Fatal("promptActivitySeen = false, want true after post-paste output")
	}
	if !screenChanged {
		t.Fatal("screenChanged = false, want true for changed snapshot")
	}
}

func fakeAgentCommand(t *testing.T, mode string) []string {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("exec.LookPath(go) error = %v", err)
	}

	sourcePath := filepath.Join(t.TempDir(), "fakeagent_testhelper.go")
	if err := os.WriteFile(sourcePath, []byte(testFakeAgentSource), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", sourcePath, err)
	}

	return []string{goBin, "run", sourcePath, "--mode", mode}
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

const testFakeAgentSource = `package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var callbackPromptPattern = regexp.MustCompile("POST JSON to (http://[^ ]+) using exactly one JSON object with required fields like \\{\"token\":\"([^\"]+)\"")

type callbackContract struct {
	URL   string
	Token string
}

const (
	bracketedPasteStart = "\x1b[200~"
	bracketedPasteEnd   = "\x1b[201~"
)

func main() {
	mode := flag.String("mode", "success", "success|needs-input|no-callback")
	callbackURL := flag.String("callback-url", "", "callback URL")
	token := flag.String("token", "", "token")
	flag.Parse()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("fakeagent: booting\r\n")
	for i := 0; i < 3; i++ {
		fmt.Printf("fakeagent: working %d\r", i)
		time.Sleep(150 * time.Millisecond)
	}
	fmt.Print("\r\n")

	switch *mode {
	case "trust-then-success":
		fmt.Println("Quick safety check: Is this a project you created or one you trust?")
		fmt.Println("1. Yes, I trust this folder")
		fmt.Println("2. No, exit")
		fmt.Println("Enter to confirm")
		input, err := waitForInputOnce(reader)
		if isTrustConfirmationInput(input, err) {
			fmt.Println("Trust confirmed")
			resolveCallbackContract(reader, callbackURL, token)
			fmt.Println("Prompt received")
			send(*callbackURL, *token, "ok", "done")
			return
		}
		blockForever()
	case "needs-input":
		resolveCallbackContract(reader, callbackURL, token)
		fmt.Println("Prompt received")
		fmt.Println("Need clarification about the target branch?")
		send(*callbackURL, *token, "needs_input", "I need clarification about the target branch.")
		fmt.Println("Waiting for input...")
		input, err := waitForInputOnce(reader)
		if shouldBlockForInput(input, err) {
			blockForever()
		}
	case "success":
		resolveCallbackContract(reader, callbackURL, token)
		fmt.Println("Prompt received")
		fmt.Println("Finalizing result")
		send(*callbackURL, *token, "ok", "done")
	case "slow-success":
		resolveCallbackContract(reader, callbackURL, token)
		fmt.Println("Prompt received")
		for i := 0; i < 12; i++ {
			fmt.Printf("fakeagent: slow phase %d\r\n", i)
			time.Sleep(200 * time.Millisecond)
		}
		fmt.Println("Finalizing result")
		send(*callbackURL, *token, "ok", "done")
	case "submit-then-success":
		resolveCallbackContract(reader, callbackURL, token)
		fmt.Println("Prompt received")
		input, err := waitForInputOnce(reader)
		if isPromptSubmitInput(input, err) {
			fmt.Println("Prompt submitted")
			send(*callbackURL, *token, "ok", "done")
			return
		}
		blockForever()
	case "no-callback":
		resolveCallbackContract(reader, callbackURL, token)
		fmt.Println("Prompt received")
		fmt.Println("Exiting without callback")
	}
}

func send(url, token, status, content string) {
	body, err := buildCallbackBody(token, status, content)
	if err != nil {
		return
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func waitForInputOnce(r *bufio.Reader) (string, error) {
	var buf strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			if err == io.EOF && buf.Len() > 0 {
				return buf.String(), nil
			}
			return buf.String(), err
		}
		buf.WriteByte(b)
		if b == '\r' || b == '\n' {
			return buf.String(), nil
		}
	}
}

func shouldBlockForInput(input string, err error) bool {
	return err != nil && !(err == io.EOF && input != "")
}

func isTrustConfirmationInput(input string, err error) bool {
	if err != nil && !(err == io.EOF && input != "") {
		return false
	}
	switch strings.TrimSpace(input) {
	case "", "1":
		return true
	default:
		return false
	}
}

func isPromptSubmitInput(input string, err error) bool {
	return isTrustConfirmationInput(input, err)
}

func blockForever() {
	select {}
}

func buildCallbackBody(token, status, content string) ([]byte, error) {
	return json.Marshal(struct {
		Token       string ` + "`json:\"token\"`" + `
		Status      string ` + "`json:\"status\"`" + `
		ContentType string ` + "`json:\"content_type\"`" + `
		Content     string ` + "`json:\"content\"`" + `
	}{
		Token:       token,
		Status:      status,
		ContentType: "text/plain",
		Content:     content,
	})
}

func callbackContractFromPrompt(prompt string) (callbackContract, bool) {
	match := callbackPromptPattern.FindStringSubmatch(stripBracketedPasteMarkers(prompt))
	if len(match) != 3 {
		return callbackContract{}, false
	}
	return callbackContract{URL: match[1], Token: match[2]}, true
}

func callbackContractFromInput(r *bufio.Reader) (callbackContract, bool, error) {
	var buf strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				if contract, ok := callbackContractFromPrompt(buf.String()); ok {
					return contract, true, nil
				}
				return callbackContract{}, false, err
			}
			return callbackContract{}, false, err
		}

		buf.WriteByte(b)
		if contract, ok := callbackContractFromPrompt(buf.String()); ok && promptBufferComplete(buf.String()) {
			return contract, true, nil
		}
	}
}

func promptBufferComplete(buf string) bool {
	if strings.Contains(buf, bracketedPasteStart) {
		return strings.Contains(buf, bracketedPasteEnd)
	}
	return true
}

func stripBracketedPasteMarkers(s string) string {
	s = strings.ReplaceAll(s, bracketedPasteStart, "")
	s = strings.ReplaceAll(s, bracketedPasteEnd, "")
	return s
}

func resolveCallbackContract(reader *bufio.Reader, callbackURL, token *string) {
	contract, ok, err := callbackContractFromInput(reader)
	if err != nil || !ok {
		return
	}
	if *callbackURL == "" {
		*callbackURL = contract.URL
	}
	if *token == "" {
		*token = contract.Token
	}
}
`

package main

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestBuildCallbackBodyEscapesContent(t *testing.T) {
	body, err := buildCallbackBody("tok-1", "needs_input", "quote: \"hi\"\nline2")
	if err != nil {
		t.Fatalf("buildCallbackBody() error = %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	want := map[string]string{
		"token":        "tok-1",
		"status":       "needs_input",
		"content_type": "text/plain",
		"content":      "quote: \"hi\"\nline2",
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("field %q = %q, want %q", key, got[key], value)
		}
	}
}

func TestWaitForInputOnceReturnsEOFWhenInputClosed(t *testing.T) {
	_, err := waitForInputOnce(bufio.NewReader(strings.NewReader("")))
	if err != io.EOF {
		t.Fatalf("waitForInputOnce() error = %v, want %v", err, io.EOF)
	}
}

func TestWaitForInputOnceStopsAtCarriageReturn(t *testing.T) {
	input, err := waitForInputOnce(bufio.NewReader(strings.NewReader("1\rrest")))
	if err != nil {
		t.Fatalf("waitForInputOnce() error = %v", err)
	}
	if input != "1\r" {
		t.Fatalf("waitForInputOnce() input = %q, want %q", input, "1\\r")
	}
}

func TestShouldBlockForInputIgnoresPartialLineEOF(t *testing.T) {
	if shouldBlockForInput("target-branch", io.EOF) {
		t.Fatal("shouldBlockForInput() = true, want false for partial input plus EOF")
	}
}

func TestIsTrustConfirmationInputAcceptsDefaultAndExplicitChoice(t *testing.T) {
	for _, input := range []string{"\n", "1\n"} {
		if !isTrustConfirmationInput(input, nil) {
			t.Fatalf("isTrustConfirmationInput(%q) = false, want true", input)
		}
	}
}

func TestIsTrustConfirmationInputRejectsOtherChoices(t *testing.T) {
	if isTrustConfirmationInput("2\n", nil) {
		t.Fatal("isTrustConfirmationInput() = true, want false")
	}
}

func TestCallbackContractFromPromptExtractsURLAndToken(t *testing.T) {
	prompt := strings.Join([]string{
		"You are running inside a local PTY automation wrapper.",
		"POST JSON to http://127.0.0.1:4321/callback with token tok-123.",
		"User task:",
		"review this diff",
	}, "\n")

	got, ok := callbackContractFromPrompt(prompt)
	if !ok {
		t.Fatal("callbackContractFromPrompt() ok = false, want true")
	}
	if got.URL != "http://127.0.0.1:4321/callback" {
		t.Fatalf("URL = %q, want %q", got.URL, "http://127.0.0.1:4321/callback")
	}
	if got.Token != "tok-123" {
		t.Fatalf("Token = %q, want %q", got.Token, "tok-123")
	}
}

func TestCallbackContractFromPromptRejectsMissingContract(t *testing.T) {
	if _, ok := callbackContractFromPrompt("review this diff"); ok {
		t.Fatal("callbackContractFromPrompt() ok = true, want false")
	}
}

func TestCallbackContractFromInputExtractsContractFromMultilinePrompt(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(strings.Join([]string{
		"You are running inside a local PTY automation wrapper.",
		"Keep normal progress in the terminal UI.",
		"POST JSON to http://127.0.0.1:4321/callback with token tok-123.",
		"User task:",
		"review this diff",
		"",
	}, "\n")))

	got, ok, err := callbackContractFromInput(reader)
	if err != nil {
		t.Fatalf("callbackContractFromInput() error = %v", err)
	}
	if !ok {
		t.Fatal("callbackContractFromInput() ok = false, want true")
	}
	if got.URL != "http://127.0.0.1:4321/callback" || got.Token != "tok-123" {
		t.Fatalf("contract = %+v", got)
	}
}

func TestCallbackContractFromInputExtractsContractFromBracketedPaste(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(
		"\x1b[200~You are running inside a local PTY automation wrapper.\n" +
			"POST JSON to http://127.0.0.1:4321/callback with token tok-123.\n" +
			"User task:\nreview this diff\n\x1b[201~\n",
	))

	got, ok, err := callbackContractFromInput(reader)
	if err != nil {
		t.Fatalf("callbackContractFromInput() error = %v", err)
	}
	if !ok {
		t.Fatal("callbackContractFromInput() ok = false, want true")
	}
	if got.URL != "http://127.0.0.1:4321/callback" || got.Token != "tok-123" {
		t.Fatalf("contract = %+v", got)
	}
}

func TestCallbackContractFromInputLeavesSubmitNewlineUnread(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(
		"\x1b[200~You are running inside a local PTY automation wrapper.\n" +
			"POST JSON to http://127.0.0.1:4321/callback with token tok-123.\n" +
			"User task:\nreview this diff\x1b[201~\n",
	))

	got, ok, err := callbackContractFromInput(reader)
	if err != nil {
		t.Fatalf("callbackContractFromInput() error = %v", err)
	}
	if !ok {
		t.Fatal("callbackContractFromInput() ok = false, want true")
	}
	if got.URL != "http://127.0.0.1:4321/callback" || got.Token != "tok-123" {
		t.Fatalf("contract = %+v", got)
	}

	input, err := waitForInputOnce(reader)
	if err != nil {
		t.Fatalf("waitForInputOnce() error = %v", err)
	}
	if input != "\n" {
		t.Fatalf("waitForInputOnce() input = %q, want %q", input, "\\n")
	}
}

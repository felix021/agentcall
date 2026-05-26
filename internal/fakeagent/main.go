package main

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

var callbackContractPattern = regexp.MustCompile(`POST JSON to (http://[^\s]+) with token ([^.\s]+)\.`)

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
		Token       string `json:"token"`
		Status      string `json:"status"`
		ContentType string `json:"content_type"`
		Content     string `json:"content"`
	}{
		Token:       token,
		Status:      status,
		ContentType: "text/plain",
		Content:     content,
	})
}

func callbackContractFromPrompt(prompt string) (callbackContract, bool) {
	match := callbackContractPattern.FindStringSubmatch(stripBracketedPasteMarkers(prompt))
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

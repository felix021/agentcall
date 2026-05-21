package runner

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	autoTrustMarker       = "\n[agentcall] auto-trust confirmed\n"
	promptInjectedMarker  = "\n[agentcall] prompt injected\n"
	promptSubmittedMarker = "\n[agentcall] prompt submitted\n"
)

var (
	cursorForwardPattern = regexp.MustCompile(`\x1b\[(\d+)C`)
	ansiPattern          = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
)

func detectTrustPrompt(transcript string) bool {
	lower := strings.ToLower(normalizeTerminalText(transcript))
	if strings.Contains(lower, "yes, i trust this folder") && strings.Contains(lower, "enter to confirm") {
		if strings.Contains(lower, "do you trust this folder") {
			return true
		}
		if strings.Contains(lower, "quick safety check") {
			return true
		}
		if strings.Contains(lower, "project you created or one you trust") {
			return true
		}
	}
	return false
}

func normalizeTerminalText(transcript string) string {
	expanded := cursorForwardPattern.ReplaceAllStringFunc(transcript, func(match string) string {
		submatch := cursorForwardPattern.FindStringSubmatch(match)
		if len(submatch) != 2 {
			return " "
		}
		count, err := strconv.Atoi(submatch[1])
		if err != nil || count <= 0 {
			return " "
		}
		return strings.Repeat(" ", count)
	})
	cleaned := ansiPattern.ReplaceAllString(expanded, "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")
	return strings.Join(strings.Fields(cleaned), " ")
}

func wrapBracketedPaste(text string) string {
	return "\x1b[200~" + text + "\x1b[201~"
}

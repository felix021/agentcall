package runner

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	autoTrustMarker       = "\n[agentcall] auto-trust confirmed\n"
	autoUpdateSkipMarker  = "\n[agentcall] auto-skipped codex update prompt\n"
	promptInjectedMarker  = "\n[agentcall] prompt injected\n"
	promptSubmittedMarker = "\n[agentcall] prompt submitted\n"
)

var (
	cursorForwardPattern = regexp.MustCompile(`\x1b\[(\d+)C`)
	ansiPattern          = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
)

type interactionBlock struct {
	State string
	Error string
}

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
	cleaned := strings.ReplaceAll(stripANSI(transcript), "\r", "\n")
	return strings.Join(strings.Fields(cleaned), " ")
}

func stripANSI(transcript string) string {
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
	return ansiPattern.ReplaceAllString(expanded, "")
}

func cleanTranscriptText(transcript string) string {
	cleaned := strings.ReplaceAll(stripANSI(transcript), "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")

	lines := strings.Split(cleaned, "\n")
	out := make([]string, 0, len(lines))
	last := ""
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" || isSpinnerFrame(line) {
			continue
		}
		if line == last {
			continue
		}
		out = append(out, line)
		last = line
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

func isSpinnerFrame(line string) bool {
	if line == "" || len(line) > 4 {
		return false
	}
	for _, r := range line {
		if !strings.ContainsRune(`|/-\◐◓◑◒⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`, r) {
			return false
		}
	}
	return true
}

func transcriptHint(transcript string, maxLines int) string {
	lines := recentCleanLines(transcript, maxLines)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, " | ")
}

func detectApprovalPrompt(command []string, transcript string) *interactionBlock {
	tool, ok := commandTool(command)
	if !ok {
		return nil
	}
	recent := strings.Join(recentCleanLines(transcript, 6), "\n")
	lower := strings.ToLower(recent)
	if !strings.Contains(lower, "action required") && !strings.Contains(lower, "approve") && !strings.Contains(lower, "permission") {
		return nil
	}

	switch tool {
	case "codex":
		if strings.Contains(lower, "action required") && (strings.Contains(lower, "approve running") || strings.Contains(lower, "run command") || strings.Contains(lower, "approval")) {
			hint := transcriptHint(transcript, 3)
			if hasCodexApprovalBypass(command) {
				return &interactionBlock{
					State: StatusApprovalRequired,
					Error: "detected Codex approval prompt despite non-interactive approval flags" + formatTranscriptHint(hint),
				}
			}
			return &interactionBlock{
				State: StatusApprovalRequired,
				Error: "detected Codex approval prompt; rerun with --dangerously-bypass-approvals-and-sandbox" + formatTranscriptHint(hint),
			}
		}
	case "claude":
		hasPermissionTerm := strings.Contains(lower, "permission") ||
			strings.Contains(lower, "allow") ||
			strings.Contains(lower, "deny") ||
			strings.Contains(lower, "permission to")
		hasConfirmationTerm := strings.Contains(lower, "confirm") ||
			strings.Contains(lower, "enter to") ||
			strings.Contains(lower, "do you want")
		isPermissionPrompt := (strings.Contains(lower, "action required") && (hasPermissionTerm || hasConfirmationTerm)) ||
			(strings.Contains(lower, "permission") &&
				(strings.Contains(lower, "allow") ||
					strings.Contains(lower, "deny") ||
					strings.Contains(lower, "confirm") ||
					strings.Contains(lower, "enter to") ||
					strings.Contains(lower, "do you want") ||
					strings.Contains(lower, "permission to"))) ||
			(strings.Contains(lower, "proceed") && (strings.Contains(lower, "?") || strings.Contains(lower, "confirm")))
		if isPermissionPrompt {
			hint := transcriptHint(transcript, 3)
			if hasClaudeApprovalBypass(command) {
				return &interactionBlock{
					State: StatusApprovalRequired,
					Error: "detected Claude permission prompt despite non-interactive permission flags" + formatTranscriptHint(hint),
				}
			}
			return &interactionBlock{
				State: StatusApprovalRequired,
				Error: "detected Claude permission prompt; rerun with --dangerously-skip-permissions" + formatTranscriptHint(hint),
			}
		}
	}

	return nil
}

func detectCodexStartupUpdatePrompt(command []string, transcript string) bool {
	tool, ok := commandTool(command)
	if !ok || tool != "codex" {
		return false
	}
	lines := recentCleanLines(transcript, 4)
	if len(lines) == 0 {
		return false
	}
	hasUpdate := false
	hasSkip := false
	hasChoice := false
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "update") || strings.Contains(lower, "codex") {
			hasUpdate = true
		}
		if strings.Contains(lower, "skip") {
			hasSkip = true
		}
		if strings.Contains(lower, "arrow keys") || strings.Contains(lower, "press enter") || strings.Contains(lower, "confirm") || strings.Contains(lower, "update now") {
			hasChoice = true
		}
	}
	return hasUpdate && hasSkip && hasChoice
}

func detectRestartRequired(command []string, transcript string) *interactionBlock {
	tool, ok := commandTool(command)
	if !ok {
		return nil
	}
	hint := transcriptHint(transcript, 3)
	lower := strings.ToLower(normalizeTerminalText(transcript))
	if tool == "codex" && strings.Contains(lower, "please restart codex.") {
		return &interactionBlock{
			State: StatusRestartRequired,
			Error: "Codex requested a restart before sending a result" + formatTranscriptHint(hint),
		}
	}
	if tool == "claude" && strings.Contains(lower, "please restart claude") {
		return &interactionBlock{
			State: StatusRestartRequired,
			Error: "Claude requested a restart before sending a result" + formatTranscriptHint(hint),
		}
	}
	return nil
}

func recentCleanLines(transcript string, maxLines int) []string {
	lines := strings.Split(strings.TrimSpace(cleanTranscriptText(transcript)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}

func formatTranscriptHint(hint string) string {
	if hint == "" {
		return ""
	}
	return ": " + hint
}

func commandTool(command []string) (string, bool) {
	if len(command) == 0 {
		return "", false
	}
	name := strings.ToLower(filepath.Base(command[0]))
	name = strings.TrimSuffix(name, ".exe")
	switch name {
	case "claude", "codex":
		return name, true
	default:
		return "", false
	}
}

func hasClaudeApprovalBypass(command []string) bool {
	for _, arg := range command[1:] {
		if arg == "--dangerously-skip-permissions" {
			return true
		}
	}
	return false
}

func hasCodexApprovalBypass(command []string) bool {
	for i := 1; i < len(command); i++ {
		arg := command[i]
		if arg == "--dangerously-bypass-approvals-and-sandbox" {
			return true
		}
		if arg == "--approval-mode" && i+1 < len(command) && command[i+1] == "never" {
			return true
		}
		if arg == "-a" && i+1 < len(command) && command[i+1] == "never" {
			return true
		}
	}
	return false
}

func wrapBracketedPaste(text string) string {
	return "\x1b[200~" + text + "\x1b[201~"
}

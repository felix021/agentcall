package prompt

import "fmt"

func Build(callbackURL, token, userPrompt string) string {
	return fmt.Sprintf(
		"You are running inside a local PTY automation wrapper. Keep normal progress and reasoning in the terminal UI. Keep any terminal-side final summary brief because the callback payload is authoritative. Always invoke the localhost callback when you stop making forward progress for the current turn, including success, clarification needed, confirmation needed, refusal, or error. POST JSON to %s with token %s. Use statuses: ok, needs_input, error, refused. User task: %s",
		callbackURL,
		token,
		userPrompt,
	)
}

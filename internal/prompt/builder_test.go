package prompt

import (
	"strings"
	"testing"
)

func TestBuildIncludesCallbackContract(t *testing.T) {
	out := Build("http://127.0.0.1:4321/callback", "tok-1", "review this diff")
	for _, needle := range []string{
		"http://127.0.0.1:4321/callback",
		"tok-1",
		"Always invoke the localhost callback when you stop making forward progress",
		"Keep any terminal-side final summary brief because the callback payload is authoritative",
		`{"token":"tok-1","status":"ok","content_type":"text/plain","content":"<final response text>"}`,
		"needs_input",
		"review this diff",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("Build() missing %q", needle)
		}
	}
}

func TestBuildReturnsSingleLinePromptForPTYInjection(t *testing.T) {
	out := Build("http://127.0.0.1:4321/callback", "tok-1", "review this diff")
	if strings.Contains(out, "\n") {
		t.Fatalf("Build() = %q, want single-line prompt", out)
	}
}

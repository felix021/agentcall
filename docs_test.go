package agentcall_test

import (
	"os"
	"strings"
	"testing"
)

func TestReadmesDocumentSkillInstallPathsForCodexAndClaude(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"README.md", "README.en.md"} {
		content := readFile(t, path)
		assertContains(t, path, content, "~/.agents/skills/agentcall/SKILL.md")
		assertContains(t, path, content, "~/.claude/skills/agentcall/SKILL.md")
	}
}

func TestReadmesDocumentHeartbeatAndStdIOSplitContract(t *testing.T) {
	t.Parallel()

	type readmeContract struct {
		path  string
		wants []string
	}

	for _, tc := range []readmeContract{
		{
			path: "README.md",
			wants: []string{
				"`--heartbeat-period`",
				"`--verbose`",
				"`--verbose=0`",
				"活跃时间足够长",
				"`stderr`",
				"`stdout` 始终保留给最终唯一一条结果 JSON envelope",
			},
		},
		{
			path: "README.en.md",
			wants: []string{
				"`--heartbeat-period`",
				"`--verbose`",
				"`--verbose=0`",
				"active long enough",
				"`stderr`",
				"`stdout` remains reserved for the single final-result JSON envelope",
			},
		},
	} {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			content := readFile(t, tc.path)
			for _, want := range tc.wants {
				assertContains(t, tc.path, content, want)
			}
		})
	}
}

func TestSkillExplainsYoloFlagsForClaudeAndCodex(t *testing.T) {
	t.Parallel()

	content := readFile(t, "skills/agentcall/SKILL.md")
	assertContains(t, "skills/agentcall/SKILL.md", content, "yolo")
	assertContains(t, "skills/agentcall/SKILL.md", content, "--dangerously-skip-permissions")
	assertContains(t, "skills/agentcall/SKILL.md", content, "--dangerously-bypass-approvals-and-sandbox")
}

func TestSkillDocumentsWorktreeAndEnvironmentContext(t *testing.T) {
	t.Parallel()

	content := readFile(t, "skills/agentcall/SKILL.md")
	assertContains(t, "skills/agentcall/SKILL.md", content, "worktree")
	assertContains(t, "skills/agentcall/SKILL.md", content, "environment")
	assertContains(t, "skills/agentcall/SKILL.md", content, "cd")
}

func TestClaudeInstructionsMentionVersionMustMatchTag(t *testing.T) {
	t.Parallel()

	content := readFile(t, "CLAUDE.md")
	assertContains(t, "CLAUDE.md", content, "version")
	assertContains(t, "CLAUDE.md", content, "tag")
	assertContains(t, "CLAUDE.md", content, "match")
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertContains(t *testing.T, path, content, want string) {
	t.Helper()

	if !strings.Contains(content, want) {
		t.Fatalf("%s does not contain %q", path, want)
	}
}

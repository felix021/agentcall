---
name: agentcall
description: Use the local PTY runner instead of claude -p when you need real terminal behavior plus a structured JSON result.
---

Use `agentcall run --prompt "..." -- claude` or `agentcall run --prompt "..." -- codex` when you need:

- real terminal-agent behavior inside a PTY
- structured JSON stop-state output
- a pollable final status JSON file, plus separate transcript/artifact files, when you provide `--status-file` or otherwise know the artifacts path

If someone asks to run the target agent in "yolo" mode, that means using the target CLI's no-approval shortcut with the correct argument: `claude --dangerously-skip-permissions` for Claude, and `codex --dangerously-bypass-approvals-and-sandbox` for Codex.

Use `--auto-trust` when the target agent may stop at a recognized workspace trust prompt and you want the runner to confirm it once automatically.

The runner starts the target TUI first and injects the wrapped task prompt through the PTY, followed by a separate Enter key. Do not assume it appends the task as a positional CLI argument.

When the task is about a specific repository or git worktree, run `agentcall` from that target worktree. If you are not already there, `cd` into the intended worktree first so the child agent starts in the correct directory and sees the right git status.

Include the relevant environment context in the injected prompt when it affects the task: repo root or worktree path, branch name, whether the diff includes untracked files, and any important local constraints that the downstream agent would not infer reliably on its own.

Prefer statuses `ok`, `needs_input`, `error`, `refused`, `timed_out`, and `callback_missing`. If the runner returns `needs_input`, inspect the saved status JSON and the separate transcript/artifact files before deciding whether to retry. Pass `--status-file` when you need a predictable status path from the caller side.

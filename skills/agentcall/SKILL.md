---
name: agentcall
description: Use the local PTY runner instead of claude -p when you need real terminal behavior plus a structured JSON result.
---

Use `agentcall run --prompt "..." -- claude` or `agentcall run --prompt "..." -- codex` when you need:

- real terminal-agent behavior inside a PTY
- structured JSON stop-state output
- a pollable final status JSON file, plus separate transcript/artifact files, when you provide `--status-file` or otherwise know the artifacts path

Use `--auto-trust` when the target agent may stop at a recognized workspace trust prompt and you want the runner to confirm it once automatically.

The runner starts the target TUI first and injects the wrapped task prompt through the PTY, followed by a separate Enter key. Do not assume it appends the task as a positional CLI argument.

Prefer statuses `ok`, `needs_input`, `error`, `refused`, `timed_out`, and `callback_missing`. If the runner returns `needs_input`, inspect the saved status JSON and the separate transcript/artifact files before deciding whether to retry. Pass `--status-file` when you need a predictable status path from the caller side.

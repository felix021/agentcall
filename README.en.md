# agentcall

[中文 README](README.md)

`agentcall` launches terminal-native agents such as interactive `claude`, interactive `codex`, and other interactive CLI agents inside a real PTY and collects structured stop-state results through a localhost HTTP callback.

The runner starts the target TUI first, injects a wrapped task prompt through the PTY, and then sends a separate Enter keystroke to stay close to human terminal interaction.

The target agent must be able to receive that injected prompt inside its interactive UI and satisfy the callback contract: it must POST JSON back to the provided localhost URL with at least `token`, `status`, `content_type`, and `content`.

## Features

- Preserves real terminal interaction instead of relying on print-only modes such as `-p`
- Injects the task prompt through the PTY instead of relying on positional prompt arguments
- Emits structured JSON results for higher-level invokers and agents
- Can persist `status.json` and `transcript.log`
- Supports `--auto-trust` to confirm a recognized workspace trust dialog once

## Basic Usage

```bash
agentcall run \
  --timeout 30s \
  --prompt "review this diff" \
  -- claude
```

## Common Flags

- `--prompt`: task text injected into the target agent through the PTY
- `--timeout`: per-run timeout, default `90s`
- `--artifacts-dir`: output directory for result and transcript artifacts; if omitted, a temporary directory is created automatically
- `--status-file`: explicit path for the status JSON; if omitted, it defaults to `artifacts-dir/status.json`
- `--auto-trust`: auto-confirms one recognized trust prompt

## Claude Example

```bash
agentcall run \
  --auto-trust \
  --timeout 180s \
  --prompt "Review the current diff and send the final result through the callback." \
  -- claude --dangerously-skip-permissions
```

## Codex Example

```bash
agentcall run \
  --timeout 180s \
  --prompt "Review the current diff and send the final result through the callback." \
  -- codex --dangerously-bypass-approvals-and-sandbox
```

## Output

Once the runner has successfully started the target agent, stdout contains a single JSON envelope for both callback results and runner-generated terminal outcomes such as `timed_out` or `callback_missing`.
For argument errors, startup failures, or JSON encoding failures, the CLI writes plain-text errors to stderr and returns exit code `1` instead.

The runner prints the final result as a single JSON object, for example:

```json
{
  "run_id": "latest",
  "state": "callback_received",
  "status": "ok",
  "exit_code": 0,
  "content_type": "text/plain",
  "content": "done",
  "error": ""
}
```

Callback-accepted `status` values:

- `ok`
- `needs_input`
- `error`
- `refused`

The runner may also emit these non-callback terminal outcomes:

- `timed_out`
- `callback_missing`

## Exit Codes

- `0`: `ok`
- `2`: `needs_input` or `refused`
- `3`: `timed_out`
- `4`: `callback_missing`
- `1`: `error`, plus other failures such as argument errors, runner startup failures, or internal errors

## Artifacts

When the runner successfully starts the target agent and reaches a terminal outcome, it writes the following files under the artifacts directory:

- `status.json`
- `transcript.log`

If the target process fails before startup completes, for example because the command does not exist, the directory may exist while these files do not.

If `--artifacts-dir` is omitted, the directory is created under a temporary path, which is fine for ad-hoc runs but not reliable for external callers.
If you need predictable final-result paths:

- pass `--status-file` for a stable final status JSON path
- pass `--artifacts-dir` for a stable transcript and artifact directory

`status.json` is written only at the end of a run. It does not continuously publish intermediate states such as `starting`, `running`, or `active`, so it should not be treated as a live progress channel.

`transcript.log` contains terminal output plus runner annotations such as `auto-trust confirmed`, which helps debug interaction flow.

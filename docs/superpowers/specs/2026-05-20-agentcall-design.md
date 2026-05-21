# Claude PTY Runner Design

Date: 2026-05-20

## Goal

Build a Go-based local CLI runner that can replace `claude -p` for structured automation by:

- running `claude` in a real PTY
- preserving interactive terminal behavior
- collecting a clean structured result over an ephemeral `127.0.0.1` HTTP callback on a local TCP port
- exposing pollable run state and recent terminal output so an invoker agent can detect stalls, clarifications, and restart needs

## Scope

This design covers v1 of `agentcall` as a single-run local CLI for Linux.

In scope:

- one invocation per process, no long-lived daemon
- PTY-based child execution
- localhost-only one-shot HTTP result callback on an ephemeral TCP port
- transcript capture and run metadata persistence
- real-time runner heuristics for active vs idle vs likely waiting-for-input states
- pollable local status for external invokers
- prompt contract for Claude stop states
- a Codex skill that standardizes use of this runner

Out of scope for v1:

- Windows and macOS support
- browser transport or remote attach
- multi-session management in one process
- a compatibility promise for non-Claude terminal UIs beyond "best effort"
- automatic answering of confirmations or clarifications

## Problem Statement

`claude -p` is convenient for automation because it produces a clean return value, but it does not preserve the same interactive TUI behavior as a real terminal session. The replacement runner must keep Claude inside a PTY while still giving the caller a deterministic machine-readable result.

The key design constraint is that terminal output is not a stable API. Claude may produce progress text, spinners, clarifying questions, confirmations, or other terminal-only state that cannot be safely parsed as the final result. Therefore the runner uses two channels:

- PTY transcript channel: progress, heuristics, debugging, recovery
- localhost HTTP callback channel: authoritative structured stop signal

## Recommended Approach

Implement a native Go CLI that starts `claude` under a PTY, injects a control preamble into the prompt, starts an ephemeral loopback TCP listener, and resolves the run from the callback payload rather than from terminal scraping.

This is preferred over shell automation because it gives explicit control over:

- PTY lifecycle
- incremental output handling
- timeout behavior
- transcript persistence
- callback validation
- state polling for external invokers

It is preferred over a daemon-first design because the first required use case is a single invocation that can replace `claude -p` in scripts and agent workflows.

## High-Level Architecture

The runner has five internal parts.

### 1. Launcher

Responsible for:

- parsing CLI arguments
- generating a run ID and callback token
- choosing artifact paths
- starting the callback server
- spawning `claude` in a PTY

### 2. PTY Session Manager

Responsible for:

- creating the PTY
- wiring stdin/stdout/stderr to the child
- reading raw PTY output incrementally
- supporting terminal resize when needed later
- terminating the child on timeout or user cancellation

### 3. Result Callback Server

Responsible for:

- binding `127.0.0.1:0`
- exposing a one-shot HTTP callback endpoint
- accepting exactly one valid result payload
- validating token and JSON shape
- marking the run complete or blocked based on callback status

The callback server is per-run and ephemeral. It is never exposed on non-loopback addresses and never uses a fixed port.

### 4. State Tracker

Responsible for:

- transcript append
- last-activity timestamps
- rolling tail buffer
- runner state transitions
- persistence of run metadata

This is the surface that lets an external invoker poll whether Claude is still active or likely waiting for input.

### 5. Heuristic Activity Detector

Responsible for classifying PTY behavior in real time.

It must not depend only on one specific spinner glyph or exact Claude UI layout. Instead it should combine:

- repaint cadence
- meaningful tail changes
- known blocker prompt patterns
- elapsed idle time without screen changes

The callback remains authoritative when it arrives. The heuristic layer exists to detect stalled sessions before callback arrival.

## CLI Surface

The first CLI stays intentionally narrow.

```bash
agentcall run -- claude
```

Expected initial options:

- `--prompt-file <path>`
- `--timeout <duration>`
- `--artifacts-dir <path>`
- `--result-format json`
- `--status-file <path>`
- `--tail-lines <n>`

Prompt input may also be accepted from stdin when `--prompt-file` is omitted.

The runner's own stdout should emit a single JSON envelope for every terminal outcome so that shells and other tools can consume one stable contract. Human-readable progress belongs on stderr or artifact files, not mixed into the structured result stream.

Suggested exit codes:

- `0`: callback status `ok`
- `1`: infrastructure failure or callback status `error`
- `2`: callback status `needs_input` or `refused`
- `3`: runner timeout
- `4`: process exited without an accepted callback

Suggested stdout envelope for all terminal outcomes:

```json
{
  "run_id": "01-example",
  "state": "callback_received",
  "status": "ok",
  "exit_code": 0,
  "content_type": "text/markdown",
  "content": "final answer here",
  "error": ""
}
```

For timeout or missing-callback cases, `content` may be empty but the envelope should still be emitted so downstream parsers never need to special-case stdout shape.

## Prompt Contract

The runner prepends a control preamble ahead of the user payload. The preamble tells Claude:

- you are running inside a local PTY automation wrapper
- keep normal progress and reasoning in the terminal UI
- when you stop, always send a structured JSON payload to `http://127.0.0.1:<port>/callback`
- include the provided token exactly
- use the callback not only for success, but for any terminal stop state
- keep any terminal-side final summary brief because the callback payload is authoritative

The crucial rule is:

> Always invoke the localhost callback when you stop making forward progress for the current turn, including success, clarification needed, confirmation needed, refusal, or error.

This avoids silent stalls where the process remains open but the invoker has no structured signal.

## Callback Protocol

The runner exposes a loopback-only HTTP endpoint on an ephemeral TCP port. Claude sends a single `POST /callback` request with a JSON body.

This keeps the control channel "via a TCP port" while using a portable send mechanism that Claude can invoke with common tools such as `curl`.

The runner should preflight that a supported send path exists before starting the main run. In v1 the expected mechanism is `curl`, with a documented fallback to `bash` plus `/dev/tcp` only if needed later.

Example success payload:

```json
{
  "token": "abc123",
  "status": "ok",
  "content_type": "text/markdown",
  "content": "final answer here",
  "metadata": {
    "tool": "claude",
    "mode": "default"
  }
}
```

Example blocked payload:

```json
{
  "token": "abc123",
  "status": "needs_input",
  "content_type": "text/plain",
  "content": "I need clarification about the target branch.",
  "metadata": {
    "reason": "clarification"
  }
}
```

Supported v1 statuses:

- `ok`
- `needs_input`
- `error`
- `refused`

Validation rules:

- source address must be loopback
- token must match exactly
- JSON must parse
- `status` must be recognized
- `content` must be present, even if empty for some errors

Transport rules:

- accept only `POST /callback`
- apply a short header/body read deadline
- reject partial or malformed requests without consuming the callback slot
- after child exit, keep the callback endpoint alive for a short grace period before declaring `callback_missing`

The first valid callback wins. Later requests are rejected and logged.

## Runner State Model

The runner tracks these explicit states:

- `starting`
- `running`
- `active`
- `idle`
- `awaiting_input`
- `callback_received`
- `exited`
- `timed_out`
- `failed`

Interpretation:

- `starting`: process and listener are being created
- `running`: child is alive, no stronger heuristic yet
- `active`: PTY screen is changing in a way consistent with active work
- `idle`: child is alive but the screen has remained stable past the idle threshold
- `awaiting_input`: idle plus strong evidence of a clarification, confirmation, or blocker prompt
- `callback_received`: authoritative stop payload has been accepted
- `exited`: child has terminated
- `timed_out`: runner timeout expired
- `failed`: infrastructure failure such as PTY spawn error or callback parse failure that ends the run

The invoker should not infer completion from `idle`. Only `callback_received`, `exited`, `timed_out`, or `failed` are terminal states.

## Polling Surface

The invoker agent must be able to inspect the run while it is still alive.

V1 will persist a local status JSON file per run rather than exposing a second status server. This keeps the one-shot CLI small while still allowing external polling.

The status file should include at least:

- `run_id`
- `state`
- `pid`
- `started_at`
- `updated_at`
- `last_pty_activity_at`
- `last_screen_change_at`
- `callback_received`
- `callback_status`
- `exit_code`
- `tail_lines`
- `artifacts`

`tail_lines` is critical. The invoker can poll the latest rendered lines if Claude appears stalled and decide whether to:

- provide missing input in a later version
- terminate and restart the run
- fail the parent workflow

## Real-Time Detection Heuristics

The runner itself performs online detection because Claude's TUI usually keeps repainting while work is in progress.

The design should avoid depending only on a specific icon. Instead:

1. Capture a rolling normalized view of the visible terminal tail.
2. Compute hashes for the normalized view at short intervals.
3. Record repaint frequency and content-change timestamps.
4. Maintain thresholds for "active" vs "stable but alive".
5. Apply prompt-pattern classifiers to the stable tail when activity stops.

Useful prompt-pattern classes:

- direct question ending with `?`
- explicit requests for clarification
- confirmation phrases like `continue?`, `proceed?`, or tool-permission prompts
- refusal or inability statements

These heuristics are advisory. The callback protocol remains the required stop signal. The heuristic exists to surface likely stalls before that signal and to improve operator decisions.

## Transcript And Artifacts

Each run should persist artifacts under a run-specific directory.

Permission requirements:

- artifact directory mode `0700`
- artifact files mode `0600`

Minimum artifacts:

- raw PTY transcript
- normalized tail snapshots or tail log
- status JSON file
- accepted callback payload if any
- run metadata file containing timestamps, CLI args, timeout, and exit code

This supports debugging when:

- Claude exits without callback
- callback payload is invalid
- Claude stalls awaiting input
- terminal UI behavior changes and heuristics need tuning

## Failure Handling

The runner must fail closed rather than guessing final output from terminal text.

Failure cases:

- no callback received before process exit: mark run failed with `callback_missing`
- invalid token: reject payload and continue waiting until timeout or exit
- invalid JSON: reject payload and continue waiting until timeout or exit
- partial HTTP request or callback read timeout: reject request and continue waiting until timeout or exit
- timeout before callback: mark `timed_out`
- PTY spawn failure: mark `failed`
- callback server bind failure: mark `failed`

The runner should not promote scraped PTY text to the final structured result in v1.

On child exit, the runner should keep the callback endpoint alive for a short grace window so an in-flight stop callback is not lost due to process-exit races.

## Security Boundaries

The result listener is localhost-only and per-run.

Security controls:

- bind only to `127.0.0.1`
- use high-entropy per-run token
- accept one callback only
- record peer address in artifacts
- never expose a fixed external port
- restrict artifact permissions because the token may appear in the PTY transcript

This is a containment measure, not a strong sandbox. Any same-user local process could still attempt to race the callback if it knows the token, so token secrecy matters.

## Skill Design

Add a repo-local skill for using this runner once the implementation exists.

Purpose:

- standardize structured Claude invocations through the PTY runner
- replace direct `claude -p` use for workflows that need real terminal semantics plus a clean machine-readable result

Expected skill responsibilities:

- explain when to prefer `agentcall` over `claude -p`
- define safe command templates for review, summarize, and general prompt execution
- require structured output expectations
- explain how to inspect the status file and tail lines on stalled runs
- explain failure handling for `needs_input`, `callback_missing`, and timeout states

The skill should be a thin wrapper around the runner contract, not a second protocol.

## Testing Strategy

Implementation must follow TDD.

V1 tests should avoid the real Claude binary and instead use a fake interactive PTY target that can:

- print incremental progress
- repaint a fake spinner/status area
- pause and wait for input
- send valid or invalid callbacks
- exit with or without callback

Required tests:

- callback listener accepts one loopback HTTP payload and validates token
- runner emits success when PTY is noisy but callback is clean
- runner marks `needs_input` when callback sends blocked status
- runner records `idle` and `awaiting_input` heuristics before terminal stop
- runner fails on invalid token
- runner fails on invalid JSON after timeout or process exit
- runner rejects partial callback bodies and preserves the callback slot
- runner honors a short post-exit grace period for an in-flight callback
- runner fails on missing callback
- runner persists artifacts in both success and failure cases
- prompt preamble includes callback contract and stop-state rule
- status file updates include recent tail lines
- CLI stdout remains clean structured output on success
- status file writes are atomic and never expose partial JSON
- brief pauses during active repainting do not flip to `idle`
- slow but meaningful progress remains `active`
- known blocker tails transition to `awaiting_input` within threshold

## Implementation Notes For Go

Use Go for the implementation.

Likely packages:

- `github.com/creack/pty` for PTY handling
- standard library `net/http`, `os/exec`, `context`, `encoding/json`, `bufio`, `time`

Suggested package boundaries:

- `cmd/agentcall`
- `internal/runner`
- `internal/ptyio`
- `internal/callback`
- `internal/state`
- `internal/prompt`
- `internal/fakeagent` for tests only

Keep the state model and callback contract separate from Claude-specific prompt wording so later adapters remain possible.

Status file writes should be atomic: write to a temp file in the same directory, then rename into place.

## Open Questions Resolved In This Design

- Language: Go
- Invocation model: single-run CLI
- Callback exposure: localhost only
- Result contract: callback is authoritative for all stop states, not only success
- Stall detection: runner performs heuristics based on screen updates and tail content
- Invoker observability: pollable local status file with recent output tail

## Success Criteria

V1 is successful when an external tool can:

1. invoke `agentcall run -- claude`
2. pass a prompt payload
3. observe live state and recent output while Claude works
4. receive a clean structured result from runner stdout when Claude reports `ok`
5. detect `needs_input`, timeout, or missing-callback cases deterministically without scraping terminal output as the final answer

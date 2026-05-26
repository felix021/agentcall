# Stderr Heartbeat Design

Date: 2026-05-26

## Goal

Make `agentcall run` emit machine-readable progress heartbeats on `stderr` while the wrapped agent is still running, without changing the single final JSON result written to `stdout`.

## Scope

This design covers one incremental feature on top of the existing runner:

- add CLI args to control heartbeat emission
- emit newline-delimited JSON heartbeat events on `stderr`
- default to one heartbeat per tick, with a `1s` tick period
- allow callers to reduce or disable heartbeat detail with a verbose level

Out of scope:

- changing the final result envelope on `stdout`
- turning `status.json` into a live progress stream
- introducing a second IPC channel beyond the existing `stderr` stream

## Problem Statement

`agentcall` currently keeps `stdout` clean by emitting exactly one terminal JSON envelope when a run finishes. That is good for automation, but it gives an external monitor no structured signal that the agent is still alive while the PTY session is in progress.

The runner already has a polling loop that derives states such as `running`, `active`, `idle`, and `awaiting_input`. The missing piece is exposing that live state in a stable, machine-readable format on `stderr`, where progress belongs.

## Recommended Approach

Add a `stderr` heartbeat emitter owned by the runner loop. The emitter writes one compact JSON object per tick as newline-delimited JSON.

This approach is preferred because it:

- keeps `stdout` fully backward compatible
- reuses the existing runner tick loop and state detector
- provides a stable monitor channel without requiring file polling
- makes heartbeat frequency and verbosity explicit CLI contract rather than hard-coded behavior

## CLI Surface

Extend `agentcall run` with two new flags:

- `--heartbeat-period <duration>`
- `--verbose <level>`

Defaults:

- `--heartbeat-period=1s`
- `--verbose=1`

Semantics:

- `--verbose=0`: disable heartbeat JSON output on `stderr`
- `--verbose=1`: emit one minimal heartbeat JSON event every tick while the child process is alive
- `--verbose=2`: emit the same per-tick heartbeat plus additional diagnostic fields useful to a supervising agent

The default behavior is intentionally noisy because the caller explicitly wants a positive liveness signal every tick, not only on state transitions.

## Heartbeat Event Contract

Each heartbeat is one JSON object followed by `\n` on `stderr`.

Base schema for `--verbose=1`:

```json
{
  "type": "heartbeat",
  "run_id": "latest",
  "seq": 3,
  "timestamp": "2026-05-26T02:37:44Z",
  "state": "active"
}
```

Field meanings:

- `type`: always `"heartbeat"` for this event family
- `run_id`: matches the runner envelope identifier, still `"latest"` in v1
- `seq`: monotonic per-run heartbeat counter starting at `1`
- `timestamp`: UTC RFC3339 timestamp for the tick
- `state`: current derived runner state such as `running`, `active`, `idle`, or `awaiting_input`

Additional fields for `--verbose=2`:

```json
{
  "type": "heartbeat",
  "run_id": "latest",
  "seq": 3,
  "timestamp": "2026-05-26T02:37:44Z",
  "state": "active",
  "screen_changed": true,
  "auto_trust_sent": false,
  "prompt_pasted": true,
  "prompt_submitted": true
}
```

These extra fields are intentionally limited to runner-owned booleans that are already known in the loop, so the feature stays small and deterministic.

## Data Flow

The runner loop already executes on a periodic ticker. The heartbeat feature layers onto that flow as follows:

1. Parse the new CLI flags into `runner.RunInput`.
2. Resolve validated defaults in runner options.
3. Construct a `HeartbeatEmitter` bound to the provided `stderr` writer.
4. On every tick, after the detector state is updated, emit a heartbeat event if verbosity is greater than zero.
5. Continue writing the final terminal envelope only to `stdout`.

The heartbeat stream stops as soon as the run reaches any terminal outcome:

- callback received
- process exited
- runner timeout
- infrastructure failure

No final heartbeat is required after terminal outcome because the existing stdout envelope remains authoritative.

## Component Changes

### CLI

`cmd/agentcall/main.go` will:

- parse `--heartbeat-period`
- parse `--verbose`
- validate that heartbeat period is positive
- validate that verbose level is not negative
- pass both values into `runner.RunInput`
- call the runner with the caller-provided `stderr` writer

### Runner

`internal/runner/runner.go` will:

- accept the heartbeat settings and `stderr` writer
- create a heartbeat emitter once per run
- emit a heartbeat on every tick before terminal-state checks continue the loop

The emission path must never break a run. If encoding a heartbeat fails, the runner should ignore that write failure and continue toward the final result.

### Shared Type

Add a dedicated heartbeat struct near runner code instead of reusing the final result envelope type. The heartbeat schema is a streaming event, not a terminal result, so keeping the types separate avoids accidental coupling.

## Error Handling

Argument validation failures remain plain-text errors on `stderr` with exit code `1`.

Rules:

- invalid `--heartbeat-period` duration: fail fast during argument parsing
- non-positive `--heartbeat-period`: fail fast during argument parsing
- negative `--verbose`: fail fast during argument parsing
- heartbeat JSON encoding or write failure after startup: swallow and continue the run

The final stdout envelope remains the only contract that affects the process exit code.

## Testing Strategy

Follow TDD with focused tests in two layers.

### CLI tests

Add tests covering:

- parsing default heartbeat settings
- parsing explicit `--heartbeat-period`
- rejecting invalid heartbeat duration
- rejecting non-positive heartbeat period
- parsing explicit verbose level
- rejecting negative verbose level

### Runner tests

Add tests covering:

- heartbeats are emitted to `stderr` while a fake agent is still running
- default cadence emits more than one heartbeat for a multi-tick run
- `--verbose=0` suppresses heartbeat output
- `--verbose=2` includes diagnostic fields
- final stdout envelope remains unchanged

Prefer deterministic assertions over wall-clock-sensitive counts. Tests should check that emitted JSON lines decode successfully and that the observed states are plausible for the fake-agent scenarios.

## Compatibility

This is backward compatible for callers that:

- ignore `stderr`
- already parse only the final stdout JSON envelope

Callers that read `stderr` as plain human text will now receive JSON lines during successful runs when verbosity is nonzero. That is acceptable because `stderr` is the designated progress channel and this change is explicitly intentional.

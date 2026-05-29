# Config Default Models Design

Date: 2026-05-29

## Goal

Allow `agentcall run` to read `~/.config/agentcall/config.yaml` and inject per-tool default model flags for supported agents when the caller did not specify a model explicitly.

## Scope

This design covers one incremental feature:

- read an optional YAML config file from `~/.config/agentcall/config.yaml`
- support an extensible schema for tool-specific defaults
- inject default model flags for `claude` and `codex` only when the command does not already carry a model flag

Out of scope:

- supporting arbitrary command rewrites from config
- changing runner behavior after command parsing
- requiring the config file to exist

## Config Shape

The config file uses an extensible nested schema:

```yaml
tools:
  claude:
    default_model: claude-opus-4-6
  codex:
    default_model: gpt-5.4
```

Unknown tools and unknown fields are ignored so the schema can grow without breaking existing configs.

## Precedence Rules

Command resolution order is:

1. explicit CLI command arguments
2. configured tool default model
3. agent builtin default model

If the config file is missing, `agentcall` behaves exactly as it does today. If the config file is invalid YAML or unreadable for reasons other than non-existence, the CLI fails fast with a clear error.

## Recommended Approach

Load config in the CLI parsing layer and normalize the command before building `runner.RunInput`.

This keeps the behavior easy to test and explain because:

- `parseRunArgs` already owns user-facing argument resolution
- the runner can remain unaware of user config files
- command mutation stays limited to supported tools and one flag family

## Tool Rules

Supported tools:

- `claude`: inject `--model <value>` when missing
- `codex`: inject `-m <value>` when missing

Model detection should treat either supported flag form for a tool as explicit user intent and avoid appending another model flag.

## Testing Strategy

Follow TDD with focused CLI-layer tests:

- missing config file leaves command unchanged
- valid config injects Claude default model
- valid config injects Codex default model
- explicit CLI model flag wins over config
- invalid YAML returns an error

Keep the runner tests unchanged because the feature should resolve entirely before runner execution.

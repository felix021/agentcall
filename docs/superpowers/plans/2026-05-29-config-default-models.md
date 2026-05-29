# Config Default Models Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add optional config-backed default model selection for supported tools without overriding explicit CLI model flags.

**Architecture:** Resolve config in the CLI layer by loading `~/.config/agentcall/config.yaml`, then merge supported per-tool default model flags into the command before building `runner.RunInput`. Keep the runner unaware of config files so the feature remains isolated to command preparation.

**Tech Stack:** Go, `flag`, `os.UserHomeDir`, `gopkg.in/yaml.v3`, Go tests

---

### Task 1: Add failing CLI tests for config-driven command defaults

**Files:**
- Modify: `cmd/agentcall/main_test.go`
- Test: `cmd/agentcall/main_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestParseRunArgsInjectsConfiguredClaudeModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeConfig(t, "tools:\n  claude:\n    default_model: claude-opus-4-6\n")

	got, err := parseRunArgs([]string{"--", "claude"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseRunArgs() error = %v", err)
	}
	if diff := cmp.Diff([]string{"claude", "--model", "claude-opus-4-6"}, got.Command); diff != "" {
		t.Fatalf("Command mismatch (-want +got):\n%s", diff)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/agentcall -run TestParseRunArgsInjectsConfiguredClaudeModel`
Expected: FAIL because config loading and command merging do not exist yet

- [ ] **Step 3: Add the rest of the failing precedence tests**

```go
func TestParseRunArgsInjectsConfiguredCodexModel(t *testing.T) {}
func TestParseRunArgsKeepsExplicitClaudeModel(t *testing.T) {}
func TestParseRunArgsIgnoresMissingConfigFile(t *testing.T) {}
func TestParseRunArgsRejectsInvalidConfigYAML(t *testing.T) {}
```

- [ ] **Step 4: Run the targeted test set to verify the failures**

Run: `go test ./cmd/agentcall -run 'TestParseRunArgs(InjectsConfiguredClaudeModel|InjectsConfiguredCodexModel|KeepsExplicitClaudeModel|IgnoresMissingConfigFile|RejectsInvalidConfigYAML)'`
Expected: FAIL with missing config behavior assertions

### Task 2: Implement config loading and command merge behavior

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Modify: `cmd/agentcall/main.go`

- [ ] **Step 1: Write the failing unit tests for config decoding and command merge helpers**

```go
func TestLoadReturnsEmptyConfigWhenFileMissing(t *testing.T) {}
func TestApplyDefaultModelAddsClaudeModel(t *testing.T) {}
func TestApplyDefaultModelRespectsExplicitCodexModel(t *testing.T) {}
```

- [ ] **Step 2: Run helper tests to verify they fail**

Run: `go test ./internal/config -run 'Test(LoadReturnsEmptyConfigWhenFileMissing|ApplyDefaultModelAddsClaudeModel|ApplyDefaultModelRespectsExplicitCodexModel)'`
Expected: FAIL because the package does not exist yet

- [ ] **Step 3: Write minimal implementation**

```go
type Config struct {
	Tools map[string]ToolConfig `yaml:"tools"`
}

type ToolConfig struct {
	DefaultModel string `yaml:"default_model"`
}

func Load() (Config, error) { ... }
func ApplyDefaultModel(cfg Config, command []string) []string { ... }
```

- [ ] **Step 4: Update CLI parsing to apply config before returning `runner.RunInput`**

Run: `go test ./cmd/agentcall -run 'TestParseRunArgs(InjectsConfiguredClaudeModel|InjectsConfiguredCodexModel|KeepsExplicitClaudeModel|IgnoresMissingConfigFile|RejectsInvalidConfigYAML)'`
Expected: PASS

### Task 3: Verify the feature and documentation

**Files:**
- Modify: `README.md`
- Modify: `README.en.md`

- [ ] **Step 1: Document config path and schema**

```md
`agentcall` also reads `~/.config/agentcall/config.yaml` for optional per-tool defaults such as `default_model`.
```

- [ ] **Step 2: Run the affected package tests**

Run: `go test ./cmd/agentcall ./internal/config`
Expected: PASS

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`
Expected: PASS

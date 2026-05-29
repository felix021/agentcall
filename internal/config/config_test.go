package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfig(t *testing.T, content string) {
	t.Helper()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	configDir := filepath.Join(homeDir, ".config", "agentcall")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestLoadReturnsEmptyConfigWhenFileMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Tools) != 0 {
		t.Fatalf("Load() Tools = %v, want empty", got.Tools)
	}
}

func TestLoadRejectsInvalidYAML(t *testing.T) {
	writeConfig(t, "tools: [\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
}

func TestApplyDefaultModelAddsClaudeModel(t *testing.T) {
	cfg := Config{
		Tools: map[string]ToolConfig{
			"claude": {DefaultModel: "claude-opus-4-6"},
		},
	}

	got := ApplyDefaultModel(cfg, []string{"claude"})
	want := []string{"claude", "--model", "claude-opus-4-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyDefaultModel() = %v, want %v", got, want)
	}
}

func TestApplyDefaultModelAddsCodexModel(t *testing.T) {
	cfg := Config{
		Tools: map[string]ToolConfig{
			"codex": {DefaultModel: "gpt-5.4"},
		},
	}

	got := ApplyDefaultModel(cfg, []string{"codex"})
	want := []string{"codex", "-m", "gpt-5.4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyDefaultModel() = %v, want %v", got, want)
	}
}

func TestApplyDefaultModelRespectsExplicitCodexModel(t *testing.T) {
	cfg := Config{
		Tools: map[string]ToolConfig{
			"codex": {DefaultModel: "gpt-5.4"},
		},
	}

	got := ApplyDefaultModel(cfg, []string{"codex", "--model", "cli-model"})
	want := []string{"codex", "--model", "cli-model"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyDefaultModel() = %v, want %v", got, want)
	}
}

func TestApplyDefaultModelRespectsExplicitCodexShortModelFlag(t *testing.T) {
	cfg := Config{
		Tools: map[string]ToolConfig{
			"codex": {DefaultModel: "gpt-5.4"},
		},
	}

	got := ApplyDefaultModel(cfg, []string{"codex", "-m", "cli-model"})
	want := []string{"codex", "-m", "cli-model"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyDefaultModel() = %v, want %v", got, want)
	}
}

func TestApplyDefaultModelRespectsExplicitClaudeEqualsModelFlag(t *testing.T) {
	cfg := Config{
		Tools: map[string]ToolConfig{
			"claude": {DefaultModel: "claude-opus-4-6"},
		},
	}

	got := ApplyDefaultModel(cfg, []string{"claude", "--model=cli-model"})
	want := []string{"claude", "--model=cli-model"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyDefaultModel() = %v, want %v", got, want)
	}
}

func TestApplyDefaultModelRespectsExplicitCodexEqualsModelFlag(t *testing.T) {
	cfg := Config{
		Tools: map[string]ToolConfig{
			"codex": {DefaultModel: "gpt-5.4"},
		},
	}

	got := ApplyDefaultModel(cfg, []string{"codex", "-m=gpt-5.1"})
	want := []string{"codex", "-m=gpt-5.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyDefaultModel() = %v, want %v", got, want)
	}
}

func TestApplyDefaultModelIgnoresModelTokensAfterArgumentTerminator(t *testing.T) {
	cfg := Config{
		Tools: map[string]ToolConfig{
			"codex": {DefaultModel: "gpt-5.4"},
		},
	}

	got := ApplyDefaultModel(cfg, []string{"codex", "--", "--model", "positional"})
	want := []string{"codex", "-m", "gpt-5.4", "--", "--model", "positional"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyDefaultModel() = %v, want %v", got, want)
	}
}

func TestApplyDefaultModelIgnoresUnsupportedTool(t *testing.T) {
	cfg := Config{
		Tools: map[string]ToolConfig{
			"gemini": {DefaultModel: "gemini-2.5-pro"},
		},
	}

	got := ApplyDefaultModel(cfg, []string{"gemini"})
	want := []string{"gemini"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyDefaultModel() = %v, want %v", got, want)
	}
}

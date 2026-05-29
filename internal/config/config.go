package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Tools map[string]ToolConfig `yaml:"tools"`
}

type ToolConfig struct {
	DefaultModel string `yaml:"default_model"`
}

func Load() (Config, error) {
	path, err := path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}

func ApplyDefaultModel(cfg Config, command []string) []string {
	if len(command) == 0 {
		return nil
	}
	tool := command[0]
	toolCfg, ok := cfg.Tools[tool]
	if !ok || toolCfg.DefaultModel == "" || hasExplicitModel(tool, command[1:]) {
		return append([]string{}, command...)
	}
	out := append([]string{}, command...)
	insertAt := len(out)
	for i, arg := range out[1:] {
		if arg == "--" {
			insertAt = i + 1
			break
		}
	}
	switch tool {
	case "claude":
		return insertArgs(out, insertAt, "--model", toolCfg.DefaultModel)
	case "codex":
		return insertArgs(out, insertAt, "-m", toolCfg.DefaultModel)
	default:
		return out
	}
}

func path() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(homeDir, ".config", "agentcall", "config.yaml"), nil
}

func hasExplicitModel(tool string, args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--model" || strings.HasPrefix(arg, "--model=") {
			return true
		}
		if tool == "codex" && (arg == "-m" || strings.HasPrefix(arg, "-m=")) {
			return true
		}
	}
	return false
}

func insertArgs(command []string, index int, args ...string) []string {
	if index >= len(command) {
		return append(command, args...)
	}
	out := make([]string, 0, len(command)+len(args))
	out = append(out, command[:index]...)
	out = append(out, args...)
	out = append(out, command[index:]...)
	return out
}

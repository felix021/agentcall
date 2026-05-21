package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func NewOptions(in OptionsInput) (Options, error) {
	if len(in.Command) == 0 {
		return Options{}, fmt.Errorf("command required")
	}
	timeout := 90 * time.Second
	if in.Timeout != "" {
		d, err := time.ParseDuration(in.Timeout)
		if err != nil {
			return Options{}, err
		}
		timeout = d
	}
	artifactsDir := in.ArtifactsDir
	if artifactsDir == "" {
		baseDir := filepath.Join(os.TempDir(), "agentcall")
		if err := os.MkdirAll(baseDir, 0o755); err != nil {
			return Options{}, err
		}
		var err error
		artifactsDir, err = os.MkdirTemp(baseDir, "run-")
		if err != nil {
			return Options{}, err
		}
	}
	statusFile := in.StatusFile
	if statusFile == "" {
		statusFile = filepath.Join(artifactsDir, "status.json")
	}
	tailLines := in.TailLines
	if tailLines <= 0 {
		tailLines = 40
	}
	return Options{
		Command:      in.Command,
		Timeout:      timeout,
		ArtifactsDir: artifactsDir,
		StatusFile:   statusFile,
		TailLines:    tailLines,
		AutoTrust:    in.AutoTrust,
	}, nil
}

func ExitCodeForStatus(status CallbackStatus) int {
	switch status {
	case CallbackStatusOK:
		return 0
	case CallbackStatusNeedsInput, CallbackStatusRefused:
		return 2
	case CallbackStatusTimedOut:
		return 3
	case CallbackStatusMissing:
		return 4
	default:
		return 1
	}
}

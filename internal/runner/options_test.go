package runner

import (
	"path/filepath"
	"testing"
)

func TestDefaultOptionsApplyArtifactsAndTailLines(t *testing.T) {
	opts, err := NewOptions(OptionsInput{
		Command:      []string{"claude"},
		Timeout:      "90s",
		ArtifactsDir: "",
		StatusFile:   "",
		TailLines:    0,
	})
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}
	if opts.TailLines != 40 {
		t.Fatalf("TailLines = %d, want 40", opts.TailLines)
	}
	if opts.ArtifactsDir == "" {
		t.Fatalf("ArtifactsDir empty")
	}
	if got, want := filepath.Base(filepath.Dir(opts.ArtifactsDir)), "agentcall"; got != want {
		t.Fatalf("ArtifactsDir parent = %q, want %q", got, want)
	}
	if filepath.Base(opts.ArtifactsDir) == "latest" {
		t.Fatalf("ArtifactsDir = %q, want run-specific path", opts.ArtifactsDir)
	}
	if opts.StatusFile == "" {
		t.Fatalf("StatusFile empty")
	}
	if got, want := opts.StatusFile, filepath.Join(opts.ArtifactsDir, "status.json"); got != want {
		t.Fatalf("StatusFile = %q, want %q", got, want)
	}
	if opts.AutoTrust {
		t.Fatal("AutoTrust = true, want false by default")
	}
}

func TestDefaultArtifactsDirIsUniquePerOptionsInstance(t *testing.T) {
	first, err := NewOptions(OptionsInput{
		Command: []string{"claude"},
	})
	if err != nil {
		t.Fatalf("first NewOptions() error = %v", err)
	}
	second, err := NewOptions(OptionsInput{
		Command: []string{"claude"},
	})
	if err != nil {
		t.Fatalf("second NewOptions() error = %v", err)
	}
	if first.ArtifactsDir == second.ArtifactsDir {
		t.Fatalf("ArtifactsDir reused shared path %q", first.ArtifactsDir)
	}
}

func TestExitCodeForStatus(t *testing.T) {
	tests := []struct {
		status CallbackStatus
		want   int
	}{
		{CallbackStatusOK, 0},
		{CallbackStatusNeedsInput, 2},
		{CallbackStatusRefused, 2},
		{CallbackStatusError, 1},
		{CallbackStatusTimedOut, 3},
		{CallbackStatusMissing, 4},
	}
	for _, tc := range tests {
		if got := ExitCodeForStatus(tc.status); got != tc.want {
			t.Fatalf("ExitCodeForStatus(%q) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

func TestExitCodeForStatusTreatsRunnerStatesAsExplicitBoundary(t *testing.T) {
	if got := ExitCodeForStatus(CallbackStatus(StatusRunning)); got != 1 {
		t.Fatalf("ExitCodeForStatus(StatusRunning) = %d, want 1", got)
	}
}

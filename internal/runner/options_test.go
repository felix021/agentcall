package runner

import (
	"path/filepath"
	"testing"
	"time"
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

func TestDefaultOptionsApplyHeartbeatSettings(t *testing.T) {
	opts, err := NewOptions(OptionsInput{
		Command: []string{"claude"},
	})
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}
	if opts.HeartbeatPeriod != time.Second {
		t.Fatalf("HeartbeatPeriod = %v, want %v", opts.HeartbeatPeriod, time.Second)
	}
	if opts.Verbose != 1 {
		t.Fatalf("Verbose = %d, want 1", opts.Verbose)
	}
}

func TestNewOptionsPreservesExplicitVerboseZero(t *testing.T) {
	opts, err := NewOptions(OptionsInput{
		Command:    []string{"claude"},
		Verbose:    0,
		VerboseSet: true,
	})
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}
	if opts.HeartbeatPeriod != time.Second {
		t.Fatalf("HeartbeatPeriod = %v, want %v", opts.HeartbeatPeriod, time.Second)
	}
	if opts.Verbose != 0 {
		t.Fatalf("Verbose = %d, want 0", opts.Verbose)
	}
}

func TestNewOptionsPreservesExplicitHeartbeatPeriod(t *testing.T) {
	opts, err := NewOptions(OptionsInput{
		Command:            []string{"claude"},
		HeartbeatPeriod:    250 * time.Millisecond,
		HeartbeatPeriodSet: true,
	})
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}
	if opts.HeartbeatPeriod != 250*time.Millisecond {
		t.Fatalf("HeartbeatPeriod = %v, want %v", opts.HeartbeatPeriod, 250*time.Millisecond)
	}
	if opts.Verbose != 1 {
		t.Fatalf("Verbose = %d, want 1", opts.Verbose)
	}
}

func TestNewOptionsRejectsNonPositiveExplicitHeartbeatPeriod(t *testing.T) {
	for _, period := range []time.Duration{0, -1 * time.Second} {
		_, err := NewOptions(OptionsInput{
			Command:            []string{"claude"},
			HeartbeatPeriod:    period,
			HeartbeatPeriodSet: true,
		})
		if err == nil {
			t.Fatalf("NewOptions(%v) error = nil, want non-nil", period)
		}
		if err.Error() != "heartbeat period must be greater than zero" {
			t.Fatalf("NewOptions(%v) error = %q, want %q", period, err.Error(), "heartbeat period must be greater than zero")
		}
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

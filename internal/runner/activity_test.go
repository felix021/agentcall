package runner

import (
	"testing"
	"time"
)

func TestDetectorStaysActiveWhenScreenKeepsChanging(t *testing.T) {
	d := NewDetector(500*time.Millisecond, []string{"clarification", "continue?", "proceed?"})
	now := time.Now()
	d.Observe("working 1", now)
	d.Observe("working 2", now.Add(100*time.Millisecond))
	if got := d.State(now.Add(150 * time.Millisecond)); got != StatusActive {
		t.Fatalf("State() = %q, want %q", got, StatusActive)
	}
}

func TestDetectorMarksIdleWhenScreenStopsChangingWithoutPrompt(t *testing.T) {
	d := NewDetector(200*time.Millisecond, []string{"clarification", "continue?", "proceed?"})
	now := time.Now()
	d.Observe("completed step", now)
	if got := d.State(now.Add(300 * time.Millisecond)); got != StatusIdle {
		t.Fatalf("State() = %q, want %q", got, StatusIdle)
	}
}

func TestDetectorStaysActiveWhenSameTextIsRepainted(t *testing.T) {
	d := NewDetector(200*time.Millisecond, []string{"clarification", "continue?", "proceed?"})
	now := time.Now()
	d.Observe("spinner frame", now)
	d.Observe("spinner frame", now.Add(150*time.Millisecond))
	if got := d.State(now.Add(300 * time.Millisecond)); got != StatusActive {
		t.Fatalf("State() = %q, want %q", got, StatusActive)
	}
}

func TestDetectorMarksAwaitingInputWhenIdleQuestionTailAppears(t *testing.T) {
	d := NewDetector(200*time.Millisecond, []string{"clarification", "continue?", "proceed?"})
	now := time.Now()
	d.Observe("Need clarification about the target branch?", now)
	if got := d.State(now.Add(300 * time.Millisecond)); got != StatusAwaitingInput {
		t.Fatalf("State() = %q, want %q", got, StatusAwaitingInput)
	}
}

func TestDetectorRecordsBlankScreenObservation(t *testing.T) {
	d := NewDetector(200*time.Millisecond, []string{"clarification", "continue?", "proceed?"})
	now := time.Now()
	d.Observe("", now)
	if got := d.State(now.Add(300 * time.Millisecond)); got != StatusIdle {
		t.Fatalf("State() = %q, want %q", got, StatusIdle)
	}
}

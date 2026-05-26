package runner

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestHeartbeatEmitterSkipsOutputWhenVerboseZero(t *testing.T) {
	var buf bytes.Buffer

	emitter := NewHeartbeatEmitter(&buf, 0)
	emitter.now = func() time.Time {
		return time.Date(2026, time.May, 26, 12, 34, 56, 0, time.UTC)
	}

	if err := emitter.Emit(3, StatusIdle, HeartbeatDiagnostics{ScreenChanged: true}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	if got := buf.String(); got != "" {
		t.Fatalf("Emit() output = %q, want empty", got)
	}
}

func TestHeartbeatEmitterWritesBaseHeartbeatJSON(t *testing.T) {
	var buf bytes.Buffer

	emitter := NewHeartbeatEmitter(&buf, 1)
	emitter.now = func() time.Time {
		return time.Date(2026, time.May, 26, 12, 34, 56, 0, time.UTC)
	}

	if err := emitter.Emit(7, StatusActive, HeartbeatDiagnostics{
		ScreenChanged:   true,
		AutoTrustSent:   true,
		PromptPasted:    true,
		PromptSubmitted: true,
	}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	if len(lines) != 2 || len(lines[1]) != 0 {
		t.Fatalf("Emit() output must be newline-delimited JSON, got %q", buf.String())
	}

	var got map[string]any
	if err := json.Unmarshal(lines[0], &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := map[string]any{
		"type":      "heartbeat",
		"run_id":    "latest",
		"seq":       float64(7),
		"timestamp": "2026-05-26T12:34:56Z",
		"state":     StatusActive,
	}
	if len(got) != len(want) {
		t.Fatalf("heartbeat field count = %d, want %d; payload=%v", len(got), len(want), got)
	}
	for key, wantValue := range want {
		if got[key] != wantValue {
			t.Fatalf("heartbeat[%q] = %v, want %v", key, got[key], wantValue)
		}
	}

	for _, key := range []string{"screen_changed", "auto_trust_sent", "prompt_pasted", "prompt_submitted"} {
		if _, ok := got[key]; ok {
			t.Fatalf("heartbeat unexpectedly included %q at verbose=1: %v", key, got)
		}
	}
}

func TestHeartbeatEmitterIncludesVerboseTwoFields(t *testing.T) {
	var buf bytes.Buffer

	emitter := NewHeartbeatEmitter(&buf, 2)
	emitter.now = func() time.Time {
		return time.Date(2026, time.May, 26, 12, 34, 56, 0, time.FixedZone("UTC+8", 8*60*60))
	}

	diag := HeartbeatDiagnostics{
		ScreenChanged:   true,
		AutoTrustSent:   false,
		PromptPasted:    true,
		PromptSubmitted: false,
	}
	if err := emitter.Emit(9, StatusAwaitingInput, diag); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	line := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))

	var got map[string]any
	if err := json.Unmarshal(line, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got["timestamp"] != "2026-05-26T04:34:56Z" {
		t.Fatalf("heartbeat timestamp = %v, want UTC RFC3339", got["timestamp"])
	}
	if got["screen_changed"] != true {
		t.Fatalf("heartbeat[screen_changed] = %v, want true", got["screen_changed"])
	}
	if got["auto_trust_sent"] != false {
		t.Fatalf("heartbeat[auto_trust_sent] = %v, want false", got["auto_trust_sent"])
	}
	if got["prompt_pasted"] != true {
		t.Fatalf("heartbeat[prompt_pasted] = %v, want true", got["prompt_pasted"])
	}
	if got["prompt_submitted"] != false {
		t.Fatalf("heartbeat[prompt_submitted] = %v, want false", got["prompt_submitted"])
	}
}

func TestHeartbeatEmitterAcceptsNilWriters(t *testing.T) {
	emitter := NewHeartbeatEmitter(nil, 2)
	emitter.now = func() time.Time {
		return time.Date(2026, time.May, 26, 12, 34, 56, 0, time.UTC)
	}

	if err := emitter.Emit(1, StatusRunning, HeartbeatDiagnostics{}); err != nil {
		t.Fatalf("Emit() with nil writer error = %v", err)
	}

	var typedNilBuffer *bytes.Buffer
	emitter = NewHeartbeatEmitter(typedNilBuffer, 2)
	emitter.now = func() time.Time {
		return time.Date(2026, time.May, 26, 12, 34, 57, 0, time.UTC)
	}

	if err := emitter.Emit(2, StatusRunning, HeartbeatDiagnostics{}); err != nil {
		t.Fatalf("Emit() with typed-nil writer error = %v", err)
	}
}

func TestNewHeartbeatEmitterNormalizesNilWritersAtConstruction(t *testing.T) {
	emitter := NewHeartbeatEmitter(nil, 1)
	if emitter.write == nil {
		t.Fatal("write = nil, want bound discard writer")
	}

	var typedNilBuffer *bytes.Buffer
	emitter = NewHeartbeatEmitter(typedNilBuffer, 1)
	if emitter.write == nil {
		t.Fatal("typed-nil write = nil, want bound discard writer")
	}
}

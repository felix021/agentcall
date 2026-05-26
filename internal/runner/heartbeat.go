package runner

import (
	"encoding/json"
	"io"
	"reflect"
	"time"
)

type HeartbeatDiagnostics struct {
	ScreenChanged   bool
	AutoTrustSent   bool
	PromptPasted    bool
	PromptSubmitted bool
}

type HeartbeatEmitter struct {
	writer  io.Writer
	verbose int
	now     func() time.Time
}

type heartbeatBase struct {
	Type      string `json:"type"`
	RunID     string `json:"run_id"`
	Seq       int    `json:"seq"`
	Timestamp string `json:"timestamp"`
	State     string `json:"state"`
}

type heartbeatVerbose struct {
	heartbeatBase
	ScreenChanged   bool `json:"screen_changed"`
	AutoTrustSent   bool `json:"auto_trust_sent"`
	PromptPasted    bool `json:"prompt_pasted"`
	PromptSubmitted bool `json:"prompt_submitted"`
}

func NewHeartbeatEmitter(writer io.Writer, verbose int) *HeartbeatEmitter {
	return &HeartbeatEmitter{
		writer:  heartbeatWriter(writer),
		verbose: verbose,
		now:     time.Now,
	}
}

func (e *HeartbeatEmitter) Emit(seq int, state string, diag HeartbeatDiagnostics) error {
	if e == nil || e.verbose == 0 {
		return nil
	}

	writer := heartbeatWriter(e.writer)

	now := time.Now
	if e.now != nil {
		now = e.now
	}

	base := heartbeatBase{
		Type:      "heartbeat",
		RunID:     "latest",
		Seq:       seq,
		Timestamp: now().UTC().Format(time.RFC3339),
		State:     state,
	}

	var payload []byte
	var err error
	if e.verbose >= 2 {
		payload, err = json.Marshal(heartbeatVerbose{
			heartbeatBase:   base,
			ScreenChanged:   diag.ScreenChanged,
			AutoTrustSent:   diag.AutoTrustSent,
			PromptPasted:    diag.PromptPasted,
			PromptSubmitted: diag.PromptSubmitted,
		})
	} else {
		payload, err = json.Marshal(base)
	}
	if err != nil {
		return err
	}

	_, err = writer.Write(append(payload, '\n'))
	return err
}

func heartbeatWriter(writer io.Writer) io.Writer {
	if writer == nil {
		return io.Discard
	}

	value := reflect.ValueOf(writer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return io.Discard
		}
	}

	return writer
}

package runner

import (
	"encoding/json"
	"io"
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
	if writer == nil {
		writer = io.Discard
	}

	return &HeartbeatEmitter{
		writer:  writer,
		verbose: verbose,
		now:     time.Now,
	}
}

func (e *HeartbeatEmitter) Emit(seq int, state string, diag HeartbeatDiagnostics) error {
	if e == nil || e.verbose == 0 {
		return nil
	}

	writer := e.writer
	if writer == nil {
		writer = io.Discard
	}

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

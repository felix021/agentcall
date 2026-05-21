package runner

import (
	"strings"
	"time"
)

type Detector struct {
	idleAfter time.Duration
	patterns  []string
	observed  bool
	lastText  string
	lastAt    time.Time
}

func NewDetector(idleAfter time.Duration, patterns []string) *Detector {
	return &Detector{idleAfter: idleAfter, patterns: patterns}
}

func (d *Detector) Observe(text string, at time.Time) {
	d.observed = true
	d.lastText = text
	d.lastAt = at
}

func (d *Detector) State(now time.Time) string {
	if !d.observed {
		return StatusRunning
	}
	if now.Sub(d.lastAt) < d.idleAfter {
		return StatusActive
	}
	lower := strings.ToLower(d.lastText)
	if strings.Contains(lower, "?") {
		return StatusAwaitingInput
	}
	for _, p := range d.patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return StatusAwaitingInput
		}
	}
	return StatusIdle
}

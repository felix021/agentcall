package runner

import (
	"time"

	"github.com/felix021/agentcall/internal/sharedtypes"
)

const (
	StatusStarting        = "starting"
	StatusRunning         = "running"
	StatusActive          = "active"
	StatusIdle            = "idle"
	StatusAwaitingInput   = "awaiting_input"
	StatusCallbackRecv    = "callback_received"
	StatusExited          = "exited"
	StatusTimedOut        = "timed_out"
	StatusFailed          = "failed"
	StatusCallbackMissing = "callback_missing"
)

type CallbackPayload = sharedtypes.CallbackPayload

type ResultEnvelope = sharedtypes.ResultEnvelope

type CallbackStatus string

const (
	CallbackStatusOK         CallbackStatus = "ok"
	CallbackStatusNeedsInput CallbackStatus = "needs_input"
	CallbackStatusRefused    CallbackStatus = "refused"
	CallbackStatusError      CallbackStatus = "error"
	CallbackStatusTimedOut   CallbackStatus = "timed_out"
	CallbackStatusMissing    CallbackStatus = "callback_missing"
)

type OptionsInput struct {
	Command            []string
	Timeout            string
	ArtifactsDir       string
	StatusFile         string
	TailLines          int
	AutoTrust          bool
	HeartbeatPeriod    time.Duration
	HeartbeatPeriodSet bool
	Verbose            int
	VerboseSet         bool
}

type Options struct {
	Command         []string
	Timeout         time.Duration
	ArtifactsDir    string
	StatusFile      string
	TailLines       int
	AutoTrust       bool
	HeartbeatPeriod time.Duration
	Verbose         int
}

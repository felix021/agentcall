package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/felix021/agentcall/internal/callback"
	"github.com/felix021/agentcall/internal/prompt"
	"github.com/felix021/agentcall/internal/ptyio"
	"github.com/felix021/agentcall/internal/state"
)

type RunInput struct {
	Command            []string
	Prompt             string
	Timeout            time.Duration
	ArtifactsDir       string
	StatusFile         string
	AutoTrust          bool
	HeartbeatPeriod    time.Duration
	HeartbeatPeriodSet bool
	Verbose            int
	VerboseSet         bool
}

type sessionWait struct {
	result ptyio.Result
	err    error
}

var randomTokenRead = rand.Read

func Run(ctx context.Context, in RunInput, stderr io.Writer) (ResultEnvelope, error) {
	const (
		controlTickPeriod    = 200 * time.Millisecond
		promptIdleAfter      = 350 * time.Millisecond
		promptFallbackAfter  = 1500 * time.Millisecond
		postTrustDelay       = 500 * time.Millisecond
		promptSubmitFallback = 1500 * time.Millisecond
		enterKey             = "\r"
	)

	opts, err := resolveRunOptions(in)
	if err != nil {
		return ResultEnvelope{}, err
	}

	token, err := randomToken()
	if err != nil {
		return ResultEnvelope{}, err
	}
	store, err := state.NewStore(opts.ArtifactsDir, opts.StatusFile)
	if err != nil {
		return ResultEnvelope{}, err
	}

	srv, err := callback.NewServer(token, 10*time.Second)
	if err != nil {
		return ResultEnvelope{}, err
	}
	defer srv.Close(context.Background())

	callbackURL := srv.URL() + "/callback"
	fullPrompt := prompt.Build(callbackURL, token, in.Prompt)
	argv := buildCommand(opts.Command)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sess, err := ptyio.Start(runCtx, argv)
	if err != nil {
		return ResultEnvelope{}, err
	}

	waitCh := make(chan sessionWait, 1)
	go func() {
		res, waitErr := sess.Wait()
		waitCh <- sessionWait{result: res, err: waitErr}
	}()

	timer := time.NewTimer(opts.Timeout)
	defer timer.Stop()
	controlTick := time.NewTicker(controlTickPeriod)
	defer controlTick.Stop()
	var heartbeatCh <-chan time.Time
	heartbeat := NewHeartbeatEmitter(stderr, opts.Verbose)
	if opts.Verbose > 0 {
		heartbeatTick := time.NewTicker(opts.HeartbeatPeriod)
		defer heartbeatTick.Stop()
		heartbeatCh = heartbeatTick.C
	}

	startedAt := time.Now()
	promptReadyAt := startedAt
	autoTrustSent := false
	promptPasted := false
	promptSubmitted := false
	promptPastedAt := time.Time{}
	promptActivitySeen := false
	lastSnapshot := ""
	detector := NewDetector(promptIdleAfter, []string{"clarification", "continue?", "proceed?"})
	currentState := StatusRunning
	screenChangedSinceHeartbeat := false
	heartbeatSeq := 0
	for {
		select {
		case got := <-srv.Results():
			cancel()
			drainWait(waitCh, store, 500*time.Millisecond)

			out := callbackEnvelope(got)
			_ = store.WriteStatus(out)
			return out, nil

		case wait := <-waitCh:
			_ = store.AppendTranscript([]byte(wait.result.Transcript))

			if out, ok := outcomeFromExit(wait, srv.Results()); ok {
				_ = store.WriteStatus(out)
				return out, nil
			}

			out := ResultEnvelope{
				RunID:    "latest",
				State:    StatusExited,
				Status:   StatusCallbackMissing,
				ExitCode: ExitCodeForStatus(CallbackStatusMissing),
				Error:    "process exited before valid callback",
			}
			_ = store.WriteStatus(out)
			return out, nil

		case <-timer.C:
			cancel()
			drainWait(waitCh, store, 500*time.Millisecond)

			out := ResultEnvelope{
				RunID:    "latest",
				State:    StatusTimedOut,
				Status:   StatusTimedOut,
				ExitCode: ExitCodeForStatus(CallbackStatusTimedOut),
				Error:    "runner timeout exceeded",
			}
			_ = store.WriteStatus(out)
			return out, nil

		case <-heartbeatCh:
			snapshot := sess.Snapshot()
			now := time.Now()
			currentState, lastSnapshot, promptActivitySeen, screenChangedSinceHeartbeat = refreshRunnerState(
				snapshot,
				now,
				detector,
				lastSnapshot,
				promptPasted,
				promptSubmitted,
				promptPastedAt,
				promptActivitySeen,
				screenChangedSinceHeartbeat,
			)
			heartbeatSeq++
			_ = heartbeat.Emit(heartbeatSeq, currentState, HeartbeatDiagnostics{
				ScreenChanged:   screenChangedSinceHeartbeat,
				AutoTrustSent:   autoTrustSent,
				PromptPasted:    promptPasted,
				PromptSubmitted: promptSubmitted,
			})
			screenChangedSinceHeartbeat = false

		case <-controlTick.C:
			snapshot := sess.Snapshot()
			now := time.Now()
			currentState, lastSnapshot, promptActivitySeen, screenChangedSinceHeartbeat = refreshRunnerState(
				snapshot,
				now,
				detector,
				lastSnapshot,
				promptPasted,
				promptSubmitted,
				promptPastedAt,
				promptActivitySeen,
				screenChangedSinceHeartbeat,
			)

			if !autoTrustSent && detectTrustPrompt(snapshot) {
				if !opts.AutoTrust {
					continue
				}
				if err := sess.SendInput("1" + enterKey); err != nil {
					continue
				}
				autoTrustSent = true
				promptReadyAt = now.Add(postTrustDelay)
				_ = store.AppendTranscript([]byte(autoTrustMarker))
				continue
			}

			if promptPasted && !promptSubmitted {
				ready := promptActivitySeen && (currentState == StatusIdle || currentState == StatusAwaitingInput)
				if !ready && now.Sub(promptPastedAt) < promptSubmitFallback {
					continue
				}
				if err := sess.SendInput(enterKey); err == nil {
					promptSubmitted = true
					_ = store.AppendTranscript([]byte(promptSubmittedMarker))
				}
				continue
			}

			if promptPasted || now.Before(promptReadyAt) {
				continue
			}

			ready := currentState == StatusIdle || currentState == StatusAwaitingInput
			if !ready && now.Sub(startedAt) < promptFallbackAfter {
				continue
			}

			if err := sess.SendInput(wrapBracketedPaste(fullPrompt)); err != nil {
				continue
			}
			promptPasted = true
			promptPastedAt = now
			promptActivitySeen = false
			_ = store.AppendTranscript([]byte(promptInjectedMarker))
		}
	}
}

func callbackEnvelope(got callback.Result) ResultEnvelope {
	return ResultEnvelope{
		RunID:       "latest",
		State:       StatusCallbackRecv,
		Status:      got.Payload.Status,
		ExitCode:    ExitCodeForStatus(CallbackStatus(got.Payload.Status)),
		ContentType: got.Payload.ContentType,
		Content:     got.Payload.Content,
	}
}

func resolveRunOptions(in RunInput) (Options, error) {
	timeout := ""
	if in.Timeout > 0 {
		timeout = in.Timeout.String()
	}

	return NewOptions(OptionsInput{
		Command:            append([]string{}, in.Command...),
		Timeout:            timeout,
		ArtifactsDir:       in.ArtifactsDir,
		StatusFile:         in.StatusFile,
		AutoTrust:          in.AutoTrust,
		HeartbeatPeriod:    in.HeartbeatPeriod,
		HeartbeatPeriodSet: in.HeartbeatPeriodSet,
		Verbose:            in.Verbose,
		VerboseSet:         in.VerboseSet,
	})
}

func buildCommand(command []string) []string {
	argv := append([]string{}, command...)
	return argv
}

func drainWait(waitCh <-chan sessionWait, store *state.Store, timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case wait := <-waitCh:
		_ = store.AppendTranscript([]byte(wait.result.Transcript))
	case <-timer.C:
	}
}

func refreshRunnerState(
	snapshot string,
	now time.Time,
	detector *Detector,
	lastSnapshot string,
	promptPasted bool,
	promptSubmitted bool,
	promptPastedAt time.Time,
	promptActivitySeen bool,
	screenChangedSinceHeartbeat bool,
) (state string, updatedSnapshot string, updatedPromptActivitySeen bool, updatedScreenChanged bool) {
	updatedSnapshot = lastSnapshot
	updatedPromptActivitySeen = promptActivitySeen
	updatedScreenChanged = screenChangedSinceHeartbeat

	if snapshot != lastSnapshot {
		detector.Observe(normalizeTerminalText(snapshot), now)
		updatedSnapshot = snapshot
		updatedScreenChanged = true
		if promptPasted && !promptSubmitted && now.After(promptPastedAt) {
			updatedPromptActivitySeen = true
		}
	}

	return detector.State(now), updatedSnapshot, updatedPromptActivitySeen, updatedScreenChanged
}

func outcomeFromExit(_ sessionWait, results <-chan callback.Result) (ResultEnvelope, bool) {
	select {
	case got := <-results:
		return callbackEnvelope(got), true
	default:
		return ResultEnvelope{}, false
	}
}

func randomToken() (string, error) {
	var buf [16]byte
	if _, err := randomTokenRead(buf[:]); err != nil {
		return "", fmt.Errorf("generate callback token: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

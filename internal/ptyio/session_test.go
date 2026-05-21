package ptyio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSessionCapturesTranscriptAndExitCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := Start(ctx, []string{"go", "run", filepath.Join("..", "fakeagent"), "--mode", "no-callback"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	sendFakeAgentPrompt(t, sess)
	res, err := sess.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if !strings.Contains(res.Transcript, "fakeagent: booting") {
		t.Fatalf("Transcript missing boot line: %q", res.Transcript)
	}
	if !strings.Contains(res.Transcript, "Exiting without callback") {
		t.Fatalf("Transcript missing final line: %q", res.Transcript)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestSessionCapturesTranscriptAndNonZeroExitCode(t *testing.T) {
	if os.Getenv("GO_WANT_SESSION_EXIT_NONZERO") == "1" {
		fmt.Print("nonzero-tail\n")
		os.Exit(7)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := Start(ctx, []string{
		"env",
		"GO_WANT_SESSION_EXIT_NONZERO=1",
		os.Args[0],
		"-test.run=TestSessionCapturesTranscriptAndNonZeroExitCode",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	res, err := sess.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if !strings.Contains(res.Transcript, "nonzero-tail") {
		t.Fatalf("Transcript missing non-zero tail: %q", res.Transcript)
	}
	if res.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", res.ExitCode)
	}
}

func TestSessionWaitReturnsStableOutcomeForRepeatedCallers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := Start(ctx, []string{"go", "run", filepath.Join("..", "fakeagent"), "--mode", "no-callback"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	sendFakeAgentPrompt(t, sess)

	type waitResult struct {
		res Result
		err error
	}
	waitOnce := func() waitResult {
		done := make(chan waitResult, 1)
		go func() {
			res, err := sess.Wait()
			done <- waitResult{res: res, err: err}
		}()
		select {
		case got := <-done:
			return got
		case <-time.After(2 * time.Second):
			t.Fatal("Wait() blocked for repeated caller")
			return waitResult{}
		}
	}

	first := waitOnce()
	second := waitOnce()
	third := waitOnce()

	if first.err != nil || second.err != nil || third.err != nil {
		t.Fatalf("Wait() errors = %v / %v / %v", first.err, second.err, third.err)
	}
	if first.res != second.res || second.res != third.res {
		t.Fatalf("Wait() results differ: %#v / %#v / %#v", first.res, second.res, third.res)
	}
}

func TestSessionWaitReturnsErrorOnContextCancellation(t *testing.T) {
	if os.Getenv("GO_WANT_SESSION_SLEEP") == "1" {
		fmt.Print("sleeping\n")
		time.Sleep(500 * time.Millisecond)
		fmt.Print("unreachable-tail\n")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sess, err := Start(ctx, []string{
		"env",
		"GO_WANT_SESSION_SLEEP=1",
		os.Args[0],
		"-test.run=TestSessionWaitReturnsErrorOnContextCancellation",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	res, err := sess.Wait()
	if err == nil {
		t.Fatalf("Wait() error = nil, got result %#v", res)
	}
	if !strings.Contains(res.Transcript, "sleeping") {
		t.Fatalf("Transcript missing pre-cancel output: %#v", res)
	}
}

func TestSessionSetsInteractiveTermWhenInheritedTermIsDumb(t *testing.T) {
	if os.Getenv("GO_WANT_SESSION_PRINT_TERM") == "1" {
		fmt.Print(os.Getenv("TERM"))
		return
	}

	prevTerm, hadTerm := os.LookupEnv("TERM")
	if err := os.Setenv("TERM", "dumb"); err != nil {
		t.Fatalf("Setenv(TERM) error = %v", err)
	}
	defer func() {
		if hadTerm {
			_ = os.Setenv("TERM", prevTerm)
			return
		}
		_ = os.Unsetenv("TERM")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := Start(ctx, []string{
		"env",
		"GO_WANT_SESSION_PRINT_TERM=1",
		os.Args[0],
		"-test.run=TestSessionSetsInteractiveTermWhenInheritedTermIsDumb",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	res, err := sess.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if got := strings.TrimSpace(normalizePTyTranscript(res.Transcript)); !strings.Contains(got, "xterm-256color") {
		t.Fatalf("TERM transcript = %q, want to contain %q", got, "xterm-256color")
	}
}

func TestSessionWaitDrainsFinalTranscriptBeforeReturning(t *testing.T) {
	if os.Getenv("GO_WANT_SESSION_HELPER_PROCESS") == "1" {
		helperTranscriptBurst()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := Start(ctx, []string{
		"env",
		"GO_WANT_SESSION_HELPER_PROCESS=1",
		os.Args[0],
		"-test.run=TestSessionWaitDrainsFinalTranscriptBeforeReturning",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	res, err := sess.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if normalizePTyTranscript(res.Transcript) != helperTranscriptBurstOutput() {
		t.Fatalf("Transcript mismatch: got %d bytes", len(res.Transcript))
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestSessionWaitReturnsWhenDescendantKeepsPTYOpen(t *testing.T) {
	switch {
	case os.Getenv("GO_WANT_SESSION_DESCENDANT_PARENT") == "1":
		helperSpawnDescendantHoldingPTY()
		return
	case os.Getenv("GO_WANT_SESSION_DESCENDANT_CHILD") == "1":
		signal.Ignore(syscall.SIGHUP)
		f, err := os.OpenFile(os.Getenv("GO_WANT_SESSION_DESCENDANT_TTY"), os.O_RDWR, 0)
		if err != nil {
			fmt.Printf("child-open-error: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := Start(ctx, []string{
		"env",
		"GO_WANT_SESSION_DESCENDANT_PARENT=1",
		os.Args[0],
		"-test.run=TestSessionWaitReturnsWhenDescendantKeepsPTYOpen",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	type waitResult struct {
		res Result
		err error
	}
	waitc := make(chan waitResult, 1)
	go func() {
		res, err := sess.Wait()
		waitc <- waitResult{res: res, err: err}
	}()

	select {
	case got := <-waitc:
		if got.err != nil {
			t.Fatalf("Wait() error = %v", got.err)
		}
		if !strings.Contains(got.res.Transcript, "parent-exit") {
			t.Fatalf("Transcript missing parent line: %q", got.res.Transcript)
		}
		if got.res.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, want 0", got.res.ExitCode)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Wait() blocked while descendant kept PTY slave open")
	}
}

func TestSessionFinishPreservesTailWhenReaderCompletesNaturally(t *testing.T) {
	originalGrace := readerDrainGrace
	readerDrainGrace = 40 * time.Millisecond
	defer func() {
		readerDrainGrace = originalGrace
	}()

	ptmx, peer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer ptmx.Close()

	s := &Session{
		ptmx:       ptmx,
		finished:   make(chan struct{}),
		readerDone: make(chan struct{}),
		readNotify: make(chan struct{}, 1),
	}

	go sessionTestReadLoop(s, ptmx)
	expectedChunks := []string{
		"chunk-1\n",
		"chunk-2\n",
		"chunk-3\n",
		"chunk-4\n",
	}
	go func() {
		for _, chunk := range expectedChunks {
			time.Sleep(25 * time.Millisecond)
			_, _ = peer.Write([]byte(chunk))
		}
		_ = peer.Close()
	}()

	s.finish(nil)

	res, err := s.Wait()
	if err != nil {
		t.Fatalf("finish() error = %v", err)
	}
	expected := strings.Join(expectedChunks, "")
	if res.Transcript != expected {
		t.Fatalf("Transcript = %q, want %q", res.Transcript, expected)
	}

	if err := ptmx.Close(); err == nil {
		t.Fatal("ptmx remained open after finish")
	}
}

func TestSessionFinishForcesReaderUnblockOnHangPath(t *testing.T) {
	originalGrace := readerDrainGrace
	readerDrainGrace = 20 * time.Millisecond
	defer func() {
		readerDrainGrace = originalGrace
	}()

	ptmx, peer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer peer.Close()

	s := &Session{
		ptmx:       ptmx,
		finished:   make(chan struct{}),
		readerDone: make(chan struct{}),
		readNotify: make(chan struct{}, 1),
	}

	go sessionTestReadLoop(s, ptmx)

	completed := make(chan struct{})
	go func() {
		s.finish(nil)
		close(completed)
	}()

	select {
	case <-completed:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("finish() blocked waiting for reader")
	}

	res, err := s.Wait()
	if err != nil {
		t.Fatalf("finish() error = %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestSessionFinishPrefersRealExitOverLaterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ptmx, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer ptmx.Close()

	cmd := exec.Command("sh", "-c", "exit 9")
	waitErr := cmd.Run()
	if waitErr == nil {
		t.Fatal("cmd.Run() error = nil, want exit error")
	}
	if _, ok := waitErr.(*exec.ExitError); !ok {
		t.Fatalf("cmd.Run() error = %T, want *exec.ExitError", waitErr)
	}

	s := &Session{
		ctx:        ctx,
		ptmx:       ptmx,
		finished:   make(chan struct{}),
		readerDone: make(chan struct{}),
		readNotify: make(chan struct{}, 1),
	}
	s.buf.WriteString("real-exit-tail\n")
	close(s.readerDone)

	cancel()
	s.finish(waitErr)

	res, err := s.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if !strings.Contains(res.Transcript, "real-exit-tail") {
		t.Fatalf("Transcript missing tail: %q", res.Transcript)
	}
	if res.ExitCode != 9 {
		t.Fatalf("ExitCode = %d, want 9", res.ExitCode)
	}
}

func TestSessionFinishPrefersRealZeroExitOverLaterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ptmx, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer ptmx.Close()

	s := &Session{
		ctx:        ctx,
		ptmx:       ptmx,
		finished:   make(chan struct{}),
		readerDone: make(chan struct{}),
		readNotify: make(chan struct{}, 1),
	}
	s.buf.WriteString("real-zero-tail\n")
	close(s.readerDone)

	cancel()
	s.finish(nil)

	res, waitErr := s.Wait()
	if waitErr != nil {
		t.Fatalf("Wait() error = %v", waitErr)
	}
	if !strings.Contains(res.Transcript, "real-zero-tail") {
		t.Fatalf("Transcript missing tail: %q", res.Transcript)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func helperTranscriptBurst() {
	fmt.Print(helperTranscriptBurstOutput())
	os.Exit(0)
}

func helperTranscriptBurstOutput() string {
	var b strings.Builder
	for i := 0; i < 4096; i++ {
		fmt.Fprintf(&b, "chunk-%04d\n", i)
	}
	b.WriteString("helper-footer\n")
	return b.String()
}

func helperSpawnDescendantHoldingPTY() {
	ttyPath, err := os.Readlink("/proc/self/fd/1")
	if err != nil {
		fmt.Printf("tty-readlink-error: %v\n", err)
		os.Exit(1)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestSessionWaitReturnsWhenDescendantKeepsPTYOpen")
	cmd.Env = append(
		filteredEnv("GO_WANT_SESSION_DESCENDANT_PARENT"),
		"GO_WANT_SESSION_DESCENDANT_CHILD=1",
		"GO_WANT_SESSION_DESCENDANT_TTY="+ttyPath,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Printf("spawn-error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("parent-exit")
	os.Exit(0)
}

func filteredEnv(dropKey string) []string {
	prefix := dropKey + "="
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, prefix) {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func sessionTestReadLoop(s *Session, ptmx *os.File) {
	defer close(s.readerDone)
	buf := make([]byte, 64)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			s.mu.Lock()
			s.buf.Write(buf[:n])
			s.mu.Unlock()
			s.signalRead()
		}
		if err != nil {
			return
		}
	}
}

func normalizePTyTranscript(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func sendFakeAgentPrompt(t *testing.T, sess *Session) {
	t.Helper()
	if err := sess.SendInput(strings.Join([]string{
		"You are running inside a local PTY automation wrapper.",
		"POST JSON to http://127.0.0.1:4321/callback with token tok-123.",
		"User task:",
		"session test",
		"",
	}, "\n")); err != nil {
		t.Fatalf("SendInput() error = %v", err)
	}
}

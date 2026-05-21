package ptyio

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
)

type Result struct {
	Transcript string
	ExitCode   int
}

type Session struct {
	ctx        context.Context
	cmd        *exec.Cmd
	ptmx       *os.File
	finished   chan struct{}
	readerDone chan struct{}
	readNotify chan struct{}
	buf        bytes.Buffer
	mu         sync.Mutex
	result     Result
	waitErr    error
	finishOnce sync.Once
	closeOnce  sync.Once
}

var readerDrainGrace = 100 * time.Millisecond

func Start(ctx context.Context, argv []string) (*Session, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = interactiveEnv(os.Environ())
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	s := &Session{
		ctx:        ctx,
		cmd:        cmd,
		ptmx:       ptmx,
		finished:   make(chan struct{}),
		readerDone: make(chan struct{}),
		readNotify: make(chan struct{}, 1),
	}
	go func() {
		defer close(s.readerDone)
		buf := make([]byte, 4096)
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
	}()
	go func() {
		s.finish(cmd.Wait())
	}()
	return s, nil
}

func (s *Session) Wait() (Result, error) {
	<-s.finished
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result, s.waitErr
}

func (s *Session) Snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *Session) SendInput(data string) error {
	if s.ptmx == nil {
		return os.ErrClosed
	}
	_, err := s.ptmx.Write([]byte(data))
	return err
}

func (s *Session) finish(err error) {
	s.finishOnce.Do(func() {
		defer func() {
			s.closePTMX()
			close(s.finished)
		}()

		code := 0
		canceled := false
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if s.ctx != nil && s.ctx.Err() != nil && exitErr.ExitCode() == -1 {
					canceled = true
				}
				code = exitErr.ExitCode()
			} else {
				s.waitErr = err
				return
			}
		}

		s.waitForReaderDrain()

		s.mu.Lock()
		s.result = Result{Transcript: s.buf.String(), ExitCode: code}
		if canceled {
			s.waitErr = s.ctx.Err()
		}
		s.mu.Unlock()
	})
}

func (s *Session) signalRead() {
	if s.readNotify == nil {
		return
	}
	select {
	case s.readNotify <- struct{}{}:
	default:
	}
}

func (s *Session) waitForReaderDrain() {
	timer := time.NewTimer(readerDrainGrace)
	defer timer.Stop()

	for {
		select {
		case <-s.readerDone:
			return
		case <-timer.C:
			s.closePTMX()
			<-s.readerDone
			return
		case <-s.readNotify:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(readerDrainGrace)
		}
	}
}

func (s *Session) closePTMX() {
	if s.ptmx == nil {
		return
	}
	s.closeOnce.Do(func() {
		_ = s.ptmx.Close()
	})
}

func interactiveEnv(base []string) []string {
	env := append([]string{}, base...)
	for i, kv := range env {
		if kv == "TERM=dumb" {
			env[i] = "TERM=xterm-256color"
			return env
		}
		if len(kv) > len("TERM=") && kv[:5] == "TERM=" {
			return env
		}
	}
	return append(env, "TERM=xterm-256color")
}

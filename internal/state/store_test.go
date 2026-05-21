package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/felix021/agentcall/internal/sharedtypes"
)

func TestStoreCreatesRestrictedArtifactPermissions(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "artifacts"), filepath.Join(dir, "artifacts", "status.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	info, err := os.Stat(store.ArtifactsDir())
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm = %#o, want %#o", info.Mode().Perm(), 0o700)
	}
}

func TestStoreTightensExistingArtifactPermissions(t *testing.T) {
	dir := t.TempDir()
	artifactsDir := filepath.Join(dir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Chmod(artifactsDir, 0o755); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	store, err := NewStore(artifactsDir, filepath.Join(artifactsDir, "status.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	info, err := os.Stat(store.ArtifactsDir())
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm = %#o, want %#o", info.Mode().Perm(), 0o700)
	}
}

func TestWriteStatusIsAtomicAndReadable(t *testing.T) {
	dir := t.TempDir()
	statusPath := filepath.Join(dir, "status.json")
	store, err := NewStore(filepath.Join(dir, "artifacts"), statusPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	status := sharedtypes.ResultEnvelope{
		RunID:    "run-1",
		State:    "active",
		Status:   "ok",
		ExitCode: 0,
	}
	if err := store.WriteStatus(status); err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}
	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var decoded sharedtypes.ResultEnvelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.RunID != "run-1" {
		t.Fatalf("RunID = %q, want run-1", decoded.RunID)
	}
	info, err := os.Stat(statusPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("status perm = %#o, want %#o", info.Mode().Perm(), 0o600)
	}
}

func TestWriteStatusCreatesMissingParentDirectory(t *testing.T) {
	dir := t.TempDir()
	statusPath := filepath.Join(dir, "nested", "status", "status.json")
	store, err := NewStore(filepath.Join(dir, "artifacts"), statusPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	status := sharedtypes.ResultEnvelope{
		RunID:    "run-2",
		State:    "active",
		Status:   "ok",
		ExitCode: 0,
	}
	if err := store.WriteStatus(status); err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}

	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var decoded sharedtypes.ResultEnvelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.RunID != "run-2" {
		t.Fatalf("RunID = %q, want run-2", decoded.RunID)
	}
}

func TestWriteStatusTightensExistingParentDirectory(t *testing.T) {
	dir := t.TempDir()
	statusDir := filepath.Join(dir, "nested", "status")
	if err := os.MkdirAll(statusDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Chmod(statusDir, 0o755); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	statusPath := filepath.Join(statusDir, "status.json")
	store, err := NewStore(filepath.Join(dir, "artifacts"), statusPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	status := sharedtypes.ResultEnvelope{
		RunID:    "run-3",
		State:    "active",
		Status:   "ok",
		ExitCode: 0,
	}
	if err := store.WriteStatus(status); err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}

	info, err := os.Stat(statusDir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm = %#o, want %#o", info.Mode().Perm(), 0o700)
	}
}

func TestWriteStatusWithRelativePathDoesNotChmodWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer os.Chdir(oldWD)

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	store, err := NewStore(filepath.Join(dir, "artifacts"), "status.json")
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	status := sharedtypes.ResultEnvelope{
		RunID:    "run-relative",
		State:    "active",
		Status:   "ok",
		ExitCode: 0,
	}
	if err := store.WriteStatus(status); err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("working dir perm = %#o, want %#o", info.Mode().Perm(), 0o755)
	}
}

func TestWriteStatusSupportsConcurrentWriters(t *testing.T) {
	dir := t.TempDir()
	statusPath := filepath.Join(dir, "status.json")
	store, err := NewStore(filepath.Join(dir, "artifacts"), statusPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	const writers = 16
	const writesPerWriter = 32

	errCh := make(chan error, writers*writesPerWriter)
	var readerWG sync.WaitGroup
	var writerWG sync.WaitGroup
	done := make(chan struct{})

	readerWG.Add(1)
	go func() {
		defer readerWG.Done()
		for {
			select {
			case <-done:
				return
			default:
			}

			raw, err := os.ReadFile(statusPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				errCh <- err
				return
			}
			if len(raw) == 0 {
				errCh <- os.ErrInvalid
				return
			}
			var decoded sharedtypes.ResultEnvelope
			if err := json.Unmarshal(raw, &decoded); err != nil {
				errCh <- err
				return
			}
		}
	}()

	for writer := range writers {
		writerWG.Add(1)
		go func(writer int) {
			defer writerWG.Done()
			for write := range writesPerWriter {
				status := sharedtypes.ResultEnvelope{
					RunID:    "run-concurrent",
					State:    "active",
					Status:   "ok",
					ExitCode: writer + write,
				}
				if err := store.WriteStatus(status); err != nil {
					errCh <- err
				}
			}
		}(writer)
	}

	writerWG.Wait()
	close(done)
	readerWG.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("WriteStatus() error = %v", err)
	}

	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var decoded sharedtypes.ResultEnvelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.RunID != "run-concurrent" {
		t.Fatalf("RunID = %q, want run-concurrent", decoded.RunID)
	}
}

func TestAppendTranscriptPersistsDataAndRestrictsPermissions(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "artifacts"), filepath.Join(dir, "artifacts", "status.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if err := store.AppendTranscript([]byte("first line\n")); err != nil {
		t.Fatalf("AppendTranscript() first error = %v", err)
	}
	if err := store.AppendTranscript([]byte("second line\n")); err != nil {
		t.Fatalf("AppendTranscript() second error = %v", err)
	}

	transcriptPath := filepath.Join(store.ArtifactsDir(), "transcript.log")
	raw, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := string(raw), "first line\nsecond line\n"; got != want {
		t.Fatalf("transcript = %q, want %q", got, want)
	}

	info, err := os.Stat(transcriptPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("transcript perm = %#o, want %#o", info.Mode().Perm(), 0o600)
	}
}

func TestAppendTranscriptTightensExistingFilePermissions(t *testing.T) {
	dir := t.TempDir()
	artifactsDir := filepath.Join(dir, "artifacts")
	store, err := NewStore(artifactsDir, filepath.Join(artifactsDir, "status.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	transcriptPath := filepath.Join(store.ArtifactsDir(), "transcript.log")
	if err := os.WriteFile(transcriptPath, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(transcriptPath, 0o644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	if err := store.AppendTranscript([]byte("next\n")); err != nil {
		t.Fatalf("AppendTranscript() error = %v", err)
	}

	raw, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := string(raw), "existing\nnext\n"; got != want {
		t.Fatalf("transcript = %q, want %q", got, want)
	}

	info, err := os.Stat(transcriptPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("transcript perm = %#o, want %#o", info.Mode().Perm(), 0o600)
	}
}

package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/felix021/agentcall/internal/sharedtypes"
)

type Store struct {
	artifactsDir string
	statusPath   string
	transcript   string
	cleanText    string
}

func NewStore(artifactsDir, statusPath string) (*Store, error) {
	if err := os.MkdirAll(artifactsDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(artifactsDir, 0o700); err != nil {
		return nil, err
	}
	return &Store{
		artifactsDir: artifactsDir,
		statusPath:   statusPath,
		transcript:   filepath.Join(artifactsDir, "transcript.log"),
		cleanText:    filepath.Join(artifactsDir, "transcript.txt"),
	}, nil
}

func (s *Store) ArtifactsDir() string { return s.artifactsDir }

func (s *Store) AppendTranscript(data []byte) error {
	f, err := os.OpenFile(s.transcript, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

func (s *Store) WriteCleanTranscript(text string) error {
	return os.WriteFile(s.cleanText, []byte(text), 0o600)
}

func (s *Store) WriteStatus(status sharedtypes.ResultEnvelope) error {
	raw, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	statusDir := filepath.Dir(s.statusPath)
	if statusDir != "." {
		if err := os.MkdirAll(statusDir, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(statusDir, 0o700); err != nil {
			return err
		}
	}

	tmpFile, err := os.CreateTemp(statusDir, filepath.Base(s.statusPath)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := tmpFile.Name()
	defer os.Remove(tmp)

	if _, err := tmpFile.Write(raw); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.statusPath)
}

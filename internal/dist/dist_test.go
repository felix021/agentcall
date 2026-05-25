package dist

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReleaseTargets(t *testing.T) {
	want := []Target{
		{GOOS: "linux", GOARCH: "amd64"},
		{GOOS: "linux", GOARCH: "arm64"},
		{GOOS: "darwin", GOARCH: "amd64"},
		{GOOS: "darwin", GOARCH: "arm64"},
		{GOOS: "windows", GOARCH: "amd64"},
		{GOOS: "windows", GOARCH: "arm64"},
	}

	if got := ReleaseTargets(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ReleaseTargets() = %#v, want %#v", got, want)
	}
}

func TestArchiveNameUsesVersionAndTarget(t *testing.T) {
	got := ArchiveName("v0.1.0", Target{GOOS: "linux", GOARCH: "amd64"})
	want := "agentcall_v0.1.0_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("ArchiveName() = %q, want %q", got, want)
	}
}

func TestCreateArchiveTarGzIncludesBinaryAndSkill(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := writeTempFile(t, tmpDir, "agentcall", "linux-binary")
	skillPath := writeTempFile(t, tmpDir, filepath.Join("skills", "agentcall", "SKILL.md"), "skill-doc")
	outDir := filepath.Join(tmpDir, "dist")

	archivePath, err := CreateArchive("v0.1.0", Target{GOOS: "linux", GOARCH: "amd64"}, binaryPath, skillPath, outDir)
	if err != nil {
		t.Fatalf("CreateArchive() error = %v", err)
	}

	if got, want := filepath.Base(archivePath), "agentcall_v0.1.0_linux_amd64.tar.gz"; got != want {
		t.Fatalf("archive basename = %q, want %q", got, want)
	}

	entries := tarGzEntries(t, archivePath)
	want := []string{
		"agentcall",
		"skills/agentcall/SKILL.md",
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("archive entries = %#v, want %#v", entries, want)
	}
}

func TestCreateArchiveZipIncludesExeAndSkill(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := writeTempFile(t, tmpDir, "agentcall.exe", "windows-binary")
	skillPath := writeTempFile(t, tmpDir, filepath.Join("skills", "agentcall", "SKILL.md"), "skill-doc")
	outDir := filepath.Join(tmpDir, "dist")

	archivePath, err := CreateArchive("v0.1.0", Target{GOOS: "windows", GOARCH: "arm64"}, binaryPath, skillPath, outDir)
	if err != nil {
		t.Fatalf("CreateArchive() error = %v", err)
	}

	if got, want := filepath.Base(archivePath), "agentcall_v0.1.0_windows_arm64.zip"; got != want {
		t.Fatalf("archive basename = %q, want %q", got, want)
	}

	entries := zipEntries(t, archivePath)
	want := []string{
		"agentcall.exe",
		"skills/agentcall/SKILL.md",
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("archive entries = %#v, want %#v", entries, want)
	}
}

func writeTempFile(t *testing.T, root, rel, content string) string {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}

func tarGzEntries(t *testing.T, path string) []string {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", path, err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip.NewReader(%q) error = %v", path, err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var entries []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return entries
		}
		if err != nil {
			t.Fatalf("tar.Next(%q) error = %v", path, err)
		}
		entries = append(entries, hdr.Name)
	}
}

func zipEntries(t *testing.T, path string) []string {
	t.Helper()

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader(%q) error = %v", path, err)
	}
	defer zr.Close()

	entries := make([]string, 0, len(zr.File))
	for _, file := range zr.File {
		entries = append(entries, file.Name)
	}
	return entries
}

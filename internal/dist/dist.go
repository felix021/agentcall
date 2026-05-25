package dist

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

type Target struct {
	GOOS   string
	GOARCH string
}

func ReleaseTargets() []Target {
	return []Target{
		{GOOS: "linux", GOARCH: "amd64"},
		{GOOS: "linux", GOARCH: "arm64"},
		{GOOS: "darwin", GOARCH: "amd64"},
		{GOOS: "darwin", GOARCH: "arm64"},
		{GOOS: "windows", GOARCH: "amd64"},
		{GOOS: "windows", GOARCH: "arm64"},
	}
}

func BinaryName(target Target) string {
	if target.GOOS == "windows" {
		return "agentcall.exe"
	}
	return "agentcall"
}

func ArchiveName(version string, target Target) string {
	ext := ".tar.gz"
	if target.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("agentcall_%s_%s_%s%s", version, target.GOOS, target.GOARCH, ext)
}

func CreateArchive(version string, target Target, binaryPath, skillPath, outDir string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}

	archivePath := filepath.Join(outDir, ArchiveName(version, target))
	files := []archiveFile{
		{
			SrcPath:  binaryPath,
			DestPath: BinaryName(target),
			Mode:     0o755,
		},
		{
			SrcPath:  skillPath,
			DestPath: path.Join("skills", "agentcall", "SKILL.md"),
			Mode:     0o644,
		},
	}

	if target.GOOS == "windows" {
		return archivePath, createZipArchive(archivePath, files)
	}
	return archivePath, createTarGzArchive(archivePath, files)
}

type archiveFile struct {
	SrcPath  string
	DestPath string
	Mode     int64
}

func createTarGzArchive(archivePath string, files []archiveFile) error {
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()

	gzw := gzip.NewWriter(out)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, file := range files {
		if err := addTarFile(tw, file); err != nil {
			return err
		}
	}
	return nil
}

func addTarFile(tw *tar.Writer, file archiveFile) error {
	src, err := os.Open(file.SrcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name: file.DestPath,
		Mode: file.Mode,
		Size: info.Size(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, src)
	return err
}

func createZipArchive(archivePath string, files []archiveFile) error {
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	for _, file := range files {
		if err := addZipFile(zw, file); err != nil {
			return err
		}
	}
	return nil
}

func addZipFile(zw *zip.Writer, file archiveFile) error {
	src, err := os.Open(file.SrcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	hdr := &zip.FileHeader{
		Name:   file.DestPath,
		Method: zip.Deflate,
	}
	hdr.SetMode(os.FileMode(file.Mode))

	writer, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, src)
	return err
}

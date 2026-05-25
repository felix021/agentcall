package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/felix021/agentcall/internal/dist"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	version := flag.String("version", "", "")
	outputDir := flag.String("output", "dist", "")
	flag.Parse()

	if *version == "" {
		return fmt.Errorf("version required")
	}

	root, err := os.Getwd()
	if err != nil {
		return err
	}

	skillPath := filepath.Join(root, "skills", "agentcall", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		return err
	}

	stageRoot, err := os.MkdirTemp("", "agentcall-dist-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageRoot)

	for _, target := range dist.ReleaseTargets() {
		binaryPath := filepath.Join(stageRoot, target.GOOS+"-"+target.GOARCH, dist.BinaryName(target))
		if err := buildBinary(root, target, binaryPath); err != nil {
			return err
		}
		archivePath, err := dist.CreateArchive(*version, target, binaryPath, skillPath, *outputDir)
		if err != nil {
			return err
		}
		fmt.Println(archivePath)
	}

	return nil
}

func buildBinary(root string, target dist.Target, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/agentcall")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

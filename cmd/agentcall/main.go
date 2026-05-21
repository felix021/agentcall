package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/felix021/agentcall/internal/runner"
)

var runRunner = runner.Run

func parseRunArgs(args []string, stderr io.Writer) (runner.RunInput, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.String("timeout", "90s", "")
	artifactsDir := fs.String("artifacts-dir", "", "")
	statusFile := fs.String("status-file", "", "")
	autoTrust := fs.Bool("auto-trust", false, "")
	prompt := fs.String("prompt", "", "")
	if err := fs.Parse(args); err != nil {
		return runner.RunInput{}, err
	}
	if fs.NArg() == 0 {
		return runner.RunInput{}, fmt.Errorf("command required")
	}
	parsedTimeout, err := time.ParseDuration(*timeout)
	if err != nil {
		return runner.RunInput{}, err
	}
	if parsedTimeout <= 0 {
		return runner.RunInput{}, fmt.Errorf("timeout must be greater than zero")
	}
	return runner.RunInput{
		Command:      fs.Args(),
		Prompt:       *prompt,
		ArtifactsDir: *artifactsDir,
		StatusFile:   *statusFile,
		Timeout:      parsedTimeout,
		AutoTrust:    *autoTrust,
	}, nil
}

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "run" {
		fmt.Fprintln(stderr, "usage: agentcall run -- <command>")
		return 1
	}
	input, err := parseRunArgs(args[1:], stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	res, err := runRunner(context.Background(), input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	if err := enc.Encode(res); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return res.ExitCode
}

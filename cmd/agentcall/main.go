package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/felix021/agentcall/internal/runner"
)

var runRunner = runner.Run

type stringFlag struct {
	value string
	set   bool
}

func (f *stringFlag) String() string {
	return f.value
}

func (f *stringFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

type intFlag struct {
	value int
	set   bool
}

func (f *intFlag) String() string {
	return fmt.Sprintf("%d", f.value)
}

func (f *intFlag) Set(value string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	f.value = parsed
	f.set = true
	return nil
}

func parseRunArgs(args []string, stderr io.Writer) (runner.RunInput, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.String("timeout", "90s", "")
	artifactsDir := fs.String("artifacts-dir", "", "")
	statusFile := fs.String("status-file", "", "")
	autoTrust := fs.Bool("auto-trust", false, "")
	prompt := fs.String("prompt", "", "")
	heartbeatPeriod := &stringFlag{value: time.Second.String()}
	fs.Var(heartbeatPeriod, "heartbeat-period", "")
	verbose := &intFlag{value: 1}
	fs.Var(verbose, "verbose", "")
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
	parsedHeartbeatPeriod, err := time.ParseDuration(heartbeatPeriod.value)
	if err != nil {
		return runner.RunInput{}, err
	}
	if parsedHeartbeatPeriod <= 0 {
		return runner.RunInput{}, fmt.Errorf("heartbeat period must be greater than zero")
	}
	if verbose.value < 0 {
		return runner.RunInput{}, fmt.Errorf("verbose must be greater than or equal to zero")
	}
	return runner.RunInput{
		Command:            fs.Args(),
		Prompt:             *prompt,
		ArtifactsDir:       *artifactsDir,
		StatusFile:         *statusFile,
		Timeout:            parsedTimeout,
		AutoTrust:          *autoTrust,
		HeartbeatPeriod:    parsedHeartbeatPeriod,
		HeartbeatPeriodSet: heartbeatPeriod.set,
		Verbose:            verbose.value,
		VerboseSet:         verbose.set,
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
	res, err := runRunner(context.Background(), input, stderr)
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

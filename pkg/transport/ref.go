package transport

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CheckedRunStdout is same as CheckedRun, but it also returns command stdout on success
func CheckedRunStdout(ctx context.Context, tr Transport, cmd []string, opts ...ExecOption) (stdout string, err error) {
	runResult, err := tr.RunCommand(ctx, cmd, opts...)
	if err != nil {
		return "", err
	}
	if runResult.ExitCode != 0 {
		return "", FormatRunError(runResult)
	}
	return string(runResult.Stdout), nil
}

// CheckedRun runs a commands, expecting zero exit code
// If command has exited with a different exit code, it returns an error.
func CheckedRun(ctx context.Context, tr Transport, cmd []string, opts ...ExecOption) error {
	runResult, err := tr.RunCommand(ctx, cmd, append([]ExecOption{WithoutStdout()}, opts...)...)
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return FormatRunError(runResult)
	}
	return nil
}

// RelaxedRun is same as CheckedRun, but it allows non-zero exit code in case command stderr contains specified substring
func RelaxedRun(ctx context.Context, tr Transport, cmd []string, allowStderr string, opts ...ExecOption) (recovered bool, err error) {
	runResult, err := tr.RunCommand(ctx, cmd, append([]ExecOption{WithoutStdout()}, opts...)...)
	if err != nil {
		return false, err
	}
	if runResult.ExitCode != 0 {
		stderr := string(runResult.Stderr)
		if strings.Contains(stderr, allowStderr) {
			return true, nil
		}
		return false, FormatRunError(runResult)
	}
	return false, nil
}

// FormatRunError creates a human-readable error from CommandResult
func FormatRunError(r RunResult) error {
	if len(r.Stderr) != 0 {
		stderr := strings.TrimSpace(string(r.Stderr))
		return fmt.Errorf("exit code %d: stderr: %s", r.ExitCode, stderr)
	}
	// use stdout as fallback if stderr is missing
	if len(r.Stdout) != 0 {
		stdout := strings.TrimSpace(string(r.Stdout))
		return fmt.Errorf("exit code %d: stdout: %s", r.ExitCode, stdout)
	}
	return fmt.Errorf("exit code %d: no output", r.ExitCode)
}

// Retry runs specified command in a loop
func Retry(period time.Duration, maxTime time.Duration, action func() error) error {
	startTime := time.Now()
	var err error
	for time.Since(startTime) < maxTime {
		err = action()
		if err == nil {
			return nil
		}
		time.Sleep(period)
	}
	return err
}

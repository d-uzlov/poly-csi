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
	result := fmt.Sprintf("exit code %d", r.ExitCode)
	if r.Cancelled {
		if r.Timeout {
			result = "timed out: " + result
		} else {
			result = "cancelled by user: " + result
		}
	}
	if len(r.Stderr) != 0 {
		stderr := strings.TrimSpace(string(r.Stderr))
		return fmt.Errorf("%s: stderr: %s", result, stderr)
	}
	// use stdout as fallback if stderr is missing
	if len(r.Stdout) != 0 {
		stdout := strings.TrimSpace(string(r.Stdout))
		return fmt.Errorf("%s: stdout: %s", result, stdout)
	}
	return fmt.Errorf("%s: no output", result)
}

// Retry runs specified command in a loop.
// See RetryUntil.
func Retry(period time.Duration, maxDuration time.Duration, action func() error) error {
	return RetryUntil(period, time.Now().Add(maxDuration), action)
}

// RetryUntil runs specified command in a loop until specified timeout.
// It runs the command until it returns nil, or until endTime.
// If endTime has already been reached, it runs the command once and immediately finishes.
func RetryUntil(period time.Duration, endTime time.Time, action func() error) error {
	for time.Now().Before(endTime) {
		err := action()
		if err == nil {
			return nil
		}
		time.Sleep(period)
	}
	// if we are here, it means action returned error previously and we reached timeout
	// let's run it one last time
	return action()
}

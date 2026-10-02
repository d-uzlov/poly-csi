package localexec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"al.essio.dev/pkg/shellescape"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/executils"
)

const defaultTimeout = time.Second

type Config struct {
}

type localExecWrapper struct {
	isRoot   bool
	config   Config
	commands *transport.ExecReplacement
}

func New(config Config) transport.Transport {
	uid := os.Getuid()
	isRoot := uid == 0
	result := &localExecWrapper{
		isRoot:   isRoot,
		config:   config,
		commands: transport.NewExecReplacement(map[string][]string{}),
	}

	return result
}

func (s *localExecWrapper) Cleanup() {}

func (s *localExecWrapper) Cmds() transport.Commands {
	return newNativeCommands(s)
}

func (s *localExecWrapper) SudoCmds() transport.Commands {
	if s.isRoot {
		return newNativeCommands(s)
	}
	// if we are running as generic user but need sudo privileged,
	// fall back to shell commands with sudo prefix
	return executils.New(s, true)
}

func (s *localExecWrapper) SetCommands(commands *transport.ExecReplacement) {
	s.commands = commands
}

func (s *localExecWrapper) RunCommand(ctx context.Context, cmd []string, opts ...transport.ExecOption) (transport.RunResult, error) {
	if ctx.Err() != nil {
		return transport.RunResult{}, ctx.Err()
	}
	if len(cmd) == 0 {
		return transport.RunResult{}, fmt.Errorf("cmd is empty")
	}

	options := transport.ExecParams{
		Timeout:     new(defaultTimeout),
		ExitTimeout: time.Second,
		Sudo:        false,
	}
	for _, v := range opts {
		v(&options)
	}

	if s.isRoot {
		options.Sudo = false
	}

	cmd = append(s.commands.Lookup(cmd[0], options.Sudo), cmd[1:]...)

	cmdString := shellescape.QuoteCommand(cmd)
	logs.FromCtx(ctx).Command("local exec: running command", "cmd", cmdString)

	ec := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	for k, v := range options.Env {
		ec.Env = append(ec.Env, k+"="+v)
	}

	ec.Cancel = func() error {
		return ec.Process.Signal(syscall.SIGTERM)
	}
	ec.WaitDelay = options.ExitTimeout

	stderrBuf := bytes.Buffer{}
	ec.Stderr = &stderrBuf

	var stdoutDoneCh chan struct{}
	stdoutBuf := bytes.Buffer{}
	if options.StreamingStdoutLines != nil {
		reader, err := ec.StdoutPipe()
		if err != nil {
			return transport.RunResult{}, fmt.Errorf("StdoutPipe: %w", err)
		}
		stdoutDoneCh = make(chan struct{})
		go func() {
			defer close(stdoutDoneCh)
			s := bufio.NewScanner(reader)
			for s.Scan() {
				options.StreamingStdoutLines(s.Text())
			}
			_ = s.Err()
		}()
	} else if options.DisableStdout {
		ec.Stdout = nil
	} else if options.StdoutFile != "" {
		file, err := os.Open(options.StdoutFile)
		if err != nil {
			return transport.RunResult{}, fmt.Errorf("open '%s' for stdout: %w", options.StdinFile, err)
		}
		defer file.Close()
		ec.Stdout = file
	} else {
		ec.Stdout = &stdoutBuf
	}

	if options.StdinBytes != nil {
		ec.Stdin = bytes.NewBuffer(options.StdinBytes)
	} else if options.StdinFile != "" {
		file, err := os.Open(options.StdinFile)
		if err != nil {
			return transport.RunResult{}, fmt.Errorf("open '%s' for stdin: %w", options.StdinFile, err)
		}
		defer file.Close()
		ec.Stdin = file
	}

	start := time.Now()
	if options.Timeout != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *options.Timeout)
		defer cancel()
	}

	err := ec.Start()
	if err != nil {
		return transport.RunResult{}, fmt.Errorf("exec start: %w", err)
	}

	commandDoneCh := make(chan struct{})
	var waitErr error
	go func() {
		defer close(commandDoneCh)
		if stdoutDoneCh != nil {
			<-stdoutDoneCh
		}
		waitErr = ec.Wait()
	}()

	cancelled := false

	select {
	case <-commandDoneCh:
	case <-ctx.Done():
		cancelled = true
		// no need to do anything, exec.Cmd will send signals on its own
	}

	// give the program some time to terminate
	select {
	case <-commandDoneCh:
		// using double timeout: after ExitTimeout exec.Cmd will send sigkill,
		// and then we want to wait a bit to get command result
	case <-time.Tick(2 * options.ExitTimeout):
		return transport.RunResult{}, fmt.Errorf("timed out waiting for command to finish")
	}

	result := transport.RunResult{
		Stdout:    stdoutBuf.Bytes(),
		Stderr:    stderrBuf.Bytes(),
		ExitCode:  0,
		Cancelled: cancelled,
	}
	if options.Timeout != nil {
		result.Timeout = time.Since(start) > *options.Timeout
	}

	if waitErr != nil {
		var exitErr *exec.ExitError
		if ok := errors.As(waitErr, &exitErr); !ok {
			return transport.RunResult{}, err
		}

		result.ExitCode = exitErr.ExitCode()
	}

	return result, nil
}

package localexec

import (
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
	isRoot bool
	config Config
}

func New(config Config) transport.Transport {
	uid := os.Getuid()
	isRoot := uid == 0
	result := &localExecWrapper{
		isRoot: isRoot,
		config: config,
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

func (s *localExecWrapper) RunCommand(ctx context.Context, cmd []string, opts ...transport.ExecOption) (transport.CommandResult, error) {
	if ctx.Err() != nil {
		return transport.CommandResult{}, ctx.Err()
	}
	if len(cmd) == 0 {
		return transport.CommandResult{}, fmt.Errorf("cmd is empty")
	}

	options := transport.ExecParams{}
	for _, v := range opts {
		v(&options)
	}
	if options.Timeout == 0 {
		options.Timeout = defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	cmdString := shellescape.QuoteCommand(cmd)
	logs.FromCtx(ctx).Command("local exec: running command", "cmd", cmdString)

	ec := exec.CommandContext(ctx, cmd[0], cmd[1:]...)

	stderrBuf := bytes.Buffer{}
	ec.Stderr = &stderrBuf

	stdoutBuf := bytes.Buffer{}
	if options.DisableStdout {
		ec.Stdout = nil
	} else if options.StdoutFile != "" {
		file, err := os.Open(options.StdoutFile)
		if err != nil {
			return transport.CommandResult{}, fmt.Errorf("open '%s' for stdout: %w", options.StdinFile, err)
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
			return transport.CommandResult{}, fmt.Errorf("open '%s' for stdin: %w", options.StdinFile, err)
		}
		defer file.Close()
		ec.Stdin = file
	}

	err := ec.Start()
	if err != nil {
		return transport.CommandResult{}, fmt.Errorf("exec start: %w", err)
	}

	// adding a buffer of size 1 to be able to send without blocking even if main thread stopped reading
	commandDoneCh := make(chan error, 1)
	go func() {
		defer close(commandDoneCh)
		commandDoneCh <- ec.Wait()
	}()

	select {
	case <-ctx.Done():
		_ = ec.Process.Signal(syscall.SIGTERM)
		return transport.CommandResult{}, ctx.Err()
	case err = <-commandDoneCh:
		if err == nil {
			return transport.CommandResult{
				Stdout:   stdoutBuf.Bytes(),
				Stderr:   stderrBuf.Bytes(),
				ExitCode: 0,
			}, nil
		}

		var exitErr *exec.ExitError
		if ok := errors.As(err, &exitErr); ok {
			return transport.CommandResult{
				Stdout:   stdoutBuf.Bytes(),
				Stderr:   stderrBuf.Bytes(),
				ExitCode: exitErr.ExitCode(),
			}, nil
		}

		return transport.CommandResult{}, fmt.Errorf("run: %w", err)
	}
}

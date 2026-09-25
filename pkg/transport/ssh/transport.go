package ssh_wrapper

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/ssh"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/executils"

	"al.essio.dev/pkg/shellescape"
)

func (s *sshWrapper) Cmds() transport.Commands {
	return executils.New(s, false)
}

func (s *sshWrapper) SudoCmds() transport.Commands {
	return executils.New(s, true)
}

func (s *sshWrapper) RunCommand(ctx context.Context, cmd []string, opts ...transport.ExecOption) (transport.RunResult, error) {
	if ctx.Err() != nil {
		return transport.RunResult{}, ctx.Err()
	}
	if len(cmd) == 0 {
		return transport.RunResult{}, fmt.Errorf("cmd is empty")
	}

	options := transport.ExecParams{}
	for _, v := range opts {
		v(&options)
	}
	if options.Timeout == 0 {
		options.Timeout = defaultTimeout
	}

	cmd = append(s.commands.Lookup(cmd[0], options.Sudo), cmd[1:]...)

	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	cmdString := shellescape.QuoteCommand(cmd)
	logs.FromCtx(ctx).Command("ssh: running command", "cmd", cmdString, "ssh-target", s.sshLogId)

	client := s.GetClient()
	if client == nil {
		return transport.RunResult{}, fmt.Errorf("ssh client is not initialized")
	}
	session, err := client.NewSession()
	if err != nil {
		if checkIoError(err) {
			// let's close the client manually, in case it's not already closed
			// client should be automatically recreated by the background thread
			go client.Close()
		}
		return transport.RunResult{}, fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	for k, v := range options.Env {
		session.Setenv(k, v)
	}

	stderrBuf := bytes.Buffer{}
	session.Stderr = &stderrBuf

	stdoutBuf := bytes.Buffer{}
	if options.DisableStdout {
		session.Stdout = nil
		cmdString = fmt.Sprintf("%s > /dev/null", cmdString)
	} else if options.StdoutFile != "" {
		cmdString = fmt.Sprintf("%s > %s", cmdString, shellescape.Quote(options.StdoutFile))
	} else {
		session.Stdout = &stdoutBuf
	}

	if options.StdinBytes != nil {
		session.Stdin = bytes.NewBuffer(options.StdinBytes)
	} else if options.StdinFile != "" {
		cmdString = fmt.Sprintf("%s < %s", cmdString, shellescape.Quote(options.StdinFile))
	}

	err = session.Start(cmdString)
	if err != nil {
		return transport.RunResult{}, fmt.Errorf("start process: %w", err)
	}

	// adding a buffer of size 1 to be able to send without blocking even if main thread stopped reading
	commandDoneCh := make(chan error, 1)
	go func() {
		defer close(commandDoneCh)
		commandDoneCh <- session.Wait()
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		// TODO do we need to wait until command actually finishes?
		return transport.RunResult{}, ctx.Err()
	case err := <-commandDoneCh:
		if err == nil {
			return transport.RunResult{
				Stdout:   stdoutBuf.Bytes(),
				Stderr:   stderrBuf.Bytes(),
				ExitCode: 0,
			}, nil
		}

		var exitErr *ssh.ExitError
		if ok := errors.As(err, &exitErr); ok {
			return transport.RunResult{
				Stdout:   stdoutBuf.Bytes(),
				Stderr:   stderrBuf.Bytes(),
				ExitCode: exitErr.ExitStatus(),
			}, nil
		}

		if checkIoError(err) {
			go client.Close()
		}

		return transport.RunResult{}, fmt.Errorf("run: %w", err)
	}
}

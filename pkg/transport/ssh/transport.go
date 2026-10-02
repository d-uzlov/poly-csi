package ssh_wrapper

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

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

	options := transport.ExecParams{
		Timeout:     new(defaultTimeout),
		ExitTimeout: time.Second,
		Sudo:        false,
	}
	for _, v := range opts {
		v(&options)
	}

	cmd = append(s.commands.Lookup(cmd[0], options.Sudo), cmd[1:]...)

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
	if options.StreamingStdoutLines != nil {
		reader, err := session.StdoutPipe()
		if err != nil {
			return transport.RunResult{}, fmt.Errorf("StdoutPipe: %w", err)
		}
		go func() {
			s := bufio.NewScanner(reader)
			for s.Scan() {
				options.StreamingStdoutLines(s.Text())
			}
			_ = s.Err()
		}()
	} else if options.DisableStdout {
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

	start := time.Now()
	if options.Timeout != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *options.Timeout)
		defer cancel()
	}

	err = session.Start(cmdString)
	if err != nil {
		return transport.RunResult{}, fmt.Errorf("start process: %w", err)
	}

	commandDoneCh := make(chan struct{})
	var waitErr error
	go func() {
		defer close(commandDoneCh)
		waitErr = session.Wait()
	}()

	select {
	case <-commandDoneCh:
	case <-ctx.Done():
		err = session.Signal(ssh.SIGTERM)
		if err != nil {
			return transport.RunResult{}, fmt.Errorf("send SIGTERM on ctx.Done(): %w", err)
		}
	}

	cancelled := false

	select {
	case <-commandDoneCh:
	case <-time.Tick(options.ExitTimeout):
		err = session.Signal(ssh.SIGKILL)
		if err != nil {
			return transport.RunResult{}, fmt.Errorf("send SIGKILL on ctx.Done(): %w", err)
		}
		return transport.RunResult{}, fmt.Errorf("timed out waiting for command to finish")
	}

	select {
	case <-commandDoneCh:
	case <-time.Tick(options.ExitTimeout):
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
		var exitErr *ssh.ExitError
		if ok := errors.As(waitErr, &exitErr); !ok {
			if checkIoError(err) {
				go client.Close()
			}

			return transport.RunResult{}, err
		}

		result.ExitCode = exitErr.ExitStatus()
	}

	return result, nil
}

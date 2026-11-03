package ssh_wrapper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"meoe.io/poly-csi/pkg/transport"

	slogctx "github.com/veqryn/slog-context"
)

const defaultTimeout = time.Second

type wrapper struct {
	watcherCtx      context.Context
	watcherCancel   func()
	config          Config
	sshClientConfig ssh.ClientConfig
	clientMutex     sync.Mutex
	client          *ssh.Client
	sshLogId        string
	logger          *slog.Logger
	commands        transport.ExecReplacement
}

func New(ctx context.Context, config Config) (transport.Transport, error) {
	if !config.verified {
		return nil, fmt.Errorf("config is not validated")
	}

	result := &wrapper{
		config:   config,
		sshLogId: fmt.Sprintf("%s@%s", config.Username, config.ServerAddress),
		commands: transport.NewExecReplacement(map[string][]string{}),
	}

	result.logger = slogctx.FromCtx(ctx).With("ssh-target", result.sshLogId)
	result.logger.Info("creating new SshConnection", "config", config)

	authMethods := []ssh.AuthMethod{}
	for i, a := range config.Auth {
		resolvedValue := ""
		// config.Validate must ensure that only one options is used
		if a.ValueFromEnv != "" {
			resolvedValue = os.Getenv(a.ValueFromEnv)
		}
		if a.ValueFromFile != "" {
			bytes, err := os.ReadFile(a.ValueFromFile)
			if err != nil {
				result.logger.Error(fmt.Sprintf("auth[i=%d/name=%s]: could not read file", i, a.Name), "file", a.ValueFromFile, "error", err.Error())
				continue
			}
			resolvedValue = string(bytes)
		}
		if resolvedValue == "" {
			result.logger.Error(fmt.Sprintf("auth[i=%d/name=%s]: value is empty", i, a.Name))
			continue
		}
		switch a.Type {
		case "password":
			authMethods = append(authMethods, ssh.Password(resolvedValue))
		case "key":
			signer, err := ssh.ParsePrivateKey([]byte(resolvedValue))
			if err != nil {
				result.logger.Error(fmt.Sprintf("auth[i=%d/name=%s]: unable to parse private key", i, a.Name), "error", err.Error())
				continue
			}
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no auth methods found")
	}

	result.sshClientConfig = ssh.ClientConfig{
		User:    config.Username,
		Auth:    authMethods,
		Timeout: config.DialTimeout,
		// TODO add ability to check host key
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	result.watcherCtx, result.watcherCancel = context.WithCancel(ctx)
	result.watcherCtx = slogctx.With(result.watcherCtx, "thread", "SshWrapper:client-watcher")

	go result.watchClient(result.watcherCtx)

	return result, nil
}

func (s *wrapper) watchClient(ctx context.Context) {
	for ctx.Err() == nil {
		s.restartClient(ctx)
		// TODO add backoff? or remove repeated attempts from logs?
		select {
		case <-ctx.Done():
		case <-time.Tick(time.Second):
		}
	}
	s.logger.Debug("context cancelled, stopping ssh watcher")
}

func (s *wrapper) restartClient(ctx context.Context) {
	clientCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	client, err := s.UpdateClient(clientCtx)
	if err == nil {
		s.logger.Info("SshWrapper.watchClient: UpdateClient created new client")
		err := client.Conn.Wait()
		s.logger.Info("client connection closed", "error", err.Error())
	} else {
		s.logger.Error("SshWrapper.watchClient: UpdateClient failed", "error", err.Error())
	}
}

func (s *wrapper) SetCommands(commands transport.ExecReplacement) {
	s.commands = commands
}

func (s *wrapper) Cleanup() {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if s.watcherCancel != nil {
		s.watcherCancel()
		s.watcherCancel = nil
	}
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
}

func (s *wrapper) GetClient() *ssh.Client {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	return s.client
}

func (s *wrapper) UpdateClient(ctx context.Context) (*ssh.Client, error) {
	s.logger.Debug("SshWrapper.UpdateClient")

	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if s.client != nil {
		s.client.Close()
		s.client = nil
	}

	dialer := &net.Dialer{
		Timeout: s.config.DialTimeout,
		KeepAliveConfig: net.KeepAliveConfig{
			Enable:   true,
			Idle:     s.config.KeepaliveInterval,
			Interval: s.config.KeepaliveInterval,
			Count:    s.config.KeepaliveCount,
		},
	}
	conn, err := dialer.DialContext(ctx, "tcp", s.config.ServerAddress)
	if err != nil {
		return nil, fmt.Errorf("DialContext: %w", err)
	}
	// https://github.com/golang/go/issues/19338#issuecomment-539057790
	err = conn.SetDeadline(time.Now().Add(s.config.DialTimeout))
	if err != nil {
		return nil, fmt.Errorf("conn.SetDeadline before NewClientConn: %w", err)
	}
	// NewClientConn uses address only for passing to sshClientConfig.HostKeyCallback
	c, chans, reqs, err := ssh.NewClientConn(conn, s.config.ServerAddress, &s.sshClientConfig)
	if err != nil {
		return nil, fmt.Errorf("NewClientConn: %w", err)
	}
	err = conn.SetDeadline(time.Time{})
	if err != nil {
		return nil, fmt.Errorf("conn.SetDeadline(0) after NewClientConn: %w", err)
	}
	s.client = ssh.NewClient(c, chans, reqs)
	s.logger.Debug("SshWrapper.UpdateClient: success")
	return s.client, nil
}

func checkIoError(err error) bool {
	var opErr *net.OpError
	var netErr net.Error
	return errors.Is(err, io.EOF) || errors.As(err, &opErr) || errors.As(err, &netErr)
}

type CommandResult struct {
	exitCode int
	err      error
}

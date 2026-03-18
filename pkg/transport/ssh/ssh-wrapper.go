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

type sshWrapper struct {
	ctx             context.Context
	watcherCtx      context.Context
	watcherCancel   func()
	config          SshConfig
	sshClientConfig *ssh.ClientConfig
	clientMutex     sync.Mutex
	client          *ssh.Client
	sshLogId        string
	logger          *slog.Logger
}

func New(ctx context.Context, config SshConfig) (transport.Transport, error) {
	if !config.verified {
		return nil, fmt.Errorf("config is not validated")
	}

	result := &sshWrapper{
		ctx:      ctx,
		config:   config,
		sshLogId: fmt.Sprintf("%s@%s", config.Username, config.ServerAddress),
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

	result.sshClientConfig = &ssh.ClientConfig{
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

func (s *sshWrapper) watchClient(ctx context.Context) {
	for ctx.Err() == nil {
		client := s.GetClient()
		if client != nil {
			err := client.Conn.Wait()
			s.logger.Info("client connection closed", "error", err.Error())
		}
		if ctx.Err() != nil {
			break
		}
		// at this point client is either nil or already closed
		err := s.UpdateClient(client)
		if err != nil {
			s.logger.Error("SshWrapper.watchClient: UpdateClient failed", "error", err.Error())
			// TODO add backoff? or remove repeated attempts from logs?
			time.Sleep(time.Second)
			continue
		}
		s.logger.Info("SshWrapper.watchClient: UpdateClient created new client")
	}
	s.logger.Debug("context cancelled, stopping ssh watcher")
}

func (s *sshWrapper) Cleanup() {
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

func (s *sshWrapper) GetClient() *ssh.Client {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	return s.client
}

func (s *sshWrapper) UpdateClient(client *ssh.Client) error {
	s.logger.Debug("SshWrapper.UpdateClient")

	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()

	if client != nil && client != s.client {
		// someone has already updated client, no need to do anything
		return nil
	}

	if s.client != nil {
		s.client.Close()
		s.client = nil
	}

	dialer := &net.Dialer{
		Timeout: s.config.DialTimeout,
		KeepAliveConfig: net.KeepAliveConfig{
			Enable:   true,
			Idle:     15 * time.Second,
			Interval: 15 * time.Second,
			Count:    2,
		},
	}
	conn, err := dialer.DialContext(s.ctx, "tcp", s.config.ServerAddress)
	if err != nil {
		return fmt.Errorf("DialContext: %w", err)
	}
	// https://github.com/golang/go/issues/19338#issuecomment-539057790
	if s.config.DialTimeout > 0 {
		conn.SetDeadline(time.Now().Add(s.sshClientConfig.Timeout))
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, s.config.ServerAddress, s.sshClientConfig)
	if err != nil {
		return fmt.Errorf("NewClientConn: %w", err)
	}
	conn.SetDeadline(time.Time{})
	s.client = ssh.NewClient(c, chans, reqs)
	s.logger.Debug("SshWrapper.UpdateClient: success")
	return nil
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

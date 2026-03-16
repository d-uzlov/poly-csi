package logs

import (
	"log/slog"
	"sync"
)

type Instance struct {
	config    Config
	logger    *slog.Logger
	mut       sync.Mutex
	queue     []logRequest
	finalized bool
}

type requestType int

const (
	// skip zero to be able to detect uninitialized variables
	__zero = iota
	requestSuccessMicroOp
	requestCommand
	requestInfo
	requestWarn
	requestError
)

type logRequest struct {
	action requestType
	msg    string
	args   []any
}

func (c *Instance) Commit(success bool) {
	// if SkipOnSuccess is not set, all messages have already been written into underlying logger
	if !c.config.SkipOnSuccess {
		return
	}
	if c.config.SkipOnSuccess && success {
		return
	}

	c.mut.Lock()
	defer c.mut.Unlock()

	c.finalized = true

	for _, v := range c.queue {
		c.logByAction(v.action, v.msg, v.args...)
	}
}

func (c *Instance) logByAction(action requestType, msg string, args ...any) {
	if !c.finalized && c.config.SkipOnSuccess {
		c.mut.Lock()
		defer c.mut.Unlock()
		c.queue = append(c.queue, logRequest{
			action: action,
			msg:    msg,
			args:   args,
		})
		return
	}

	switch action {
	case requestSuccessMicroOp, requestCommand:
		c.logger.Debug(msg, args...)
	case requestInfo:
		c.logger.Info(msg, args...)
	case requestWarn:
		c.logger.Warn(msg, args...)
	case requestError:
		c.logger.Error(msg, args...)
	default:
		c.logger.Error(msg, args...)
	}
}

func (c *Instance) MicroOpSuccess(msg string, args ...any) {
	if !c.config.ConfirmMicroOps {
		return
	}
	c.logByAction(requestSuccessMicroOp, msg, args...)
}

func (c *Instance) Command(msg string, args ...any) {
	if !c.config.EchoCommands {
		return
	}
	c.logByAction(requestCommand, msg, args...)
}

func (c *Instance) Info(msg string, args ...any) {
	c.logByAction(requestInfo, msg, args...)
}

func (c *Instance) Warn(msg string, args ...any) {
	c.logByAction(requestWarn, msg, args...)
}

func (c *Instance) Error(msg string, args ...any) {
	c.logByAction(requestError, msg, args...)
}

func DefaultInstance() *Instance {
	return &Instance{
		config: Config{
			LogLevel: "info",
		},
		logger: slog.Default(),
	}
}

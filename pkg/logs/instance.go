package logs

import (
	"log/slog"
	"slices"
	"sync"
)

type Instance struct {
	config   Config
	logger   *slog.Logger
	keepCopy bool

	// mut protects all fields below
	// fields above mut must be immutable
	mut          sync.Mutex
	requestQueue []LogRequest
	copyQueue    []LogRequest
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

type LogRequest struct {
	Action    requestType `json:"action"`
	Msg       string      `json:"msg"`
	Args      []any       `json:"args"`
	ephemeral bool        `json:"-"`
}

func (c *Instance) Commit(success bool) {
	c.mut.Lock()
	defer c.mut.Unlock()

	if !(c.config.SkipOnSuccess && success) {
		for _, v := range c.requestQueue {
			c.executeRequest(v)
		}
	}

	c.requestQueue = nil
}

func (c *Instance) GetCopy() []LogRequest {
	c.mut.Lock()
	defer c.mut.Unlock()
	return slices.Clone(c.copyQueue)
}

func (c *Instance) ConsumeCopy(copy []LogRequest) {
	for _, v := range copy {
		// TODO add fields to indicate that this is copy?
		c.request(v)
	}
}

func (c *Instance) request(req LogRequest) {
	if !c.config.SkipOnSuccess {
		c.executeRequest(req)
	}

	if !c.config.SkipOnSuccess && !c.keepCopy {
		return
	}

	c.mut.Lock()
	defer c.mut.Unlock()

	if c.config.SkipOnSuccess {
		c.requestQueue = append(c.requestQueue, req)
	}

	if c.keepCopy && !req.ephemeral {
		c.copyQueue = append(c.copyQueue, req)
	}
}

func (c *Instance) executeRequest(req LogRequest) {
	switch req.Action {
	case requestSuccessMicroOp:
		if c.config.EchoCommands {
			c.logger.Debug(req.Msg, req.Args...)
		}
	case requestCommand:
		if c.config.ConfirmMicroOps {
			c.logger.Debug(req.Msg, req.Args...)
		}
	case requestInfo:
		c.logger.Info(req.Msg, req.Args...)
	case requestWarn:
		c.logger.Warn(req.Msg, req.Args...)
	case requestError:
		c.logger.Error(req.Msg, req.Args...)
	default:
		c.logger.Error(req.Msg, req.Args...)
	}
}

func (c *Instance) MicroOp(msg string, args ...any) {
	c.request(LogRequest{
		Action: requestSuccessMicroOp,
		Msg:    msg,
		Args:   args,
	})
}

func (c *Instance) Command(msg string, args ...any) {
	c.request(LogRequest{
		Action: requestCommand,
		Msg:    msg,
		Args:   args,
	})
}

func (c *Instance) Info(msg string, args ...any) {
	c.request(LogRequest{
		Action: requestInfo,
		Msg:    msg,
		Args:   args,
	})
}

// EphemeralInfo creates an Info log entry which will not be present in GetCopy result
func (c *Instance) EphemeralInfo(msg string, args ...any) {
	c.request(LogRequest{
		Action:    requestInfo,
		Msg:       msg,
		Args:      args,
		ephemeral: true,
	})
}

func (c *Instance) Warn(msg string, args ...any) {
	c.request(LogRequest{
		Action: requestWarn,
		Msg:    msg,
		Args:   args,
	})
}

func (c *Instance) Error(msg string, args ...any) {
	c.request(LogRequest{
		Action: requestError,
		Msg:    msg,
		Args:   args,
	})
}

// EphemeralError creates an Error log entry which will not be present in GetCopy result
func (c *Instance) EphemeralError(msg string, args ...any) {
	c.request(LogRequest{
		Action:    requestError,
		Msg:       msg,
		Args:      args,
		ephemeral: true,
	})
}

func DefaultInstance() *Instance {
	return &Instance{
		config: Config{
			LogLevel: "info",
		},
		logger: slog.Default(),
	}
}

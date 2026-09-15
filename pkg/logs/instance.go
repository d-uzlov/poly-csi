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
	requestArgs
	requestResult
)

// LogRequest describes an intent to write a log line.
// It can be modified or cancelled (deleted) to alter actual log.
type LogRequest struct {
	Action    requestType `json:"action"`
	Msg       string      `json:"msg"`
	Args      []any       `json:"args"`
	ephemeral bool        `json:"-"`
}

// Commit processes all pending log requests, writing of skipping them, according to state and options.
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

// GetCopy returns an independent copy of log requests.
// Primary reason to use it is to consume the result in ConsumeCopy().
func (c *Instance) GetCopy() []LogRequest {
	c.mut.Lock()
	defer c.mut.Unlock()
	return slices.Clone(c.copyQueue)
}

// ConsumeCopy adds input log requests to internal queue.
func (c *Instance) ConsumeCopy(copy []LogRequest, from string) {
	if from != "" {
		for _, v := range copy {
			v.Args = append(v.Args, "from", from)
		}
	}
	for _, v := range copy {
		c.processRequest(v)
	}
}

// processRequest processes single input processRequest,
// possibly writing it and/or adding to internal queue.
func (c *Instance) processRequest(req LogRequest) {
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

// executeRequest writes given input request to underlying logger.
func (c *Instance) executeRequest(req LogRequest) {
	switch req.Action {
	case requestArgs:
		if c.config.EchoArgs {
			c.logger.Debug(req.Msg, req.Args...)
		}
	case requestCommand:
		if c.config.EchoCommands {
			c.logger.Debug(req.Msg, req.Args...)
		}
	case requestResult:
		if c.config.EchoResult {
			c.logger.Debug(req.Msg, req.Args...)
		}
	case requestSuccessMicroOp:
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

// Args creates log entry with 'Args' tag
func (c *Instance) Args(msg string, args ...any) {
	c.processRequest(LogRequest{
		Action: requestArgs,
		Msg:    msg,
		Args:   args,
	})
}

// MicroOp creates log entry with 'MicroOp' tag
func (c *Instance) MicroOp(msg string, args ...any) {
	c.processRequest(LogRequest{
		Action: requestSuccessMicroOp,
		Msg:    msg,
		Args:   args,
	})
}

// Command creates log entry with 'Command' tag
func (c *Instance) Command(msg string, args ...any) {
	c.processRequest(LogRequest{
		Action: requestCommand,
		Msg:    msg,
		Args:   args,
	})
}

// Result creates log entry with 'Result' tag
func (c *Instance) Result(msg string, args ...any) {
	c.processRequest(LogRequest{
		Action: requestResult,
		Msg:    msg,
		Args:   args,
	})
}

// EphemeralInfo creates a generic Info log entry
func (c *Instance) Info(msg string, args ...any) {
	c.processRequest(LogRequest{
		Action: requestInfo,
		Msg:    msg,
		Args:   args,
	})
}

// EphemeralInfo creates a generic Info log entry which will not be present in GetCopy result
func (c *Instance) EphemeralInfo(msg string, args ...any) {
	c.processRequest(LogRequest{
		Action:    requestInfo,
		Msg:       msg,
		Args:      args,
		ephemeral: true,
	})
}

// EphemeralInfo creates a generic Warn log entry
func (c *Instance) Warn(msg string, args ...any) {
	c.processRequest(LogRequest{
		Action: requestWarn,
		Msg:    msg,
		Args:   args,
	})
}

// EphemeralInfo creates a generic Error log entry
func (c *Instance) Error(msg string, args ...any) {
	c.processRequest(LogRequest{
		Action: requestError,
		Msg:    msg,
		Args:   args,
	})
}

// EphemeralError creates a generic Error log entry which will not be present in GetCopy result
func (c *Instance) EphemeralError(msg string, args ...any) {
	c.processRequest(LogRequest{
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

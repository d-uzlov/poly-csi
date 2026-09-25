package transport

import "time"

type ExecParams struct {
	// Sudo requests transport to run command as root
	Sudo bool
	// StdinBytes sets command stdin from content.
	StdinBytes []byte
	// StdinFile uses file on the target machine as stdin for the command.
	StdinFile string
	// StdoutFile redirects command stdout into a file on the target machine.
	StdoutFile string
	// DisableStdout can be used as an optimization in cases when command output is not needed.
	// Stderr is not affected by this.
	DisableStdout bool
	// Timeout sets a timeout on full command execution.
	// When set to 0, exec transport should use its default timeout.
	// Using timeout should be equivalent to using context.WithTimeout before calling RunCommand
	Timeout time.Duration

	Env map[string]string
}

type ExecOption func(o *ExecParams)

// see ExecParams.Sudo
func WithSudo(value bool) ExecOption {
	return func(o *ExecParams) {
		o.Sudo = value
	}
}

// WithStdinString is equivalent to WithStdinBytes([]byte(value))
func WithStdinString(value string) ExecOption {
	return func(o *ExecParams) {
		o.StdinBytes = []byte(value)
	}
}

// see ExecParams.StdinBytes
func WithStdinBytes(value []byte) ExecOption {
	return func(o *ExecParams) {
		o.StdinBytes = value
	}
}

// see ExecParams.StdinFile
func WithStdinFile(value string) ExecOption {
	return func(o *ExecParams) {
		o.StdinFile = value
	}
}

// see ExecParams.StdoutFile
func WithStdoutFile(value string) ExecOption {
	return func(o *ExecParams) {
		o.StdoutFile = value
	}
}

// see ExecParams.DisableStdout
func WithoutStdout() ExecOption {
	return func(o *ExecParams) {
		o.DisableStdout = true
	}
}

// see ExecParams.Timeout
func WithTimeout(value time.Duration) ExecOption {
	return func(o *ExecParams) {
		o.Timeout = value
	}
}

// see ExecParams.Env
func WithEnv(key string, value string) ExecOption {
	return func(o *ExecParams) {
		if o.Env == nil {
			o.Env = map[string]string{}
		}
		o.Env[key] = value
	}
}

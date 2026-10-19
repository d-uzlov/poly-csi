package transport

import (
	"context"
	"time"
)

type Transport interface {
	// Cleanup stops background processes and releases external resources
	// After cleanup this transport may be inoperable
	Cleanup()
	// SetCommands sets command overrides
	// It's not thread-safe.
	SetCommands(commands *ExecReplacement)
	// RunCommand runs specified command to completion
	// Safe to run in parallel.
	RunCommand(ctx context.Context, cmd []string, opts ...ExecOption) (RunResult, error)
	// Cmds creates Commands object with optimized common commands
	// Safe to run in parallel.
	Cmds() Commands
	SudoCmds() Commands
}

type RunResult struct {
	// Stdout contains raw command stdout.
	// Can be empty if stdout was disabled.
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	// Cancelled is set if command was cancelled for any reason.
	Cancelled bool
	// Timeout is set if command exceeded timeout.
	Timeout bool
}

type Commands interface {
	Read(ctx context.Context, path string, trim bool) ([]byte, error)
	Write(ctx context.Context, path string, data []byte) error
	// List file names in a directory
	Ls(ctx context.Context, path string) ([]string, error)
	// Create directory. Idempotent, returns nil is directory already exists
	Mkdir(ctx context.Context, path string, parents bool) error
	Move(ctx context.Context, source string, dest string) error
	RmDirRecursive(ctx context.Context, path string) error
	// Delete directory. Can fail on files
	RmDir(ctx context.Context, path string) error
	// Delete file. Can fail on directories
	RmFile(ctx context.Context, path string) error
	// ReadLink reads a path in filesystem, following symlinks.
	// Returns same filename if last part does not exist.
	// Returns empty value if parent paths don't exist.
	ReadLink(ctx context.Context, path string) (string, error)
	// Stat probes info about ReadLink(path)
	Stat(ctx context.Context, path string) (StatResult, error)
	Df(ctx context.Context, path string) (*FsSizeInfo, error)
	EnsureSymlink(ctx context.Context, link string, target string) error
	MkFifo(ctx context.Context, path string) (alreadyExists bool, err error)
	Chown(ctx context.Context, path string, user string, group string, recursive bool) error
	Chmod(ctx context.Context, path string, mode string, recursive bool) error

	// Creates and/or capture a lock file in specified location.
	// When it returns, lock file will contain lease info, which must be released via a provided callback.
	// Lease must have a maximum lifetime specified. It serves as a safeguard against stuck or crashed programs.
	// Successive CreateLock requests on the same file will fail or hang, until release is called, or original lease lifetime expires.
	CreateLock(ctx context.Context, lockFile string, maxDuration time.Duration) (release func(ctx context.Context) error, err error)
}

const (
	// skip zero to be able to detect uninitialized variables
	__zero = iota
	FtDirectory
	FtBlock
	FtFile
	FtFifo
	// FtLink
)

type StatResult struct {
	Type int
	// Path is the full file path with all symlinks resolved.
	Path         string
	FileExists   bool
	ParentExists bool
	Uid          string
	Gid          string
}

type FsSizeInfo struct {
	Size      int64
	Used      int64
	Available int64
}

func (r StatResult) IsDirectory() bool { return r.Type == FtDirectory }
func (r StatResult) IsBlock() bool     { return r.Type == FtBlock }
func (r StatResult) IsFile() bool      { return r.Type == FtFile }
func (r StatResult) IsFifo() bool      { return r.Type == FtFifo }

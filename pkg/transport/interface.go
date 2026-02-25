package transport

import (
	"context"
	"time"
)

type Transport interface {
	Cleanup()
	RunCommand(ctx context.Context, cmd []string, opts ...ExecOption) (CommandResult, error)
	Cmds() Commands
	SudoCmds() Commands
}

type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
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
	// Stat returns StatResult for existing directory, or null if directory is not found
	Stat(ctx context.Context, path string) (StatResult, error)
	Df(ctx context.Context, path string) (*FsSizeInfo, error)
	CreateSymlink(ctx context.Context, link string, target string) error
	MkFifo(ctx context.Context, path string) (alreadyExists bool, err error)

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
	// FtLink
	FtBlock
	FtFile
)

type StatResult struct {
	Type int
	// Path is the full file path with all symlinks resolved.
	Path         string
	FileExists   bool
	ParentExists bool
}

type FsSizeInfo struct {
	Size      int64
	Used      int64
	Available int64
}

func (r StatResult) IsDirectory() bool { return r.Type == FtDirectory }

// func (r StatResult) IsLink() bool      { return r.Type == FtLink }
func (r StatResult) IsBlock() bool { return r.Type == FtBlock }
func (r StatResult) IsFile() bool  { return r.Type == FtFile }

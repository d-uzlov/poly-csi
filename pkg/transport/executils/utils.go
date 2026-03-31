package executils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

var _ transport.Commands = &ExecCommands{}

type ExecCommands struct {
	tr   transport.Transport
	sudo bool
}

func New(tr transport.Transport, sudo bool) *ExecCommands {
	return &ExecCommands{tr: tr, sudo: sudo}
}

func (ec *ExecCommands) Read(ctx context.Context, path string, trim bool) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	cmd := []string{"cat", path}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo))
	if err != nil {
		return nil, err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such file or directory") {
			return nil, nil
		}
		return nil, transport.FormatRunError(runResult)
	}
	result := runResult.Stdout
	if trim {
		result = []byte(strings.TrimSpace(string(result)))
	}
	return result, nil
}

func (ec *ExecCommands) Write(ctx context.Context, path string, data []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	cmd := []string{"tee", path}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo), transport.WithStdinBytes(data), transport.WithoutStdout())
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (ec *ExecCommands) WriteMove(ctx context.Context, path string, data []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	tmpFile := path + ".~tmp"

	err := ec.Write(ctx, tmpFile, data)
	if err != nil {
		return err
	}
	return ec.Move(ctx, tmpFile, path)
}

func (ec *ExecCommands) Ls(ctx context.Context, path string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	cmd := []string{"/bin/ls", path}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo))
	if err != nil {
		return nil, err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such file or directory") {
			return nil, nil
		}
		return nil, transport.FormatRunError(runResult)
	}
	return strings.Fields(string(runResult.Stdout)), nil
}

func (ec *ExecCommands) Move(ctx context.Context, source string, dest string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if source == dest {
		return nil
	}

	cmd := []string{"mv", source, dest}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo), transport.WithoutStdout())
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "are the same file") {
			return nil
		}
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (ec *ExecCommands) Mkdir(ctx context.Context, path string, parents bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	cmd := []string{"mkdir", path}
	if parents {
		cmd = append(cmd, "-p")
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo), transport.WithoutStdout())
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "File exists") {
			return nil
		}
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (ec *ExecCommands) RmDirRecursive(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	cmd := []string{"rm", "-r", path}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo), transport.WithoutStdout())
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such file or directory") {
			return nil
		}
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (ec *ExecCommands) RmDir(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	cmd := []string{"rmdir", path}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo), transport.WithoutStdout())
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such file or directory") {
			return nil
		}
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (ec *ExecCommands) RmFile(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	cmd := []string{"rm", path}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo), transport.WithoutStdout())
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such file or directory") {
			return nil
		}
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (ec *ExecCommands) ReadLink(ctx context.Context, path string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// -f means follow symlinks and print same filename for non-existent files
	// -n no newline
	// can't use proper --canonicalize and --no-newline because we need to support busybox version of readlink
	cmd := []string{"readlink", "-f", "-n", path}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo))
	if err != nil {
		return "", err
	}
	if runResult.ExitCode != 0 {
		if runResult.ExitCode == 1 && len(runResult.Stdout) == 0 && len(runResult.Stderr) == 0 {
			// it's arguable if we should return nil value or an error here
			// TODO change interface?
			return "", nil
		}
		return "", transport.FormatRunError(runResult)
	}
	if len(runResult.Stdout) == 0 {
		return "", fmt.Errorf("readlink gave empty result")
	}
	return string(runResult.Stdout), nil
}

func (ec *ExecCommands) Stat(ctx context.Context, path string) (transport.StatResult, error) {
	if ctx.Err() != nil {
		return transport.StatResult{}, ctx.Err()
	}

	realPath, err := ec.ReadLink(ctx, path)
	if err != nil {
		return transport.StatResult{}, fmt.Errorf("readlink: %w", err)
	}
	if realPath == "" {
		return transport.StatResult{
			Type:         0,
			Path:         "",
			FileExists:   false,
			ParentExists: false,
		}, nil
	}

	// -c FMT  Use the specified format
	// can't use proper --printf because we need to support busybox version of stat
	cmd := []string{"stat", realPath, "-c", "%F"}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo))
	if err != nil {
		return transport.StatResult{}, fmt.Errorf("stat: %w", err)
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such file or directory") {
			return transport.StatResult{
				Type:         0,
				Path:         realPath,
				FileExists:   false,
				ParentExists: true,
			}, nil
		}
		return transport.StatResult{}, transport.FormatRunError(runResult)
	}
	statStdout := strings.TrimSpace(string(runResult.Stdout))
	if statStdout == "" {
		return transport.StatResult{}, fmt.Errorf("stat gave empty result")
	}
	lines := strings.Split(statStdout, "\n")
	if len(lines) != 1 {
		return transport.StatResult{}, fmt.Errorf("stat gave unexpected result: '%s', expected 1 values but got %d values", string(runResult.Stdout), len(lines))
	}
	fileType := lines[0]

	result := transport.StatResult{
		Path:         realPath,
		FileExists:   true,
		ParentExists: true,
	}
	switch fileType {
	case "directory":
		result.Type = transport.FtDirectory
	// case "symbolic link":
	// 	result.Type = transport.FtLink
	// 	linkInfo := strings.SplitN(lines[1], " -> ", 2)
	// 	if len(linkInfo) != 2 {
	// 		return nil, fmt.Errorf("unexpected link format from stat: %s", lines[1])
	// 	}
	// 	linkInfo[0] = strings.Trim(linkInfo[0], "'")
	// 	linkInfo[1] = strings.Trim(linkInfo[1], "'")
	// 	result.Path = filepath.Clean(filepath.Dir(linkInfo[0]) + "/" + linkInfo[1])
	case "block special file":
		result.Type = transport.FtBlock
	case "regular file", "regular empty file":
		result.Type = transport.FtFile
	case "fifo":
		result.Type = transport.FtFifo
	}
	return result, nil
}

func (ec *ExecCommands) Df(ctx context.Context, path string) (*transport.FsSizeInfo, error) {
	cmd := []string{"df", "--output=size,used,avail", "--block-size=1", path}
	// cmd := fmt.Sprintf("df --output=avail --block-size=1 '%s' | tail -n +2", path)
	stdout, err := transport.CheckedRunStdout(ctx, ec.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}
	lines := strings.Split(stdout, "\n")
	if len(lines) != 2 {
		return nil, fmt.Errorf("expected only 2 lines in df stdout: %s", stdout)
	}

	fields := strings.Fields(lines[1])
	if len(fields) != 3 {
		return nil, fmt.Errorf("expected 3 fields in df stdout: %s", stdout)
	}

	size, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse size from df stdout")
	}
	used, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse used from df stdout")
	}
	avail, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse avail from df stdout")
	}

	return &transport.FsSizeInfo{
		Size:      size,
		Used:      used,
		Available: avail,
	}, nil
}

func (ec *ExecCommands) EnsureSymlink(ctx context.Context, link string, target string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if link == target {
		return nil
	}

	sr, err := ec.Stat(ctx, link)
	if err != nil {
		return fmt.Errorf("check existing link at '%s': %w", link, err)
	}
	if sr.FileExists {
		if sr.Path == link {
			return fmt.Errorf("path '%s' already contains non-link", link)
		}
		if sr.Path != target {
			return fmt.Errorf("existing link '%s' points to '%s'", link, sr.Path)
		}
		// link already points to required path
		return nil
	}
	// link does not exist, need to create it
	cmd := []string{"ln", "--symbolic", "--no-target-directory", target, link}
	return transport.CheckedRun(ctx, ec.tr, cmd, transport.WithSudo(ec.sudo))
}

func (ec *ExecCommands) MkFifo(ctx context.Context, path string) (alreadyExists bool, err error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	cmd := []string{"mkfifo", path}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithSudo(ec.sudo), transport.WithoutStdout())
	if err != nil {
		return false, err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "File exists") {
			sr, err := ec.Stat(ctx, path)
			if err != nil {
				return true, err
			}
			if !sr.IsFifo() {
				return true, fmt.Errorf("file already exists but it has different type: %v", sr.Type)
			}
			return true, nil
		}
		return false, transport.FormatRunError(runResult)
	}
	return false, nil
}

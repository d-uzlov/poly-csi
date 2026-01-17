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
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd)
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
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithStdinBytes(data), transport.WithoutStdout())
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (ec *ExecCommands) Ls(ctx context.Context, path string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	cmd := []string{"/bin/ls", path}
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd)
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

	cmd := []string{"mv", source, dest}
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithoutStdout())
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
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	if parents {
		cmd = append(cmd, "-p")
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithoutStdout())
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
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithoutStdout())
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
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithoutStdout())
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
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithoutStdout())
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

func (ec *ExecCommands) Stat(ctx context.Context, path string) (*transport.StatResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	cmd := []string{"readlink", "--canonicalize", "--no-newline", path}
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such file or directory") {
			return nil, nil
		}
		return nil, transport.FormatRunError(runResult)
	}
	if len(runResult.Stdout) == 0 {
		return nil, fmt.Errorf("readlink gave empty result")
	}

	realPath := string(runResult.Stdout)

	cmd = []string{"stat", realPath, "--printf=%F\n%N"}
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err = ec.tr.RunCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "No such file or directory") {
			return nil, nil
		}
		return nil, transport.FormatRunError(runResult)
	}
	if len(runResult.Stdout) == 0 {
		return nil, fmt.Errorf("stat gave empty result")
	}
	lines := strings.Split(string(runResult.Stdout), "\n")
	if len(lines) != 2 {
		return nil, fmt.Errorf("stat gave unexpected result: %s", string(runResult.Stdout))
	}
	fileType := lines[0]

	result := &transport.StatResult{
		Path: realPath,
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
	}
	return result, nil
}

func (ec *ExecCommands) Df(ctx context.Context, path string) (*transport.FsSizeInfo, error) {
	cmd := []string{"df", "--output=size,used,avail", "--block-size=1", path}
	// cmd := fmt.Sprintf("df --output=avail --block-size=1 '%s' | tail -n +2", path)
	stdout, err := transport.CheckedRunStdout(ctx, cmd)
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

func (ec *ExecCommands) CreateSymlink(ctx context.Context, link string, target string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	cmd := []string{"ln", "--symbolic", "--no-target-directory", target, link}
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithoutStdout())
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

func (ec *ExecCommands) MkFifo(ctx context.Context, path string) (alreadyExists bool, err error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	cmd := []string{"mkfifo", path}
	if ec.sudo {
		cmd = append([]string{"sudo"}, cmd...)
	}
	runResult, err := ec.tr.RunCommand(ctx, cmd, transport.WithoutStdout())
	if err != nil {
		return false, err
	}
	if runResult.ExitCode != 0 {
		if strings.Contains(string(runResult.Stderr), "File exists") {
			return true, nil
		}
		return false, transport.FormatRunError(runResult)
	}
	return false, nil
}

package dispatcher

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/transport"
)

type sendFuncType func(ctx context.Context, targetFile string, timeout time.Duration) error
type receiveFuncType func(ctx context.Context, sourceFile string, timeout time.Duration) (storage.Volume, error)

func createClonePipes(ctx context.Context, id string, source *parsedServer, target *parsedServer, timeout time.Duration) (send string, receive string, release func(), err error) {
	if source.conf.StableId == target.conf.StableId {
		fifo := fmt.Sprintf("/tmp/clone-local-%s", id)
		_, err = source.trInstance.Cmds().MkFifo(ctx, fifo)
		if err != nil {
			return "", "", nil, fmt.Errorf("create pipe '%s': %w", fifo, err)
		}
		return fifo, fifo, func() { source.trInstance.Cmds().RmFile(ctx, fifo) }, nil
	}

	send = fmt.Sprintf("/tmp/clone-send-%s", id)
	receive = fmt.Sprintf("/tmp/clone-receive-%s", id)
	success := false
	deletePipes := func() {
		_ = source.trInstance.Cmds().RmFile(ctx, send)
		_ = target.trInstance.Cmds().RmFile(ctx, receive)
	}
	defer func() {
		if success {
			return
		}
		deletePipes()
	}()

	_, err = source.trInstance.Cmds().MkFifo(ctx, send)
	if err != nil {
		return "", "", nil, fmt.Errorf("create send pipe '%s': %w", send, err)
	}
	_, err = target.trInstance.Cmds().MkFifo(ctx, receive)
	if err != nil {
		return "", "", nil, fmt.Errorf("create receive pipe '%s': %w", receive, err)
	}

	// TODO better address selection
	targetAddress := target.conf.AddressList[0].Address
	// select random port between 40k and 50k
	port := rand.Intn(10000) + 40000
	portString := fmt.Sprintf("%d", port)

	sendCtx, sendCancel := context.WithCancel(ctx)
	sendCmd := []string{"ncat", "--send-only", targetAddress, portString}

	receiveCtx, receiveCancel := context.WithCancel(ctx)
	receiveCmd := []string{"ncat", "-l", portString}

	// we need to be careful to avoid leaked goroutines if an error happens and release was never called
	go func() {
		runResult, err := source.trInstance.RunCommand(sendCtx, sendCmd, transport.WithStdinFile(send), transport.WithoutStdout(), transport.WithTimeout(timeout))
		if err != nil {
			slogctx.FromCtx(ctx).Error("ncat send failed", "error", err)
		} else if runResult.ExitCode != 0 {
			slogctx.FromCtx(ctx).Error("ncat send failed", "error", transport.FormatRunError(runResult))
		}
	}()
	go func() {
		runResult, err := target.trInstance.RunCommand(receiveCtx, receiveCmd, transport.WithStdoutFile(receive), transport.WithTimeout(timeout))
		if err != nil {
			slogctx.FromCtx(ctx).Error("ncat receive failed", "error", err)
		} else if runResult.ExitCode != 0 {
			slogctx.FromCtx(ctx).Error("ncat receive failed", "error", transport.FormatRunError(runResult))
		}
	}()

	success = true
	return send, receive, func() {
		sendCancel()
		receiveCancel()
		deletePipes()
	}, nil
}

func sendTar(ctx context.Context, sourceDir string, targetFile string, timeout time.Duration) error {
	tr := transport.FromContext(ctx)

	// tar will `cd` into sourceDir and create archive from . (current folder)
	cmd := []string{"sudo", "tar", "--create", "--file=-", "--directory", sourceDir, "."}
	runResult, err := tr.RunCommand(ctx, cmd, transport.WithStdoutFile(targetFile), transport.WithTimeout(timeout))
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

func receiveTar(ctx context.Context, sourceFile string, targetDir string, timeout time.Duration) error {
	tr := transport.FromContext(ctx)

	cmd := []string{"sudo", "tar", "--extract", "--file=-", "--directory", targetDir}
	runResult, err := tr.RunCommand(ctx, cmd, transport.WithStdinFile(sourceFile), transport.WithoutStdout(), transport.WithTimeout(timeout))
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

func sendFile(ctx context.Context, sourceFile string, targetFile string, timeout time.Duration) error {
	tr := transport.FromContext(ctx)

	cmd := []string{"sudo", "dd", "bs=1M", fmt.Sprintf("if=%s", sourceFile)}
	runResult, err := tr.RunCommand(ctx, cmd, transport.WithStdoutFile(targetFile), transport.WithTimeout(timeout))
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

func receiveFile(ctx context.Context, sourceFile string, targetFile string, timeout time.Duration) error {
	tr := transport.FromContext(ctx)

	cmd := []string{"sudo", "dd", "bs=1M", fmt.Sprintf("of=%s", targetFile)}
	runResult, err := tr.RunCommand(ctx, cmd, transport.WithStdinFile(sourceFile), transport.WithoutStdout(), transport.WithTimeout(timeout))
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

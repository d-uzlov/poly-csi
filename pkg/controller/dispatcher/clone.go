package dispatcher

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/storage/storagefactory"
	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
	"meoe.io/poly-csi/pkg/transport"
)

type sendFuncType func(ctx context.Context, targetFile string, timeout time.Duration) error
type receiveFuncType func(ctx context.Context, sourceFile string, timeout time.Duration) (storage.Volume, error)

func makeSendReceive(sourceCtx context.Context, targetCtx context.Context, sourceSrv *parsedServer, sourceVi voldbwrapper.VolumeInfo, targetStr storage.StorageManager, req storage.CreateRequest) (sendFuncType, receiveFuncType, error) {
	sourceStr, err := storagefactory.StorageFromType(sourceVi.StorageType, sourceSrv.conf.Commands, sourceSrv.conf.GlobalParams)
	if err != nil {
		return nil, nil, fmt.Errorf("get source storage: %w", err)
	}

	sourceCaps := sourceStr.GetCapabilities()
	targetCaps := targetStr.GetCapabilities()

	sourceVol, err := sourceStr.ParseVolume(sourceVi.StorageData)
	if err != nil {
		return nil, nil, fmt.Errorf("parse source storage data: %w", err)
	}

	// storage-specific send-receive
	if sourceCaps.NativeSnapshot &&
		sourceCaps.LocalSend != "" && targetCaps.LocalSend != "" &&
		sourceCaps.LocalSend == targetCaps.LocalSend {
		sendFunc := func(ctx context.Context, targetFile string, timeout time.Duration) error {
			snap, err := sourceVol.MakeNativeSnapshot(ctx, req.Id)
			if err != nil {
				return fmt.Errorf("create snapshot for send: %w", err)
			}
			// TODO cleanup should use a different context, with extended timeout
			defer snap.Delete(ctx)
			return snap.Send(ctx, targetFile, timeout)
		}
		receiveFunc := func(ctx context.Context, sourceFile string, timeout time.Duration) (storage.Volume, error) {
			return targetStr.Receive(ctx, sourceFile, req, timeout)
		}
		return sendFunc, receiveFunc, nil
	}

	// if storage does not support specialized send-receive,
	// or if source and target use different methods,
	// we can fall back to generic file-based transfer

	if sourceCaps.LocalVolume == storage.VolFolder {
		sourceLocalPath, err := sourceVol.GetLocalPath(sourceCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("get source local path: %w", err)
		}
		if sourceLocalPath == "" {
			return nil, nil, fmt.Errorf("generic copy is not available without local path")
		}

		targetVol, err := targetStr.CreateVolume(targetCtx, req)
		if err != nil {
			return nil, nil, fmt.Errorf("storage.CreateVolume: %w", err)
		}
		slogctx.FromCtx(sourceCtx).Info("storage.CreateVolume: success")

		targetLocalPath, err := targetVol.GetLocalPath(targetCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("get target local path: %w", err)
		}
		if targetLocalPath == "" {
			return nil, nil, fmt.Errorf("generic copy is not available without local path")
		}

		sendFunc := func(ctx context.Context, targetFile string, timeout time.Duration) error {
			err := sendTar(ctx, sourceLocalPath, targetFile, timeout)
			if err != nil {
				return fmt.Errorf("send tar: %w", err)
			}
			return nil
		}
		receiveFunc := func(ctx context.Context, sourceFile string, timeout time.Duration) (storage.Volume, error) {
			err := receiveTar(ctx, sourceFile, targetLocalPath, timeout)
			if err != nil {
				return nil, fmt.Errorf("receive tar: %w", err)
			}
			return targetVol, nil
		}
		return sendFunc, receiveFunc, nil
	}

	return nil, nil, fmt.Errorf("generic copy is not available for volumes of type '%s'", sourceCaps.LocalVolume)
}

func createClonePipes(ctx context.Context, id string, source *parsedServer, target *parsedServer, timeout time.Duration) (send string, receive string, release func(), err error) {
	if source.conf.StableId == target.conf.StableId {
		fifo := fmt.Sprintf("/tmp/clone-local-%s", id)
		err = source.trInstance.Cmds().MkFifo(ctx, fifo)
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

	err = source.trInstance.Cmds().MkFifo(ctx, send)
	if err != nil {
		return "", "", nil, fmt.Errorf("create send pipe '%s': %w", send, err)
	}
	err = target.trInstance.Cmds().MkFifo(ctx, receive)
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

func sendTar(ctx context.Context, sourceFile string, targetFile string, timeout time.Duration) error {
	tr := transport.FromContext(ctx)

	cmd := []string{"sudo", "tar", "--create", "-C", sourceFile, "."}
	runResult, err := tr.RunCommand(ctx, cmd, transport.WithStdoutFile(targetFile), transport.WithTimeout(timeout))
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

func receiveTar(ctx context.Context, sourceFile string, targetFile string, timeout time.Duration) error {
	tr := transport.FromContext(ctx)

	cmd := []string{"sudo", "tar", "--extract", "--file", "-", "-C", targetFile}
	runResult, err := tr.RunCommand(ctx, cmd, transport.WithStdinFile(sourceFile), transport.WithoutStdout(), transport.WithTimeout(timeout))
	if err != nil {
		return err
	}
	if runResult.ExitCode != 0 {
		return transport.FormatRunError(runResult)
	}
	return nil
}

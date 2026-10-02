package cntrublk

// import (
// 	"context"
// 	"slices"
// 	"sync"

// 	"meoe.io/poly-csi/pkg/logs"
// 	"meoe.io/poly-csi/pkg/transport"
// 	"meoe.io/poly-csi/pkg/utils/ublk"
// )

// type Watcher struct {
// 	tr             transport.Transport
// 	mut            sync.Mutex
// 	watchedDevices []string
// }

// func NewWatcher(tr transport.Transport) *Watcher {
// 	return &Watcher{
// 		tr: tr,
// 	}
// }

// func (w *Watcher) InitialRestore(ctx context.Context) error {
// 	w.mut.Lock()
// 	defer w.mut.Unlock()

// 	logger := logs.FromCtx(ctx)
// 	snip := ublk.New(w.tr)
// 	list, err := snip.List(ctx, "")
// 	if err != nil {
// 		return err
// 	}
// 	for _, v := range list {
// 		if v.State != "QUIESCED" {
// 			continue
// 		}
// 		if v.Target == "" {
// 			continue
// 		}
// 		info, err := snip.Recover(ctx, v.DevId)
// 		if err != nil {
// 			logger.Error("failed to recover ublk device", "devId", v.DevId, "error", err)
// 			continue
// 		}
// 		logger.Info("successfully recovered ublk device", "devId", v.DevId, "state", info.State)
// 		w.watchedDevices = append(w.watchedDevices, info.DevId)
// 	}
// 	return nil
// }

// func (w *Watcher) Add(devId string, pid string) {
// 	w.mut.Lock()
// 	defer w.mut.Unlock()

// 	w.watchedDevices = append(w.watchedDevices, devId)
// }

// func (w *Watcher) Remove(devId string) {
// 	w.mut.Lock()
// 	defer w.mut.Unlock()

// 	w.watchedDevices = slices.DeleteFunc(w.watchedDevices, func(v string) bool { return v == devId })
// }

// func (w *Watcher) Reconcile(ctx context.Context) error {
// 	logger := logs.FromCtx(ctx)
// 	snip := ublk.New(w.tr)
// 	list, err := snip.List(ctx, "")
// 	if err != nil {
// 		return err
// 	}
// 	for _, v := range list {
// 		if v.State != "QUIESCED" {
// 			continue
// 		}
// 		if !slices.Contains(w.watchedDevices, v.DevId) {
// 			continue
// 		}
// 		if v.Target == "" {
// 			logger.Error("device in the watched list does not have target info", "devId", v.DevId)
// 			continue
// 		}
// 		info, err := snip.Recover(ctx, v.DevId)
// 		if err != nil {
// 			logger.Error("failed to recover ublk device", "devId", v.DevId, "error", err)
// 			continue
// 		}
// 		logger.Info("successfully recovered ublk device", "devId", v.DevId, "state", info.State)
// 	}
// 	return nil
// }

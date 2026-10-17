package dispatcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"
)

type Dispatcher struct {
	csi.UnimplementedControllerServer

	// confMutex locks conf as a whole but does not lock its contents.
	// When managing the content, you should lock confMutex, find what you need,
	// and then lock mutex on the object you have found.
	confMutex sync.RWMutex
	conf      *DispatcherConf
}

func NewDispatcher(ctx context.Context) *Dispatcher {
	result := &Dispatcher{
		conf: &DispatcherConf{},
	}
	return result
}

func (d *Dispatcher) Cleanup() {
	var conf *DispatcherConf
	func() {
		d.confMutex.Lock()
		defer d.confMutex.Unlock()

		conf = d.conf
		d.conf = nil
	}()
	conf.mutex.Lock()
	defer conf.mutex.Unlock()

	conf.Cleanup()
}

func (d *Dispatcher) ReloadConfig(ctx context.Context, newConf *DispatcherConf) {
	logger := slogctx.FromCtx(ctx)

	logger.Info("starting to reload config", "new-config", newConf)

	for _, v := range newConf.Servers {
		logPrefix := fmt.Sprintf("server[name=%s].", v.Conf.Name)
		err := v.InitTransport(ctx, logPrefix)
		if err != nil {
			logger.Error("failed to init server", "server-name", v.Conf.Name, "server-id", v.Conf.StableId, "error", fmt.Errorf("init exec transport: %w", err))
			return
		}

		err = v.InitDatabase(ctx, logPrefix)
		if err != nil {
			logger.Error("failed to init server", "server-name", v.Conf.Name, "server-id", v.Conf.StableId, "error", fmt.Errorf("init database: %w", err))
			return
		}
	}

	var oldConf *DispatcherConf
	func() {
		d.confMutex.Lock()
		defer d.confMutex.Unlock()

		oldConf = d.conf
		d.conf = newConf
	}()

	oldConf.mutex.Lock()
	defer oldConf.mutex.Unlock()

	oldConf.Cleanup()
}

func (d *Dispatcher) GetConfig() *DispatcherConf {
	d.confMutex.RLock()
	defer d.confMutex.RUnlock()

	return d.conf
}

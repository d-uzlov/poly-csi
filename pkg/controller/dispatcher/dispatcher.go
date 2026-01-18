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

func (d *Dispatcher) Cleanup(ctx context.Context) {
	var servers []*parsedServer
	func() {
		d.confMutex.Lock()
		defer d.confMutex.Unlock()

		servers = d.conf.servers
		d.conf.servers = nil
	}()

	for _, v := range servers {
		func() {
			v.mutex.Lock()
			defer v.mutex.Unlock()

			if v.trInstance != nil {
				v.trInstance.Cleanup(ctx)
			}
			v.trInstance = nil

			if v.dbInstance != nil {
				v.dbInstance.Cleanup(ctx)
			}
			v.dbInstance = nil
		}()
	}
}

func (d *Dispatcher) ReloadConfig(ctx context.Context, newConf *DispatcherConf) {
	logger := slogctx.FromCtx(ctx)

	logger.Info("starting to reload config", "new-config", newConf)

	var oldConf *DispatcherConf

	func() {
		d.confMutex.Lock()
		defer d.confMutex.Unlock()

		oldConf = d.conf
		d.conf = newConf
	}()

	d.confMutex.RLock()
	defer d.confMutex.RUnlock()

	for _, v := range d.conf.servers {
		func() {
			v.mutex.Lock()
			defer v.mutex.Unlock()

			oldSrv := oldConf.FindServerById(v.conf.StableId)
			if oldSrv != nil {
				oldSrv.mutex.Lock()
				defer oldSrv.mutex.Unlock()

				v.Reuse(oldSrv)
			}

			logPrefix := fmt.Sprintf("server[name=%s].", v.conf.Name)

			err := v.InitTransport(ctx, logPrefix)
			if err != nil {
				logger.Error("failed to init server", "server-name", v.conf.Name, "server-id", v.conf.StableId, "error", fmt.Errorf("init exec transport: %w", err))
				return
			}

			err = v.InitDatabase(ctx, logPrefix)
			if err != nil {
				logger.Error("failed to init server", "server-name", v.conf.Name, "server-id", v.conf.StableId, "error", fmt.Errorf("init database: %w", err))
				return
			}

			logger.Info("successfully initialized server", "server-name", v.conf.Name, "server-id", v.conf.StableId)
		}()
	}

	for _, v := range oldConf.servers {
		func() {
			v.mutex.Lock()
			defer v.mutex.Unlock()

			v.Cleanup(ctx)
		}()
	}
}

func (d *Dispatcher) FindServerByName(name string) (result *parsedServer, err error) {
	d.confMutex.RLock()
	defer d.confMutex.RUnlock()

	result = d.conf.FindServerByName(name)
	if result == nil {
		return nil, fmt.Errorf("server[name=%s] is not found", name)
	}

	return result, nil
}

func (d *Dispatcher) FindServerById(id string) (result *parsedServer, err error) {
	d.confMutex.RLock()
	defer d.confMutex.RUnlock()

	result = d.conf.FindServerById(id)
	if result == nil {
		return nil, fmt.Errorf("server[id=%s] is not found", id)
	}

	return result, nil
}

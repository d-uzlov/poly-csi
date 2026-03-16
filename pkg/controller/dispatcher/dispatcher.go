package dispatcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/logs"
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
	var servers []*parsedServer
	func() {
		d.confMutex.Lock()
		defer d.confMutex.Unlock()

		servers = d.conf.Servers
		d.conf.Servers = nil
	}()

	for _, v := range servers {
		func() {
			v.mutex.Lock()
			defer v.mutex.Unlock()

			if v.trInstance != nil {
				v.trInstance.Cleanup()
			}
			v.trInstance = nil

			if v.dbInstance != nil {
				v.dbInstance.Cleanup()
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

	for _, v := range d.conf.Servers {
		func() {
			v.mutex.Lock()
			defer v.mutex.Unlock()

			oldSrv := oldConf.FindServerById(v.Conf.StableId)
			if oldSrv != nil {
				oldSrv.mutex.Lock()
				defer oldSrv.mutex.Unlock()

				v.Reuse(oldSrv)
			}

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

			logger.Info("successfully initialized server", "server-name", v.Conf.Name, "server-id", v.Conf.StableId)
		}()
	}

	for _, v := range oldConf.Servers {
		func() {
			v.mutex.Lock()
			defer v.mutex.Unlock()

			v.Cleanup()
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

func (d *Dispatcher) GetRpcLogConfig(rpcPath string) logs.Config {
	d.confMutex.RLock()
	defer d.confMutex.RUnlock()

	result := logs.Config{
		LogLevel:     "info",
		EchoCommands: false,
		ExtraFields: map[string]string{
			"method": rpcPath,
		},
	}
	const defaultLogConfigKey = "default"
	config, ok := d.conf.Logs.Rpc[defaultLogConfigKey]
	if ok {
		result = result.Combine(config)
	}
	config, ok = d.conf.Logs.Rpc[rpcPath]
	if ok {
		result = result.Combine(config)
	}
	return result
}

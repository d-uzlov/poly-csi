package dispatcher

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sync"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/config"
	"meoe.io/poly-csi/pkg/controller/voldb"
	"meoe.io/poly-csi/pkg/controller/voldb/voldbfactory"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/transportfactory"
)

type DispatcherConf struct {
	mutex      sync.RWMutex
	alive      bool
	NodeInfoDb NodeInfoStorageConfig
	Servers    []*parsedServer
	Logs       Logs
}

func (c *DispatcherConf) Lock() (release func()) {
	c.mutex.Lock()
	return c.mutex.Unlock
}

func (c *DispatcherConf) IsAlive() bool {
	return c.alive
}

func (c *DispatcherConf) Validate() (errorList []string, warningList []string) {
	if c.NodeInfoDb.Address == "" {
		errorList = append(errorList, "nodeInfo.address: field is empty")
	}
	for i, v := range c.Servers {
		er, wl := v.Validate()
		for _, iss := range er {
			errorList = append(errorList, fmt.Sprintf("servers[i=%d/name=%s].%s", i, v.Conf.Name, iss))
		}
		for _, iss := range wl {
			warningList = append(warningList, fmt.Sprintf("servers[i=%d/name=%s].%s", i, v.Conf.Name, iss))
		}
	}
	return
}

func (c *DispatcherConf) FindServerByName(name string) *parsedServer {
	for _, v := range c.Servers {
		if v.Conf.Name == name {
			return v
		}
	}
	return nil
}

func (c *DispatcherConf) FindServerById(id string) *parsedServer {
	for _, v := range c.Servers {
		if v.Conf.StableId == id {
			return v
		}
	}
	return nil
}

func (c *DispatcherConf) GetRpcLogConfig(rpcPath string) logs.Config {
	result := logs.Config{
		LogLevel:     "info",
		EchoCommands: false,
		ExtraFields: map[string]string{
			"method": rpcPath,
		},
	}
	const defaultLogConfigKey = "default"
	config, ok := c.Logs.Rpc[defaultLogConfigKey]
	if ok {
		result = result.Combine(config)
	}
	config, ok = c.Logs.Rpc[rpcPath]
	if ok {
		result = result.Combine(config)
	}
	return result
}

func (c *DispatcherConf) Cleanup() {
	for _, v := range c.Servers {
		v.Cleanup()
	}
}

type parsedServer struct {
	valid bool
	Conf  *ServerConfig

	ExecReplacements transport.ExecReplacement

	trInstance transport.Transport
	trConfig   config.ValidatableConfig

	dbInstance     voldb.VolumeDatabase
	metadataConfig config.ValidatableConfig
}

func (c *parsedServer) Validate() (errorList []string, warningList []string) {
	el, wl := c.Conf.Validate()
	errorList = slices.Concat(errorList, el)
	warningList = slices.Concat(warningList, wl)
	el, wl = c.trConfig.Validate()
	errorList = slices.Concat(errorList, el)
	warningList = slices.Concat(warningList, wl)
	el, wl = c.metadataConfig.Validate()
	errorList = slices.Concat(errorList, el)
	warningList = slices.Concat(warningList, wl)
	return
}

func (c *parsedServer) Check() error {
	if !c.valid {
		return fmt.Errorf("config is invalid")
	}
	if c.trInstance == nil {
		return fmt.Errorf("exec transport is not initialized")
	}
	if c.dbInstance == nil {
		return fmt.Errorf("database is not initialized")
	}
	return nil
}

// try to transfer objects without reinitialization
// func (c *parsedServer) Reuse(old *parsedServer) {
// 	if old.trInstance != nil {
// 		if old.Conf.ExecType == c.Conf.ExecType && reflect.DeepEqual(c.Conf.ExecConfig, old.Conf.ExecConfig) {
// 			c.trInstance = old.trInstance
// 			old.trInstance = nil
// 			c.trInstance.SetCommands(&c.ExecReplacements)
// 		}
// 	}
// 	if old.dbInstance != nil {
// 		if old.Conf.DbType == c.Conf.DbType && reflect.DeepEqual(c.Conf.DbConfig, old.Conf.DbConfig) {
// 			c.dbInstance = old.dbInstance
// 			old.dbInstance = nil
// 		}
// 	}
// }

func (c *parsedServer) Cleanup() {
	if c.trInstance != nil {
		c.trInstance.Cleanup()
		c.trInstance = nil
	}
	if c.dbInstance != nil {
		c.dbInstance.Cleanup()
		c.dbInstance = nil
	}
}

func (c *parsedServer) InitTransport(ctx context.Context, logPrefix string) error {
	if c.trInstance != nil {
		return nil
	}

	if c.Conf.ExecType == "" {
		return fmt.Errorf("type is empty")
	}

	inflater, cfg, err := transportfactory.TransportFromType(c.Conf.ExecType, c.Conf.ExecConfig)
	if err != nil {
		return fmt.Errorf("TransportFromType: %w", err)
	}
	if inflater == nil {
		return fmt.Errorf("TransportFromType: nil result")
	}

	el, wl := cfg.Validate()
	logger := slogctx.FromCtx(ctx)
	for _, iss := range wl {
		logger.Warn(logPrefix + "exec." + iss)
	}
	for _, iss := range el {
		logger.Error(logPrefix + "exec." + iss)
	}
	if len(el) != 0 {
		return fmt.Errorf("config contains errors")
	}

	instance, err := inflater(ctx)
	if err != nil {
		return fmt.Errorf("inflate: %w", err)
	}

	c.trInstance = instance
	c.trConfig = cfg

	c.trInstance.SetCommands(&c.ExecReplacements)

	return nil
}

// InitDatabase fills parsedServer.dbInstance
// must be called after InitTransport
func (c *parsedServer) InitDatabase(ctx context.Context, logPrefix string) error {
	if c.dbInstance != nil {
		return nil
	}

	inflater, cfg, err := voldbfactory.VolumeDatabaseFromType(c.Conf.DbType, c.Conf.DbConfig)
	if err != nil {
		return fmt.Errorf("VolumeDatabaseFromType: %w", err)
	}

	el, wl := cfg.Validate()
	logger := slogctx.FromCtx(ctx)
	for _, iss := range wl {
		logger.Warn(logPrefix + "exec." + iss)
	}
	for _, iss := range el {
		logger.Error(logPrefix + "exec." + iss)
	}
	if len(el) != 0 {
		return fmt.Errorf("config contains errors")
	}

	instance, err := inflater(c.trInstance)
	if err != nil {
		return fmt.Errorf("inflate: %w", err)
	}

	c.dbInstance = instance
	c.metadataConfig = cfg

	return nil
}

func ParseDispatcherConfig(ctx context.Context, raw []byte) (*DispatcherConf, error) {
	logger := slogctx.FromCtx(ctx)

	dec := yaml.NewDecoder(bytes.NewBuffer(raw))
	dec.KnownFields(true)

	var config Config
	err := dec.Decode(&config)
	// err := yaml.UnmarshalStrict(raw, &result)
	if err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}

	sl := []*parsedServer{}
	for _, v := range config.Servers {
		srv := parsedServer{
			// TODO remove valid?
			valid:            true,
			Conf:             v,
			ExecReplacements: *transport.NewExecReplacement(v.Commands),
		}

		sl = append(sl, &srv)

		// TODO remove
		// el, wl := srv.Validate()
		// for _, iss := range el {
		// 	logger.Error(fmt.Sprintf("servers[i=%d/name=%s].%s", i, srv.conf.Name, iss))
		// }
		// for _, iss := range wl {
		// 	logger.Warn(fmt.Sprintf("servers[i=%d/name=%s].%s", i, srv.conf.Name, iss))
		// }
		// if len(el) > 0 {
		// 	srv.valid = false
		// }

		if srv.valid {
			logger.Info(fmt.Sprintf("successfully parsed server '%s'", srv.Conf.Name))
		} else {
			logger.Error(fmt.Sprintf("parsing server '%s' failed", srv.Conf.Name))
		}
	}

	return &DispatcherConf{
		alive:      true,
		NodeInfoDb: config.NodeInfoDb,
		Servers:    sl,
		Logs:       config.Logs,
	}, nil
}

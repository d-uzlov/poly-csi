package dispatcher

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"slices"
	"sync"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/config"
	"meoe.io/poly-csi/pkg/controller/voldb"
	"meoe.io/poly-csi/pkg/controller/voldb/voldbfactory"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/transportfactory"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type DispatcherConf struct {
	encrypt    parsedEncrypt
	nodeInfoDb NodeInfoStorageConfig
	servers    []*parsedServer
}

type parsedEncrypt struct {
	key  encrypt.RsaPrivateKey
	conf *EncryptConfig
}

func (c *DispatcherConf) Validate() (errorList []string, warningList []string) {
	if c.nodeInfoDb.Address == "" {
		errorList = append(errorList, "nodeInfo.address: field is empty")
	}
	for i, v := range c.servers {
		er, wl := v.Validate()
		for _, iss := range er {
			errorList = append(errorList, fmt.Sprintf("servers[i=%d/name=%s].%s", i, v.conf.Name, iss))
		}
		for _, iss := range wl {
			warningList = append(warningList, fmt.Sprintf("servers[i=%d/name=%s].%s", i, v.conf.Name, iss))
		}
	}
	return
}

func (c *DispatcherConf) FindServerByName(name string) *parsedServer {
	for _, v := range c.servers {
		if v.conf.Name == name {
			return v
		}
	}
	return nil
}

func (c *DispatcherConf) FindServerById(id string) *parsedServer {
	for _, v := range c.servers {
		if v.conf.StableId == id {
			return v
		}
	}
	return nil
}

type parsedServer struct {
	mutex sync.RWMutex
	valid bool
	conf  *ServerConfig

	trInstance transport.Transport
	trConfig   config.ValidatableConfig

	dbInstance     voldb.VolumeDatabase
	metadataConfig config.ValidatableConfig
}

func (c *parsedServer) Validate() (errorList []string, warningList []string) {
	el, wl := c.conf.Validate()
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
func (c *parsedServer) Reuse(old *parsedServer) {
	if old.trInstance != nil {
		if old.conf.ExecType == c.conf.ExecType && reflect.DeepEqual(c.conf.ExecConfig, old.conf.ExecConfig) {
			c.trInstance = old.trInstance
			old.trInstance = nil
		}
	}
	if old.dbInstance != nil {
		if old.conf.DbType == c.conf.DbType && reflect.DeepEqual(c.conf.DbConfig, old.conf.DbConfig) {
			c.dbInstance = old.dbInstance
			old.dbInstance = nil
		}
	}
}

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

	if c.conf.ExecType == "" {
		return fmt.Errorf("type is empty")
	}

	byteConfig, err := yaml.Marshal(c.conf.ExecConfig)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	inflater, cfg, err := transportfactory.TransportFromType(c.conf.ExecType, byteConfig)
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

	return nil
}

// InitDatabase fills parsedServer.dbInstance
// must be called after InitTransport
func (c *parsedServer) InitDatabase(ctx context.Context, logPrefix string) error {
	if c.dbInstance != nil {
		return nil
	}

	byteConfig, err := yaml.Marshal(c.conf.DbConfig)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	inflater, cfg, err := voldbfactory.VolumeDatabaseFromType(c.conf.DbType, byteConfig)
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

	rsa, err := encrypt.LoadPrivateKey(config.Encrypt.ControllerKey.Path)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}

	sl := []*parsedServer{}
	for _, v := range config.Servers {
		srv := parsedServer{
			// TODO remove valid?
			valid: true,
			conf:  v,
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
			logger.Info(fmt.Sprintf("successfully parsed server '%s'", srv.conf.Name))
		} else {
			logger.Error(fmt.Sprintf("parsing server '%s' failed", srv.conf.Name))
		}
	}

	return &DispatcherConf{
		encrypt: parsedEncrypt{
			key:  *rsa,
			conf: config.Encrypt,
		},
		nodeInfoDb: config.NodeInfoDb,
		servers:    sl,
	}, nil
}

package nodedispatcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type ParsedEncrypt struct {
	NodeKey       encrypt.RsaPrivateKey
	Conf          nodeconfig.NodeEncryptConfig
}

// TODO add cleanup policy for debugging
type parsedConfig struct {
	Encrypt                 ParsedEncrypt
	NodeInfoDb              nodeconfig.NodeInfoDbConfig
	Multipath               mpfactory.ParsedMultipath
	Mounts                  []*connectorfactory.ParsedConnector
	Networks                []nodeconfig.NodeNetworkConfig
	ExecConfig              localexec.Config
	Logs                    nodeconfig.Logs
	DisableCleanupOnFailure bool
}

func ParseConfig(ctx context.Context, raw []byte) (*parsedConfig, error) {
	logger := slogctx.FromCtx(ctx)

	dec := yaml.NewDecoder(bytes.NewBuffer(raw))
	dec.KnownFields(true)

	var config nodeconfig.NodeConfig
	err := dec.Decode(&config)
	// err := yaml.UnmarshalStrict(raw, &result)
	if err != nil {
		return nil, fmt.Errorf("config: yaml decode failed: %w", err)
	}

	logger.Info("parsing config", "new-config", config)

	nodeKey, err := loadOrGenerateKey(config.Encrypt.NodeKey.Path)
	if err != nil {
		return nil, fmt.Errorf("load node private key: %w", err)
	}

	ml := []*connectorfactory.ParsedConnector{}
	logger.Info(fmt.Sprintf("connectors in config: %d", len(config.Connectors)))
	for i, v := range config.Connectors {
		parsed := connectorfactory.ParseMount(ctx, v, config.DevFolder)
		if parsed == nil {
			logger.Info(fmt.Sprintf("parsing mount %d failed", i))
			continue
		}

		ml = append(ml, parsed)

		// el, wl := parsed.Validate()
		// for _, iss := range el {
		// 	logger.Error(fmt.Sprintf("servers[i=%d/id=%s].%s", i, parsed.conf.Id, iss))
		// }
		// for _, iss := range wl {
		// 	logger.Warn(fmt.Sprintf("servers[i=%d/id=%s].%s", i, parsed.conf.Id, iss))
		// }
		// if len(el) > 0 {
		// 	parsed.valid = false
		// }

		if parsed.Valid {
			logger.Info(fmt.Sprintf("successfully parsed connector '%s'", parsed.Conf.Id))
		} else {
			logger.Error(fmt.Sprintf("parsing connector '%s' failed", parsed.Conf.Id))
		}
	}

	mp := mpfactory.ParseMultipath(ctx, &config.Multipath, config.DevFolder)

	return &parsedConfig{
		Encrypt: ParsedEncrypt{
			NodeKey:       *nodeKey,
			Conf:          config.Encrypt,
		},
		NodeInfoDb:              config.NodeInfoDb,
		Multipath:               *mp,
		Mounts:                  ml,
		Networks:                config.Networks,
		ExecConfig:              config.ExecConfig,
		Logs:                    config.Logs,
		DisableCleanupOnFailure: config.DisableCleanupOnFailure,
	}, nil
}

func loadOrGenerateKey(filepath string) (*encrypt.RsaPrivateKey, error) {
	rsa, err := encrypt.LoadPrivateKey(filepath)
	if err == nil {
		return rsa, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	rsa, err = encrypt.GenerateRsa()
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath, rsa.SerializePem(), 0600)
	if err != nil {
		return nil, err
	}
	return rsa, nil
}

func (c *parsedConfig) findConnectorById(id string) *connectorfactory.ParsedConnector {
	for _, v := range c.Mounts {
		if v.Conf.Id == id {
			return v
		}
	}

	return nil
}

func (c *parsedConfig) getRpcLogConfig(rpcPath string) logs.Config {
	result := logs.Config{
		LogLevel:     "info",
		EchoCommands: false,
		ExtraFields: map[string]string{
			"scope":  "method",
			"method": rpcPath,
		},
	}
	result = result.Combine(c.Logs.Default)
	config, ok := c.Logs.Rpc[rpcPath]
	if ok {
		result = result.Combine(config)
	}
	return result
}

func (c *parsedConfig) getNodeInfoLogConfig() logs.Config {
	result := logs.Config{
		LogLevel:     "info",
		EchoCommands: false,
		ExtraFields: map[string]string{
			"scope": "node-info",
		},
	}
	result = result.Combine(c.Logs.Default)
	result = result.Combine(c.Logs.NodeInfo)
	return result
}

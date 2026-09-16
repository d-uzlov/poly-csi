package nodedispatcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

func ParseConfig(ctx context.Context, raw []byte) (*nodeconfig.ParsedConfig, error) {
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

	netList := []nodeconfig.ParsedNetworks{}
	for i, v := range config.Networks {
		networks, err := parseNetworks(v)
		if err != nil {
			return nil, fmt.Errorf("networks[i=%d]: %w", i, err)
		}
		netList = append(netList, networks)
	}

	nodeKey, err := loadOrGenerateKey(config.Encrypt.NodeKey.Path)
	if err != nil {
		return nil, fmt.Errorf("load node private key: %w", err)
	}

	connectorList := []string{}
	for _, v := range config.Connectors {
		connectorList = append(connectorList, v.Id)
	}

	ml := []*connectorfactory.ParsedConnector{}
	logger.Info(fmt.Sprintf("connectors in config: %d: %v", len(connectorList), connectorList))
	for i, v := range config.Connectors {
		parsed := connectorfactory.ParseConnector(ctx, v)
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

	presetMap := map[string]map[string]string{}
	for i, v := range config.Presets {
		if v.Name == "" {
			logger.Error(fmt.Sprintf("preset[i=%d]: missing 'name' field", i))
			continue
		}
		if _, ok := presetMap[v.Name]; ok {
			logger.Error(fmt.Sprintf("preset[i=%d]: duplicate preset with name=%s", i, v.Name))
			continue
		}
		presetMap[v.Name] = v.Params
	}

	return &nodeconfig.ParsedConfig{
		Encrypt: nodeconfig.ParsedEncrypt{
			NodeKey: *nodeKey,
			Conf:    config.Encrypt,
		},
		NodeInfoDb:              config.NodeInfoDb,
		DevFolder:               config.DevFolder,
		PodCgroupRoot:           config.PodCgroupRoot,
		Multipath:               *mp,
		Connectors:              ml,
		Presets:                 presetMap,
		Networks:                netList,
		ExecConfig:              config.ExecConfig,
		Logs:                    config.Logs,
		DisableCleanupOnFailure: config.DisableCleanupOnFailure,
	}, nil
}

func parseNetworks(c nodeconfig.NodeNetworkConfig) (nodeconfig.ParsedNetworks, error) {
	result := nodeconfig.ParsedNetworks{}
	for i, v := range c.Include {
		_, cidr, err := net.ParseCIDR(v)
		if err != nil {
			return nodeconfig.ParsedNetworks{}, fmt.Errorf("include[i=%d]='%s': %w", i, v, err)
		}
		if cidr == nil {
			return nodeconfig.ParsedNetworks{}, fmt.Errorf("include[i=%d]='%s': cidr not found", i, v)
		}
		result.Include = append(result.Include, *cidr)
	}
	for i, v := range c.Exclude {
		_, cidr, err := net.ParseCIDR(v)
		if err != nil {
			return nodeconfig.ParsedNetworks{}, fmt.Errorf("exclude[i=%d]='%s': %w", i, v, err)
		}
		if cidr == nil {
			return nodeconfig.ParsedNetworks{}, fmt.Errorf("exclude[i=%d]='%s': cidr not found", i, v)
		}
		result.Exclude = append(result.Exclude, *cidr)
	}
	return result, nil
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

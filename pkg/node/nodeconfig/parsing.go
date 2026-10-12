package nodeconfig

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/hpke"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/blockcon"
	"meoe.io/poly-csi/pkg/node/blockcon/blockfactory"
	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/node/filecon/fsfactory"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
)

func ParseConfig(ctx context.Context, raw []byte) (*ParsedConfig, error) {
	logger := logs.FromCtx(ctx)

	dec := yaml.NewDecoder(bytes.NewBuffer(raw))
	dec.KnownFields(true)

	var config NodeConfig
	err := dec.Decode(&config)
	// err := yaml.UnmarshalStrict(raw, &result)
	if err != nil {
		return nil, fmt.Errorf("config: yaml decode failed: %w", err)
	}

	logger.Info("parsing config", "new-config", config)

	netList := []ParsedNetworks{}
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

	// connectorList := []string{}
	// for _, v := range config.Connectors {
	// 	connectorList = append(connectorList, v.Id)
	// }

	// TODO can we move more tr usages here?
	tr := localexec.New(config.ExecConfig)

	// logger.Info(fmt.Sprintf("connectors in config: %d: %v", len(connectorList), connectorList))
	bm := ParseBlockList(logger, tr, config.DevFolder, config.BlockConnectors)
	fm := ParseFilesystems(logger, tr, config.Filesystems)

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

	return &ParsedConfig{
		Encrypt: ParsedEncrypt{
			NodeKey: nodeKey,
			Conf:    config.Encrypt,
		},
		NodeInfoDb:              config.NodeInfoDb,
		DevFolder:               config.DevFolder,
		PodCgroupRoot:           config.PodCgroupRoot,
		Multipath:               *mp,
		BlockConnectors:         bm,
		FsConnectors:            fm,
		Presets:                 presetMap,
		Networks:                netList,
		ExecConfig:              config.ExecConfig,
		Logs:                    config.Logs,
		DisableCleanupOnFailure: config.DisableCleanupOnFailure,
	}, nil
}

func parseNetworks(c NodeNetworkConfig) (ParsedNetworks, error) {
	result := ParsedNetworks{}
	for i, v := range c.Include {
		_, cidr, err := net.ParseCIDR(v)
		if err != nil {
			return ParsedNetworks{}, fmt.Errorf("include[i=%d]='%s': %w", i, v, err)
		}
		if cidr == nil {
			return ParsedNetworks{}, fmt.Errorf("include[i=%d]='%s': cidr not found", i, v)
		}
		result.Include = append(result.Include, *cidr)
	}
	for i, v := range c.Exclude {
		_, cidr, err := net.ParseCIDR(v)
		if err != nil {
			return ParsedNetworks{}, fmt.Errorf("exclude[i=%d]='%s': %w", i, v, err)
		}
		if cidr == nil {
			return ParsedNetworks{}, fmt.Errorf("exclude[i=%d]='%s': cidr not found", i, v)
		}
		result.Exclude = append(result.Exclude, *cidr)
	}
	return result, nil
}

func loadOrGenerateKey(filepath string) (hpke.PrivateKey, error) {
	kem := hpke.DHKEM(ecdh.X25519())

	keyBytes, err := os.ReadFile(filepath)
	if err == nil {
		return kem.NewPrivateKey(keyBytes)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	key, err := kem.GenerateKey()
	if err != nil {
		return nil, err
	}
	keyBytes, err = key.Bytes()
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath, keyBytes, 0600)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func ParseBlockList(logger *logs.Instance, tr transport.Transport, devFolder string, list []BlockSpec) map[string]blockcon.Manager {
	cons := map[string]blockcon.Manager{}
	for i, v := range list {
		if v.Id == "" {
			logger.Error(fmt.Sprintf("block[i=%d]: missing id", i))
			continue
		}
		if _, ok := cons[v.Id]; ok {
			logger.Error(fmt.Sprintf("block[i=%d/id=%s]: duplicate id", i, v.Id))
			continue
		}

		if v.Type == "" {
			logger.Error(fmt.Sprintf("block[i=%d/id=%s]: missing type", i, v.Id))
			continue
		}
		inflate, err := blockfactory.GetByType(v.Type)
		if err != nil {
			logger.Error(fmt.Sprintf("block[i=%d/id=%s]: GetByType failed", i, v.Id), "error", err)
			continue
		}
		if inflate == nil {
			logger.Error(fmt.Sprintf("block[i=%d/id=%s]: GetByType returned nil", i, v.Id))
			continue
		}
		con, err := inflate(blockcon.CommonConfig{
			Tr:        tr,
			DevFolder: devFolder,
			Cfg:       v.Config,
		})
		if err != nil {
			logger.Error(fmt.Sprintf("block[i=%d/id=%s]: inflate failed", i, v.Id), "error", err)
		}

		cons[v.Id] = con
	}
	return cons
}

func ParseFilesystems(logger *logs.Instance, tr transport.Transport, list []FsSpec) map[string]filecon.Manager {
	cons := map[string]filecon.Manager{}
	for i, v := range list {
		if v.Id == "" {
			logger.Error(fmt.Sprintf("filesystem[i=%d]: missing id", i))
			continue
		}
		if _, ok := cons[v.Id]; ok {
			logger.Error(fmt.Sprintf("filesystem[i=%d/id=%s]: duplicate id", i, v.Id))
			continue
		}

		if v.Type == "" {
			logger.Error(fmt.Sprintf("filesystem[i=%d/id=%s]: missing type", i, v.Id))
			continue
		}
		fsInfo, err := fsfactory.GetByType(v.Type)
		if err != nil {
			logger.Error(fmt.Sprintf("filesystem[i=%d/id=%s]: GetByType failed", i, v.Id), "error", err)
			continue
		}
		if fsInfo.FsNew == nil {
			logger.Error(fmt.Sprintf("filesystem[i=%d/id=%s]: GetByType returned nil", i, v.Id))
			continue
		}
		con, err := fsInfo.FsNew(filecon.CommonConfig{
			Tr:  tr,
			Cfg: v.Config,
		})
		if err != nil {
			logger.Error(fmt.Sprintf("filesystem[i=%d/id=%s]: inflate failed", i, v.Id), "error", err)
		}

		cons[v.Id] = con
	}
	return cons
}

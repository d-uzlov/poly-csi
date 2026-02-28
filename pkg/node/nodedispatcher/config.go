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
	"meoe.io/poly-csi/pkg/node/mounts/mounterfactory"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/node/nodeconfig"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type ParsedEncrypt struct {
	NodeKey       encrypt.RsaPrivateKey
	ControllerKey encrypt.RsaPublicKey
	Conf          nodeconfig.NodeEncryptConfig
}

// TODO add cleanup policy for debugging
type parsedConfig struct {
	Encrypt    ParsedEncrypt
	NodeInfoDb nodeconfig.NodeInfoDbConfig
	Multipath  mpfactory.ParsedMultipath
	Mounts     []*mounterfactory.ParsedMount
	Networks   []nodeconfig.NodeNetworkConfig
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

	nodeKey, err := loadOrGenerateKey(config.Encrypt.NodeKey.Path)
	if err != nil {
		return nil, fmt.Errorf("load node private key: %w", err)
	}

	controllerKey, err := encrypt.LoadPublicKey(config.Encrypt.ControllerPublicKey.Path)
	if err != nil {
		return nil, fmt.Errorf("load controller public key: %w", err)
	}

	ml := []*mounterfactory.ParsedMount{}
	logger.Info(fmt.Sprintf("mounts in config: %d", len(config.Mounters)))
	for i, v := range config.Mounters {
		parsed := mounterfactory.ParseMount(ctx, v)
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
			logger.Info(fmt.Sprintf("successfully parsed mounter '%s'", parsed.Conf.Id))
		} else {
			logger.Error(fmt.Sprintf("parsing mounter '%s' failed", parsed.Conf.Id))
		}
	}

	mp := mpfactory.ParseMultipath(ctx, &config.Multipath)

	return &parsedConfig{
		Encrypt: ParsedEncrypt{
			NodeKey:       *nodeKey,
			ControllerKey: *controllerKey,
			Conf:          config.Encrypt,
		},
		NodeInfoDb: config.NodeInfoDb,
		Multipath:  *mp,
		Mounts:     ml,
		Networks:   config.Networks,
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

func (c *parsedConfig) findMounterById(id string) *mounterfactory.ParsedMount {
	for _, v := range c.Mounts {
		if v.Conf.Id == id {
			return v
		}
	}

	return nil
}

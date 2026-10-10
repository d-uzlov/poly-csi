package cntraoe

import (
	"context"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/api/protocols"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type connector struct {
	connectors.NoInfo

	conf connectors.CommonConfig
}

type aoeConfig struct {
	Interfaces []string `yaml:"interfaces"`
	DeadSecs   string   `yaml:"deadSecs"`
	MaxOut     string   `yaml:"maxOut"`
	MaxSectors string   `yaml:"maxSectors"`
}

func New(conf connectors.CommonConfig) (connectors.Manager, error) {
	var cfg aoeConfig
	err := conf.Cfg.Decode(cfg)
	if err != nil {
		return nil, fmt.Errorf("parse aoe config: %w", err)
	}
	result := &connector{
		conf: conf,
	}
	// TODO move this somewhere where context is available
	err = result.setAoeModuleParams(context.TODO(), cfg)
	if err != nil {
		return nil, fmt.Errorf("set module parameters: %w", err)
	}
	return result, nil
}

func (c *connector) setAoeModuleParams(ctx context.Context, conf aoeConfig) error {
	const folder = "/sys/module/aoe/parameters/"
	err := c.conf.Tr.Cmds().Write(ctx, folder+"/aoe_iflist", []byte(strings.Join(conf.Interfaces, " ")))
	if err != nil {
		return fmt.Errorf("set aoe_iflist: %w", err)
	}

	if conf.DeadSecs == "" {
		// disable timeout by default
		conf.DeadSecs = "0"
	}
	err = c.conf.Tr.Cmds().Write(ctx, folder+"/aoe_deadsecs", []byte(conf.DeadSecs))
	if err != nil {
		return fmt.Errorf("set aoe_deadsecs: %w", err)
	}

	if conf.MaxOut == "" {
		conf.MaxOut = "64"
	}
	err = c.conf.Tr.Cmds().Write(ctx, folder+"/aoe_maxout", []byte(conf.MaxOut))
	if err != nil {
		return fmt.Errorf("set aoe_maxout: %w", err)
	}

	if conf.MaxSectors == "" {
		// 2560 seems to be the default value on modern linux
		conf.MaxSectors = "2560"
	}
	err = c.conf.Tr.Cmds().Write(ctx, folder+"/aoe_maxsectors", []byte(conf.MaxSectors))
	if err != nil {
		return fmt.Errorf("set aoe_maxsectors: %w", err)
	}

	return nil
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		Protocol:    protocols.Aoe,
		HasNodeInfo: false,
		ConnectType: connectors.ConnectTypeBlock,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req connectors.ConnectRequest) (connectors.Handle, error) {
	if len(req.AddressOptions) != 1 {
		return nil, fmt.Errorf("aoe connector does not support more than one address")
	}

	pc, err := protocols.Parse[protocols.AoeShare](req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("protocols.Parse: %w", err)
	}

	return &state{
		tr:        c.conf.Tr,
		devFolder: c.conf.DevFolder,
		Pc:        pc,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData string) (connectors.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.conf.Tr
	res.devFolder = c.conf.DevFolder

	return res, nil
}

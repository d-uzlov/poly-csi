package cntraoe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/blockspec"
)

type connector struct {
	blockapi.NoInfo

	conf blockapi.CommonConfig
}

type AoeConfig struct {
	Interfaces []string `yaml:"interfaces"`
	DeadSecs   string   `yaml:"deadSecs"`
	MaxOut     string   `yaml:"maxOut"`
	MaxSectors string   `yaml:"maxSectors"`
}

func New(conf blockapi.CommonConfig) (blockapi.Manager, error) {
	var cfg AoeConfig
	err := conf.Cfg.Decode(&cfg)
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

func (c *connector) setAoeModuleParams(ctx context.Context, conf AoeConfig) error {
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

func (c *connector) Capabilities(ctx context.Context) (blockapi.Capabilities, error) {
	return blockapi.Capabilities{
		Protocol:    blockspec.Aoe,
		HasNodeInfo: false,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req blockapi.ConnectRequest) (blockapi.Handle, error) {
	proto, ok := req.Proto.(blockspec.AoeShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch: got %T", req.Proto)
	}

	return &state{
		tr:        c.conf.Tr,
		devFolder: c.conf.DevFolder,
		Pc:        proto,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData json.RawMessage) (blockapi.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.conf.Tr
	res.devFolder = c.conf.DevFolder

	return res, nil
}

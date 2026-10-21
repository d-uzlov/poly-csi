package cntriscsiadm

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/blockspec"
)

type connector struct {
	commonConfig blockapi.CommonConfig
	config       Config
}

func New(conf blockapi.CommonConfig) (blockapi.Manager, error) {
	result := &connector{}
	result.commonConfig = conf
	err := conf.Cfg.Decode(&result.config)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	result.config.FillDefaults()
	if err := result.config.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return result, nil
}

func (c *connector) Configure() error {
	return nil
}

func (c *connector) Capabilities(ctx context.Context) (blockapi.Capabilities, error) {
	return blockapi.Capabilities{
		Protocol:    blockspec.Iscsi,
		HasNodeInfo: true,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req blockapi.ConnectRequest) (blockapi.Handle, error) {
	proto, ok := req.Proto.(blockspec.IscsiShare)
	if !ok {
		return nil, fmt.Errorf("protocol mismatch")
	}

	snip := newIscsiSnippets(c.commonConfig.Tr, c.config.IscsiadmCmd)

	attributes := map[string]string{}

	switch proto.AuthType {
	case blockspec.IscsiAuthNone:
		attributes["node.session.auth.authmethod"] = "None"
	case blockspec.IscsiAuthChap:
		attributes["node.session.auth.authmethod"] = "CHAP"
		attributes["node.session.auth.username"] = proto.ClientUser
		attributes["node.session.auth.password"] = proto.ClientPass
		attributes["node.session.auth.username_in"] = proto.TargetUser
		attributes["node.session.auth.password_in"] = proto.TargetPass
	default:
		return nil, fmt.Errorf("unknown chap type: '%s'", proto.AuthType)
	}

	var ecs []string

	for _, ao := range proto.ServerAddress {
		ecs = append(ecs, ao)
	}

	return &state{
		tr:                      c.commonConfig.Tr,
		snip:                    &snip,
		DevFolder:               c.commonConfig.DevFolder,
		DisableCleanupOnFailure: req.DisableCleanupOnFailure,
		Iqn:                     proto.Iqn,
		Port:                    proto.Port,
		Attributes:              attributes,
		Connections:             ecs,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData string) (blockapi.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.commonConfig.Tr
	snip := newIscsiSnippets(c.commonConfig.Tr, c.config.IscsiadmCmd)
	res.snip = &snip

	return res, nil
}

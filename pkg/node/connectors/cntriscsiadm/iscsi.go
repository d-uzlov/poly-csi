package cntriscsiadm

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/api/protocols"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type connector struct {
	commonConfig connectors.CommonConfig
	config       Config
}

func New(conf connectors.CommonConfig) (connectors.Manager, error) {
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

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		Protocol:    protocols.Iscsi,
		HasNodeInfo: true,
		ConnectType: connectors.ConnectTypeBlock,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req connectors.ConnectRequest) (connectors.Handle, error) {
	pc, err := protocols.Parse[protocols.IscsiShare](req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("protocols.Parse: %w", err)
	}

	snip := newIscsiSnippets(c.commonConfig.Tr, c.config.IscsiadmCmd)

	attributes := map[string]string{}

	switch pc.AuthType {
	case protocols.IscsiAuthNone:
		attributes["node.session.auth.authmethod"] = "None"
	case protocols.IscsiAuthChap:
		attributes["node.session.auth.authmethod"] = "CHAP"
		attributes["node.session.auth.username"] = pc.ClientUser
		attributes["node.session.auth.password"] = pc.ClientPass
		attributes["node.session.auth.username_in"] = pc.TargetUser
		attributes["node.session.auth.password_in"] = pc.TargetPass
	default:
		return nil, fmt.Errorf("unknown chap type: '%s'", pc.AuthType)
	}

	var ecs []Connection

	for _, ao := range req.AddressOptions {
		ecs = append(ecs, Connection{
			Addr: ao,
		})
	}

	return &state{
		tr:                      c.commonConfig.Tr,
		snip:                    &snip,
		DevFolder:               c.commonConfig.DevFolder,
		DisableCleanupOnFailure: req.DisableCleanupOnFailure,
		Iqn:                     pc.Iqn,
		Port:                    pc.Port,
		Attributes:              attributes,
		Connections:             ecs,
	}, nil
}

func (c *connector) ParseHandle(serializedConnectorData string) (connectors.Handle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.commonConfig.Tr
	snip := newIscsiSnippets(c.commonConfig.Tr, c.config.IscsiadmCmd)
	res.snip = &snip

	return res, nil
}

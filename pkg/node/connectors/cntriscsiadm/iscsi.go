package cntriscsiadm

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type connector struct {
	commonConfig connectors.CommonConfig
	config       Config
}

func New() connectors.Connector {
	return &connector{}
}

func (c *connector) Configure(ctx context.Context, conf connectors.CommonConfig, cfg yaml.Node) error {
	c.commonConfig = conf
	err := cfg.Decode(&c.config)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	c.config.FillDefaults()
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		ConnectType: connectors.ConnectTypeBlock,
	}, nil
}

func (c *connector) GetHandle(ctx context.Context, req connectors.ConnectRequest) (connectors.ConnectionHandle, error) {
	pc, sc, err := ParsePublishContext(req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	err = validateContext(pc, sc)
	if err != nil {
		return nil, fmt.Errorf("validateContext: %w", err)
	}

	snip := newIscsiSnippets(c.commonConfig.Tr, c.config.IscsiadmCmd)

	attributes := map[string]string{}

	switch pc.AuthType {
	case AuthNone:
		attributes["node.session.auth.authmethod"] = "None"
	case AuthChapFromPublishSecret:
		attributes["node.session.auth.authmethod"] = "CHAP"
		attributes["node.session.auth.username"] = sc.ClientUser
		attributes["node.session.auth.password"] = sc.ClientPass
		attributes["node.session.auth.username_in"] = sc.TargetUser
		attributes["node.session.auth.password_in"] = sc.TargetPass
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

func (c *connector) ParseHandle(serializedConnectorData string) (connectors.ConnectionHandle, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.tr = c.commonConfig.Tr
	snip := newIscsiSnippets(c.commonConfig.Tr, c.config.IscsiadmCmd)
	res.snip = &snip

	return res, nil
}

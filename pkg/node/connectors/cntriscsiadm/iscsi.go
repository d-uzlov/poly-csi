package cntriscsiadm

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
)

type connector struct {
	commonConfig connectors.CommonConfig
	config       Config
}

func New() connectors.Connector {
	return &connector{
	}
}

func (c *connector) Configure(ctx context.Context, conf connectors.CommonConfig, opaqueConfig []byte) error {
	c.commonConfig = conf
	if err := yaml.Unmarshal(opaqueConfig, &c.config); err != nil {
		return fmt.Errorf("parse opaqueConfig: %w", err)
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

func (c *connector) Connect(ctx context.Context, req connectors.ConnectRequest) (connectors.State, error) {
	pc, sc, err := ParsePublishContext(req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	err = validateContext(pc, sc)
	if err != nil {
		return nil, fmt.Errorf("validateContext: %w", err)
	}

	snip := newIscsiSnippets(c.commonConfig.Tr, ctx, c.config.IscsiadmCmd)

	// setting up cleanup before even attempting to connect
	// because failure could be caused by invalid state, that should be forcefully disconnected.
	defer func() {
		if req.DisableCleanupOnFailure {
			return
		}
		if err == nil {
			return
		}
		logs.FromCtx(ctx).MicroOp("detected failure, running cleanup")
		for _, v := range req.AddressOptions {
			addr := fmt.Sprintf("%s:%s", v.Address, pc.Port)
			cleanupErr := snip.disconnect(pc.Iqn, addr)
			if cleanupErr != nil {
				logs.FromCtx(ctx).Error("failed to run cleanup", "address", addr, "error", cleanupErr.Error())
			}
		}
	}()

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

	var combinedError string
	successCount := 0
	for _, v := range req.AddressOptions {
		addr := fmt.Sprintf("%s:%s", v.Address, pc.Port)
		err = snip.configureTarget(pc.Iqn, addr, attributes)
		if err != nil {
			if len(req.AddressOptions) <= 1 {
				return nil, fmt.Errorf("iscsi: configureTarget: %w", err)
			}

			logs.FromCtx(ctx).Error("iscsi: configureTarget failed", "address", addr, "error", err.Error())
			combinedError += fmt.Sprintf("failed to configure connection to %s: %s ; ", addr, err.Error())
			continue
		}
		err = snip.connect(pc.Iqn, addr)
		if err != nil {
			if len(req.AddressOptions) <= 1 {
				return nil, fmt.Errorf("iscsi: connect: %w", err)
			}

			logs.FromCtx(ctx).Error("iscsi: connect failed", "address", addr, "error", err.Error())
			combinedError += fmt.Sprintf("failed to connect to %s: %s ; ", addr, err.Error())
			continue
		}

		successCount++
	}
	if successCount == 0 {
		return nil, fmt.Errorf("all %d iscsi multipath connections failed: %s", len(req.AddressOptions), combinedError)
	}

	return &state{
		con: c,
		Iqn: pc.Iqn,
	}, nil
}

func (c *connector) ParseState(serializedConnectorData string) (connectors.State, error) {
	res, err := parseState(serializedConnectorData)
	if err != nil {
		return nil, err
	}

	res.con = c

	return res, nil
}

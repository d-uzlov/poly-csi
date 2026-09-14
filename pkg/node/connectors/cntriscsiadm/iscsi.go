package cntriscsiadm

import (
	"context"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type connector struct {
	tr        transport.Transport
	config    Config
	devFolder string
}

func New(config Config, devFolder string) connectors.Connector {
	return &connector{
		config:    config,
		devFolder: devFolder,
	}
}

func (c *connector) Configure(ctx context.Context, tr transport.Transport, preset map[string]string) error {
	c.tr = tr
	return nil
}

func (c *connector) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	return connectors.Capabilities{
		ConnectType: connectors.ConnectTypeBlock,
	}, nil
}

func (m *connector) Connect(ctx context.Context, req connectors.ConnectRequest) (result *connectors.ConnectResult, err error) {
	pc, sc, err := ParsePublishContext(req.PublishContext, req.PublishSecret)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}
	err = validateContext(pc, sc)
	if err != nil {
		return nil, fmt.Errorf("validateContext: %w", err)
	}

	snip := newIscsiSnippets(m.tr, ctx, m.config.IscsiadmCmd)

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

	is, err := snip.findSessions(pc.Iqn)
	if err != nil {
		return nil, fmt.Errorf("find iscsi sessions: %w", err)
	}

	logs.FromCtx(ctx).MicroOp("iscsi: find device paths")
	var devices []string
	for _, v := range is {
		// /sys/class/iscsi_session/sessionX/state
		// /sys/class/iscsi_connection/connectionX\:0/state
		statePath := fmt.Sprintf("/sys/class/iscsi_session/session%s/state", v.index)
		err = transport.Retry(time.Millisecond*100, time.Second, func() error {
			stateBytes, err := m.tr.Cmds().Read(ctx, statePath, true)
			if err != nil {
				// runtime errors should be self-explanatory
				return err
			}
			state := string(stateBytes)
			if state != "LOGGED_IN" {
				return fmt.Errorf("unexpected iscsi session state: %s", state)
			}
			return nil
		})
		if err != nil {
			combinedError += fmt.Sprintf("iscsi device for %s is not online: %s ; ", v.address, err.Error())
			logs.FromCtx(ctx).Error("iscsi device is not online", "address", v.address, "error", err.Error())
			continue
		}

		deviceLink := fmt.Sprintf(m.devFolder+"/disk/by-path/ip-%s-iscsi-%s-lun-0", v.address, v.iqn)
		// iscsi creates links in background, need to wait for it to appear
		err = transport.Retry(time.Millisecond*100, time.Second, func() error {
			stat, err := m.tr.Cmds().Stat(ctx, deviceLink)
			if err != nil {
				// runtime errors should be self-explanatory
				return err
			}
			if !stat.FileExists {
				return fmt.Errorf("link does not exist: %s", deviceLink)
			}
			devices = append(devices, stat.Path)
			return nil
		})
		if err != nil {
			combinedError += fmt.Sprintf("failed to get iscsi device file link at %s: %s ; ", deviceLink, err.Error())
			logs.FromCtx(ctx).Error("failed to get iscsi device file link", "address", v.address, "path", deviceLink, "error", err.Error())
		}
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("iscsi device is not found: %s", combinedError)
	}
	if len(devices) < len(req.AddressOptions) {
		logs.FromCtx(ctx).Warn("iscsi: some addresses failed to connect, returning partial list")
	}
	return &connectors.ConnectResult{
		Devices:    devices,
		ConnectorData: VolumeData{
			Iqn: pc.Iqn,
		},
	}, nil
}

func (m *connector) Disconnect(ctx context.Context, serializedConnectorData string) error {
	vd, err := parseVolumeData(serializedConnectorData)
	if err != nil {
		return fmt.Errorf("parse serialized connector data: %w", err)
	}

	snip := newIscsiSnippets(m.tr, ctx, m.config.IscsiadmCmd)

	is, err := snip.findSessions(vd.Iqn)
	if err != nil {
		return fmt.Errorf("find iscsi sessions: %w", err)
	}
	for _, v := range is {
		err = snip.disconnect(vd.Iqn, v.address)
		if err != nil {
			return fmt.Errorf("iscsi disconnect from %s: %w", v.address, err)
		}
	}
	return nil
}

func (m *connector) Expand(ctx context.Context, serializedConnectorData string) error {
	snip := newIscsiSnippets(m.tr, ctx, m.config.IscsiadmCmd)
	// TODO rescan only target devices?
	err := snip.rescanAll()
	if err != nil {
		return err
	}

	return nil
}

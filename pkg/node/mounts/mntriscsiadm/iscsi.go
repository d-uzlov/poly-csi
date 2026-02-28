package mntriscsiadm

import (
	"context"
	"fmt"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/transport"
)

type iscsiadmMounter struct {
	config    Config
	devFolder string
}

func NewIscsiadmMounter(config Config, devFolder string) mounts.NodeMounter {
	return &iscsiadmMounter{
		config:    config,
		devFolder: devFolder,
	}
}

func (m *iscsiadmMounter) Connect(ctx context.Context, req mounts.ConnectRequest) (*mounts.ConnectResult, error) {
	pc, err := ParsePublishContext(req.PublishContext)
	if err != nil {
		return nil, fmt.Errorf("ParsePublishContext: %w", err)
	}

	attributes := map[string]string{}

	switch pc.Chap {
	case ChapNone:
		attributes["node.session.auth.authmethod"] = "None"
	case ChapFromPublishSecret:
		clientChap, err := ParseChap(req.PublishContextSecret, "chap.client")
		if err != nil {
			return nil, fmt.Errorf("parse chap.client from publish secret: %w", err)
		}
		targetChap, err := ParseChap(req.PublishContextSecret, "chap.target")
		if err != nil {
			return nil, fmt.Errorf("parse chap.target from publish secret: %w", err)
		}

		attributes["node.session.auth.authmethod"] = "CHAP"
		attributes["node.session.auth.username"] = clientChap.User
		attributes["node.session.auth.password"] = clientChap.Pass
		attributes["node.session.auth.username_in"] = targetChap.User
		attributes["node.session.auth.password_in"] = targetChap.Pass
	default:
		return nil, fmt.Errorf("unknown chap type: '%s'", pc.Chap)
	}

	snip := newIscsiSnippets(ctx, m.config.IscsiadmCmd)

	tr := transport.FromContext(ctx)

	var combinedError string
	successCount := 0
	for _, v := range req.AddressOptions {
		addr := fmt.Sprintf("%s:%s", v.Address, pc.Port)
		err = snip.configureTarget(pc.Iqn, addr, attributes)
		if err != nil {
			if len(req.AddressOptions) <= 1 {
				return nil, fmt.Errorf("iscsi: configureTarget: %w", err)
			}

			slogctx.FromCtx(ctx).Error("iscsi: configureTarget failed", "address", addr, "error", err)
			combinedError += fmt.Sprintf("failed to configure connection to %s: %s ; ", addr, err.Error())
			continue
		}
		err = snip.connect(pc.Iqn, addr)
		if err != nil {
			if len(req.AddressOptions) <= 1 {
				return nil, fmt.Errorf("iscsi: connect: %w", err)
			}

			slogctx.FromCtx(ctx).Error("iscsi: connect failed", "address", addr, "error", err)
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
	var devices []string
	for _, v := range is {
		deviceLink := fmt.Sprintf(m.devFolder + "/disk/by-path/ip-%s-iscsi-%s-lun-0", v.address, v.iqn)
		// iscsi creates links in background, need to wait for it to appear
		err = transport.Retry(time.Millisecond*100, time.Second, func() error {
			stat, err := tr.Cmds().Stat(ctx, deviceLink)
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
			slogctx.FromCtx(ctx).Error("failed to get iscsi device file link", "address", v.address, "path", deviceLink, "error", err)
		}
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("iscsi device is not found: %s", combinedError)
	}
	return &mounts.ConnectResult{
		FolderPath: "",
		Devices:    devices,
		ManagedOps: mounts.ManagedOperations{
			MountFs:   true,
			UnmountFs: true,
			ExpandFs:  true,
		},
		MounterData: VolumeData{
			Iqn: pc.Iqn,
		},
	}, nil
}

func (m *iscsiadmMounter) Disconnect(ctx context.Context, serializedMounterData string) error {
	vd, err := parseVolumeData(serializedMounterData)
	if err != nil {
		return fmt.Errorf("parse serialized mounter data: %w", err)
	}

	snip := newIscsiSnippets(ctx, m.config.IscsiadmCmd)

	is, err := snip.findSessions(vd.Iqn)
	if err != nil {
		return fmt.Errorf("find iscsi sessions: %w", err)
	}
	for _, v := range is {
		err = snip.disconnect(vd.Iqn, v.address)
		if err != nil {
			return fmt.Errorf("nvmeDisconnect: %w", err)
		}
	}
	return nil
}

func (m *iscsiadmMounter) Expand(ctx context.Context, serializedMounterData string) error {
	snip := newIscsiSnippets(ctx, m.config.IscsiadmCmd)
	// TODO rescan only target devices?
	err := snip.rescanAll()
	if err != nil {
		return err
	}

	return nil
}

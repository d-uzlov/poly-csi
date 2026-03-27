package volumeinit

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/fsutils"
)

const PreferredPrefix = "init."

const (
	placeController = "controller"
	placeNode       = "node"
)

type InitConfig struct {
	On          string `map:"on"          json:"on"`
	BlockFormat string `map:"blockFormat" json:"block_format"`
	OwnerUser   string `map:"ownerUser"   json:"owner_user"`
	OwnerGroup  string `map:"ownerGroup"  json:"owner_group"`
	RootDirMode string `map:"rootDirMode" json:"root_dir_mode"`
}

func (c *InitConfig) Validate() error {
	switch c.On {
	case "":
		return fmt.Errorf("missing 'init on' setting")
	case placeNode, placeController:
	default:
		return fmt.Errorf("unknown value for 'on': %s", c.On)
	}

	// if c.BlockFormat == "" {
	// 	return fmt.Errorf("block format cmd")
	// }
	if c.OwnerUser == "" {
		return fmt.Errorf("missing owner user")
	}
	if c.OwnerGroup == "" {
		return fmt.Errorf("missing owner group")
	}
	if c.RootDirMode == "" {
		return fmt.Errorf("missing root dir mode")
	}

	return nil
}

type Init struct {
	place string
	tr    transport.Transport
}

func NewController(tr transport.Transport) Init {
	return Init{
		place: placeController,
		tr:    tr,
	}
}

func NewNode(tr transport.Transport) Init {
	return Init{
		place: placeNode,
		tr:    tr,
	}
}

func (n *Init) RunBlock(ctx context.Context, cfg InitConfig, devicePath string) (err error) {
	if cfg.On != n.place {
		return nil
	}

	fsnip := fsutils.New(n.tr)

	fsInfo, err := fsnip.GetBlkid(ctx, devicePath)
	if err != nil {
		return fmt.Errorf("GetBlkid: %w", err)
	}

	if fsInfo != nil {
		logs.FromCtx(ctx).MicroOp("will not format over existing fs", "device", devicePath, "fs-type", fsInfo.FsType)
		return nil
	}

	logs.FromCtx(ctx).MicroOp("fs info is not found, will format fs", "device", devicePath)
	if cfg.BlockFormat == "" {
		return fmt.Errorf("block format command is not set")
	}

	if cfg.BlockFormat == "" {
		return fmt.Errorf("missing block format command")
	}
	var cmd = []string{cfg.BlockFormat}
	cmd = append(cmd, devicePath)
	err = transport.CheckedRun(ctx, n.tr, cmd)
	if err != nil {
		return fmt.Errorf("format: %w", err)
	}
	logs.FromCtx(ctx).MicroOp("fs format success", "device", devicePath)

	return nil
}

func (n *Init) RunFsWithTmpMount(ctx context.Context, cfg InitConfig, devicePath string) error {
	if cfg.On != n.place {
		return nil
	}

	tmpMount := fmt.Sprintf("/tmp/%s", uuid.NewString())
	err := n.tr.Cmds().Mkdir(ctx, tmpMount, false)
	if err != nil {
		return fmt.Errorf("mkdir for temp mount: %w", err)
	}
	defer func() {
		cleanupErr := n.tr.Cmds().RmDir(ctx, tmpMount)
		if cleanupErr != nil {
			logs.FromCtx(ctx).Error("failed to cleanup temp dir", "path", tmpMount, "error", cleanupErr.Error())
		}
	}()

	fs := fsutils.New(n.tr)
	_, err = fs.EnsureMounted(ctx, devicePath, tmpMount, "", nil)
	if err != nil {
		return fmt.Errorf("tmp mount: %w", err)
	}
	defer func() {
		fs := fsutils.New(n.tr)
		deferErr := fs.Unmount(ctx, tmpMount)
		if err == nil {
			err = fmt.Errorf("cleanup tmp mount: %w", deferErr)
		} else {
			logs.FromCtx(ctx).Error("failed to cleanup tmp mount", "device", devicePath, "path", tmpMount, "error", deferErr.Error())
		}
	}()

	return n.RunFs(ctx, cfg, tmpMount)
}

func (n *Init) RunFs(ctx context.Context, cfg InitConfig, fsPath string) error {
	if cfg.On != n.place {
		return nil
	}

	var cmd []string
	var err error
	if cfg.OwnerUser != "-" && cfg.OwnerGroup != "-" {
		cmd = []string{"chown", fmt.Sprintf("%s:%s", cfg.OwnerUser, cfg.OwnerGroup), fsPath}
	} else if cfg.OwnerUser != "-" {
		cmd = []string{"chown", cfg.OwnerUser, fsPath}
	} else if cfg.OwnerGroup != "-" {
		cmd = []string{"chgrp", cfg.OwnerGroup, fsPath}
	}
	if len(cmd) != 0 {
		err = transport.CheckedRun(ctx, n.tr, cmd)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd[0], err)
		}
	}

	cmd = []string{"chmod", cfg.RootDirMode, fsPath}
	err = transport.CheckedRun(ctx, n.tr, cmd)
	if err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	return nil
}

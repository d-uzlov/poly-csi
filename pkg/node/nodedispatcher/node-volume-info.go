package nodedispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/blockcon"
	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/transport"
)

type NodeVolumeInfo struct {
	// PublishContext publishcontext.PublishContext
	// FsRootDirMode sets access more for mount root dir
	FsRootDirMode string `json:"fsRootDirMode"`
	// FsStagePath points to the root folder of the mount
	FsStagePath string `json:"fsMountPath"`
	// MultipathData        string            `json:"multipathData"`
	// MultipathJournalData string            `json:"multipathJournalData"`
	ConnectorData string            `json:"connectorData"`
	Preset        map[string]string `json:"preset"`

	Blocks map[string]Block

	FsType    string         `json:"fsType"`
	FsData    string         `json:"fsData"`
	fsHandler filecon.Handle `json:"-"`

	tr transport.Transport `json:"-"`
	// mp           multipath.Multipath `json:"-"`
	parentFolder string `json:"-"`
}

type Block struct {
	ConnectorId string          `json:"connectorId"`
	Data        string          `json:"data"`
	handler     blockcon.Handle `json:"-"`
}

type SavedParams struct {
}

const metadataFile = "/mount-info.json"

func newVolumeHandle(parentFolder string, tr transport.Transport, preset map[string]string) NodeVolumeInfo {
	return NodeVolumeInfo{
		parentFolder: parentFolder,
		Preset:       preset,
		tr:           tr,
		Blocks:       map[string]Block{},
	}
}

func ParseNodeVolumeInfo(ctx context.Context, parentFolder string, tr transport.Transport, devFolder string, bm map[string]blockcon.Manager, fm map[string]filecon.Manager) (*NodeVolumeInfo, error) {
	result, err := TryParseMountInfo(ctx, parentFolder, tr, devFolder, bm, fm)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("file is missing: %s", parentFolder+metadataFile)
	}
	return result, nil
}

func TryParseMountInfo(ctx context.Context, parentFolder string, tr transport.Transport, devFolder string, bm map[string]blockcon.Manager, fm map[string]filecon.Manager) (*NodeVolumeInfo, error) {
	content, err := os.ReadFile(parentFolder + metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result NodeVolumeInfo
	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		return nil, fmt.Errorf("parsing mount info from %s: %w", parentFolder+metadataFile, err)
	}

	for k, v := range result.Blocks {
		if v.Data == "" {
			continue
		}
		m, ok := bm[v.ConnectorId]
		if !ok {
			return nil, fmt.Errorf("block connector type is not found: '%s'", v.ConnectorId)
		}

		v.handler, err = m.ParseHandle(v.Data)
		if err != nil {
			return nil, fmt.Errorf("parse block data i=%s: %w", k, err)
		}
		err = v.handler.Validate()
		if err != nil {
			return nil, fmt.Errorf("validate block data i=%s: %w", k, err)
		}
		result.Blocks[k] = v
	}

	if result.FsType != "" {
		fsManager, ok := fm[result.FsType]
		if !ok {
			return nil, fmt.Errorf("filesystem type not found: '%s'", result.FsType)
		}
		result.fsHandler, err = fsManager.ParseHandle(result.FsData)
		if err != nil {
			return nil, fmt.Errorf("parse filesystem data: %w", err)
		}
		err = result.fsHandler.Validate()
		if err != nil {
			return nil, fmt.Errorf("validate fs data: %w", err)
		}
	}

	result.parentFolder = parentFolder
	result.tr = tr
	return &result, nil
}

func (nvi *NodeVolumeInfo) deleteFromDisk(ctx context.Context) error {
	err := os.Remove(nvi.parentFolder + metadataFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logs.FromCtx(ctx).Warn("underlying file does not exist", "path", nvi.parentFolder+metadataFile)
			return nil
		}
		return err
	}
	return nil
}

func (nvi *NodeVolumeInfo) writeToDisk() error {
	for k, v := range nvi.Blocks {
		serialized, err := v.handler.Serialize()
		if err != nil {
			return fmt.Errorf("serialize block handler i=%s: %w", k, err)
		}
		v.Data = serialized
		nvi.Blocks[k] = v
	}

	if nvi.fsHandler != nil {
		serialized, err := nvi.fsHandler.Serialize()
		if err != nil {
			return fmt.Errorf("serialize fs handler: %w", err)
		}
		nvi.FsData = serialized
	}

	encoded, err := json.MarshalIndent(nvi, "", "  ")
	if err != nil {
		return err
	}
	// todo do this via transport
	return os.WriteFile(nvi.parentFolder+metadataFile, encoded, 0660)
}

// TODO use
// func (nvi *NodeVolumeInfo) adjustPermissions(ctx context.Context, group string) error {
// 	fsAdjStart := time.Now()
// 	sr, err := nvi.tr.Cmds().Stat(ctx, nvi.FsMountPath)
// 	if err != nil {
// 		return fmt.Errorf("stat path: %w", err)
// 	}
// 	if !sr.FileExists {
// 		return fmt.Errorf("target path does not exist")
// 	}
// 	if group == "" || group == sr.Gid {
// 		return nil
// 	}
// 	err = nvi.tr.SudoCmds().Chown(ctx, nvi.FsMountPath, sr.Uid, group, true)
// 	if err != nil {
// 		return fmt.Errorf("chown: %w", err)
// 	}
// 	logs.FromCtx(ctx).MicroOp("chown: success", "duration", time.Since(fsAdjStart).String())
// 	return nil
// }

func (nvi *NodeVolumeInfo) cleanup(ctx context.Context) (err error) {
	defer func() {
		if err == nil {
			logs.FromCtx(ctx).MicroOp("cleanup success, deleting underlying file")
			err = nvi.deleteFromDisk(ctx)
			if err != nil {
				logs.FromCtx(ctx).Error("failed to delete from disk", "error", err)
			}
		} else {
			logs.FromCtx(ctx).Error("cleanup failure, saving partial result")
			deferErr := nvi.writeToDisk()
			if deferErr != nil {
				logs.FromCtx(ctx).Error("failed to save partial result", "error", deferErr)
			}
		}
	}()

	if nvi.fsHandler != nil && nvi.FsStagePath != "" {
		err = nvi.fsHandler.Unmount(ctx, nvi.FsStagePath)
		if err != nil {
			return fmt.Errorf("unmount stage path: %w", err)
		}
		nvi.fsHandler = nil
		nvi.FsData = ""
		nvi.FsType = ""
	}

	err = nvi.tr.Cmds().RmDir(ctx, nvi.FsStagePath)
	if err != nil {
		return fmt.Errorf("remove stage mount folder: %w", err)
	}
	nvi.FsStagePath = ""

	for k, v := range nvi.Blocks {
		if v.handler == nil {
			continue
		}
		err = v.handler.Disconnect(ctx)
		if err != nil {
			return fmt.Errorf("disconnect block i=%s: %w", k, err)
		}
		nvi.Blocks[k] = Block{}
	}

	return nil
}

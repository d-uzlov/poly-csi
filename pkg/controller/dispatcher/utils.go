package dispatcher

import (
	"context"
	"fmt"
	"time"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
	"meoe.io/poly-csi/pkg/transport"
)

type volumeManagementHandle struct {
	volumeName string
	srv        *parsedServer
	vi         *voldbwrapper.VolumeInfo
	wrapper    voldbwrapper.Wrapper
}

func (h *volumeManagementHandle) release() {
	if h.srv == nil {
		return
	}
	h.srv.mutex.RUnlock()
}

func (d *Dispatcher) getVolumeInfo(ctx context.Context, volumeId string) (*volumeManagementHandle, error) {
	srvId, volumeName, err := parseVolumeId(volumeId)
	if err != nil {
		return nil, fmt.Errorf("invalid volume id: %w", err)
	}
	srv, err := d.FindServerById(srvId)
	if err != nil {
		return nil, fmt.Errorf("server[id=%s]: not found", srvId)
	}

	srv.mutex.RLock()
	release := srv.mutex.RUnlock
	success := false
	defer func() {
		if success {
			return
		}
		release()
	}()

	err = srv.Check()
	if err != nil {
		return nil, fmt.Errorf("server[id=%s]: %w", srvId, err)
	}

	dbEntry, err := srv.dbInstance.FindEntry(ctx, volumeName)
	if err != nil {
		return nil, fmt.Errorf("server[id=%s].db: get entry: %w", srvId, err)
	}
	if dbEntry == nil {
		return &volumeManagementHandle{}, nil
	}
	wrapper := voldbwrapper.New(dbEntry)
	vi, err := wrapper.GetVolumeInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("server[id=%s].db.entry: get volume info %w", srvId, err)
	}

	success = true
	return &volumeManagementHandle{
		volumeName: volumeName,
		srv:        srv,
		vi:         vi,
		wrapper:    wrapper,
	}, nil
}

type snapshotManagementHandle struct {
	volumeName   string
	snapshotName string
	srv          *parsedServer
	si           *voldbwrapper.SnapshotInfo
	wrapper      voldbwrapper.Wrapper
}

func (h *snapshotManagementHandle) release() {
	if h.srv == nil {
		return
	}
	h.srv.mutex.RUnlock()
}

func (d *Dispatcher) getSnapshotHandle(ctx context.Context, snapshotId string) (*snapshotManagementHandle, error) {
	srvId, volumeName, snapshotName, err := parseSnapshotId(snapshotId)
	if err != nil {
		return nil, fmt.Errorf("invalid snapshot id: %w", err)
	}
	srv, err := d.FindServerById(srvId)
	if err != nil {
		return nil, fmt.Errorf("server[id=%s]: not found", srvId)
	}

	srv.mutex.RLock()
	release := srv.mutex.RUnlock
	success := false
	defer func() {
		if success {
			return
		}
		release()
	}()

	err = srv.Check()
	if err != nil {
		return nil, fmt.Errorf("server[id=%s]: %w", srvId, err)
	}

	dbEntry, err := srv.dbInstance.FindEntry(ctx, volumeName)
	if err != nil {
		return nil, fmt.Errorf("server[id=%s].db: get entry: %w", srvId, err)
	}
	if dbEntry == nil {
		return &snapshotManagementHandle{}, nil
	}
	wrapper := voldbwrapper.New(dbEntry)
	si, err := wrapper.GetSnapshot(ctx, snapshotName)
	if err != nil {
		return nil, fmt.Errorf("server[id=%s].db.entry: get snapshot %w", srvId, err)
	}

	success = true
	return &snapshotManagementHandle{
		volumeName:   volumeName,
		snapshotName: snapshotName,
		srv:          srv,
		si:           si,
		wrapper:      wrapper,
	}, nil
}

func checkFile(ctx context.Context, tr transport.Transport, expectedType string, path string) error {
	var stat transport.StatResult
	var err error

	switch expectedType {
	case storage.VolDevice:
		// let's give block devices a bit of time to appear
		err = transport.Retry(time.Millisecond*100, time.Second, func() error {
			stat, err = tr.Cmds().Stat(ctx, path)
			if err != nil {
				// error description is appended below
				return err
			}
			if !stat.FileExists {
				return fmt.Errorf("file does not exist: %s", path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("stat '%s': %w", path, err)
		}
		if !stat.IsBlock() {
			return fmt.Errorf("file '%s' is not a block device", path)
		}
	case storage.VolFolder:
		stat, err = tr.Cmds().Stat(ctx, path)
		if err != nil {
			return fmt.Errorf("stat '%s': %w", path, err)
		}
		if !stat.FileExists {
			return fmt.Errorf("file '%s' does not exist", path)
		}
		if !stat.IsDirectory() {
			return fmt.Errorf("file '%s' is not a directory", path)
		}
	default:
		return fmt.Errorf("unknown local volume type: %s", expectedType)
	}
	return nil
}

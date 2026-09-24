package dispatcher

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
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

// getExistingVolumeHandle takes volumeId, parses it,
// finds the appropriate server and reads volume info from the database
func (d *Dispatcher) getExistingVolumeHandle(ctx context.Context, volumeId string) (*volumeManagementHandle, error) {
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

package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/voldbwrapper"
)

// getExistingVolumeHandle takes volumeId, parses it,
// finds the appropriate server and reads volume info from the database
func (d *Dispatcher) getExistingVolumeHandle(ctx context.Context, volumeId string, conf *DispatcherConf) (VolumeDriver, error) {
	srvId, volumeName, err := parseVolumeId(volumeId)
	if err != nil {
		return nil, fmt.Errorf("invalid volume id: %w", err)
	}
	srv := conf.FindServerById(srvId)
	if srv == nil {
		return nil, fmt.Errorf("server[id=%s]: not found", srvId)
	}

	err = srv.Check()
	if err != nil {
		return nil, fmt.Errorf("server[id=%s]: %w", srvId, err)
	}

	dbEntry, err := srv.dbInstance.FindEntry(ctx, volumeName)
	if err != nil {
		return nil, fmt.Errorf("server[id=%s].db: get entry: %w", srvId, err)
	}
	if dbEntry == nil {
		return nil, nil
	}
	wrapper := voldbwrapper.New(dbEntry)
	vi, err := wrapper.GetVolumeInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("server[id=%s].db.entry: get volume info %w", srvId, err)
	}

	switch vi.VolumeType {
	case "block-fs":
		var s SerializedDispatchBlockFs
		err = json.Unmarshal(vi.Data, &s)
		if err != nil {
			return nil, fmt.Errorf("json.Unmarshal: %w", err)
		}

		result, err := s.parse(ctx, conf, srv)
		if err != nil {
			return nil, fmt.Errorf("parse SerializedDispatchBlockFs: %w", err)
		}

		return &result, nil
	case "native-fs":
		var s SerializedDispatchNativeFs
		err = json.Unmarshal(vi.Data, &s)
		if err != nil {
			return nil, fmt.Errorf("json.Unmarshal: %w", err)
		}

		result, err := s.parse(ctx, conf, srv)
		if err != nil {
			return nil, fmt.Errorf("parse SerializedDispatchNativeFs: %w", err)
		}

		return &result, nil
	default:
		return nil, fmt.Errorf("unknown volume type: '%s'", vi.VolumeType)
	}
}

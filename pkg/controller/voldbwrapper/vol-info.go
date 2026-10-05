package voldbwrapper

import (
	"context"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/controller/storage"
	"meoe.io/poly-csi/pkg/controller/volumecontext"
)

const mainVolumeModule = "volume.json"

type VolumeStorageInfo struct {
	// NeedsControllerFormat indicates if the controller is expected to format the volume.
	//
	// Always false if storage is file-based.
	//
	// False if CO requests raw block device. False if user requests node-side format. False if already formatted.
	NeedsControllerFormat bool

	// DataSource can be 'scratch' for volumes without a data source, or volume_id, or snapshot_id
	// When empty, indicates that volume is not initialized.
	DataSource string `json:"data_source"`
	// StorageType is the id of the storage manager
	StorageType string `json:"storage_type"`
	// StorageData is the value returned by storage.Volume.Serialize()
	StorageData string `json:"volume_data"`

	JournalStorageType string `json:"journal_storage_type"`
	JournalStorageData string `json:"journal_storage_data"`

	// LocalPathType is the saved value from storage.StorageManager.GetCapabilities().LocalPathType
	LocalPathType storage.LocalVolumeType `json:"local_type"`
	// Size is the saved value of storage.Volume.GetState().Size
	Size int64 `json:"size"`
}

type VolumeInfo struct {
	Storage VolumeStorageInfo `json:"storage"`

	ShareType string `json:"share_type"`
	ShareData string `json:"share_data"`

	// VolumeContext is the literal volume_context map, as sent to CO in CreateVolume
	VolumeContext volumecontext.VolumeContext `json:"volume_context"`
	ConnectorId   string                      `json:"connector_id"`
	AddressIds    []string                    `json:"address_ids"`
}

func (w Wrapper) SetVolumeInfo(ctx context.Context, info VolumeInfo) error {
	encoded, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return w.entry.SetModule(ctx, mainVolumeModule, encoded)
}

func (w Wrapper) GetVolumeInfo(ctx context.Context) (*VolumeInfo, error) {
	content, err := w.entry.GetModule(ctx, mainVolumeModule)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, nil
	}
	var result VolumeInfo
	err = json.Unmarshal(content, &result)
	if err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return &result, nil
}

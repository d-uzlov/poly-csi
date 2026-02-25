package nodedispatcher

import (
	"encoding/json"
	"fmt"
	"os"

	"meoe.io/poly-csi/pkg/node/mounts"
)

type NodeVolumeInfo struct {
	// MounterId holds id of the mounter, to be used on disconnect and other operations.
	MounterId string `json:"mounter_id"`
	// MounterPath is a path to a folder,
	// which was created to allow mounter to store data in filesystem
	// or create mounts.
	// TODO remove this? mounters should not manually store data in filesystem
	MounterPath string `json:"mounter_path"`
	// Folder path is a path to the main volume mount
	FolderPath string `json:"folder_path"`
	// SerializedMultipathData contains serialized data returned by multipath module
	SerializedMultipathData string `json:"multipath_data"`
	// ManagedOps is copied from mounter ConnectResult
	ManagedOps mounts.ManagedOperations `json:"managed_ops"`
	// ManagedOps contains serialized data returned by mounter connect
	SerializedMounterData string `json:"mounter_data"`
}

// mountSuffix defines name of the file inside staging folder containing NodeVolumeInfo
const mountSuffix = "/mount-info.json"

func SaveMountInfo(stagingPath string, info NodeVolumeInfo) error {
	encoded, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stagingPath+mountSuffix, encoded, 0660)
}

func ParseNodeVolumeInfo(stagingPath string) (*NodeVolumeInfo, error) {
	result, err := TryParseMountInfo(stagingPath)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("file is missing: %s", stagingPath+mountSuffix)
	}
	return result, nil
}

func TryParseMountInfo(stagingPath string) (*NodeVolumeInfo, error) {
	content, err := os.ReadFile(stagingPath + mountSuffix)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var info NodeVolumeInfo
	err = json.Unmarshal([]byte(content), &info)
	if err != nil {
		return nil, fmt.Errorf("parsing mount info from %s: %w", stagingPath+mountSuffix, err)
	}
	return &info, nil
}

func DeleteMountInfo(stagingPath string) error {
	return os.Remove(stagingPath + mountSuffix)
}

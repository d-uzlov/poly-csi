package mountinfo

import (
	"encoding/json"
	"fmt"
	"os"

	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/multipath"
)

type NodeVolumeInfo struct {
	MounterId         string                     `json:"mounter_id"`
	MounterPath       string                     `json:"mounter_path"`
	FolderPath        string                     `json:"folder_path"`
	DevicePath        string                     `json:"device_path"`
	DeviceIsMultipath bool                       `json:"device_is_multipath"`
	MultipathContext  multipath.MultipathContext `json:"multipath_context"`
	MounterContext    mounts.MounterContext      `json:"mounter_context"`
}

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

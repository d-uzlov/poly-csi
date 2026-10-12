package volumecontext

import "fmt"

const Prefix = "poly-csi/"

type VolumeContext struct {
	ServerName    string   `map:"server_name"`
	StorageType   string   `map:"storage_type"`
	ShareType     string   `map:"share_type"`
	Presets       []string `map:"preset"`
	FsRootDirMode string   `map:"fs_root_dir_mode"`
	FsGroupMethod string   `map:"fs_group_method"`
	Uuid          string   `map:"uuid"`
	FsType        string   `map:"fs_type"`
	FsData        string   `map:"fs_data"`
	JournalVolume string   `map:"journal_volume"`
}

func (v *VolumeContext) Validate() error {
	if v.ServerName == "" {
		return fmt.Errorf("missing server name")
	}
	if v.StorageType == "" {
		return fmt.Errorf("missing storage type")
	}
	if v.ShareType == "" {
		return fmt.Errorf("missing share type")
	}
	// if v.FsGroupMethod == "" {
	// 	return fmt.Errorf("missing fsGroup method")
	// }
	if v.Uuid == "" {
		return fmt.Errorf("missing uuid")
	}
	return nil
}

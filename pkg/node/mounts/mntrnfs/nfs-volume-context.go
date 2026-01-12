package mntrnfs

import (
	"fmt"
)

type NfsMountContext struct {
	Path    string
}

func InjectPublishContext(dst map[string]string, src NfsMountContext) error {
	dst["nfs.share_path"] = src.Path
	return nil
}

func ParsePublishContext(vc map[string]string) (NfsMountContext, error) {
	share, ok := vc["nfs.share_path"]
	if !ok {
		return NfsMountContext{}, fmt.Errorf("volume_context: 'nfs.share_path': field is empty")
	}
	return NfsMountContext{
		Path:    share,
	}, nil
}

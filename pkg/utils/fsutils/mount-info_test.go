package fsutils_test

import (
	"encoding/json"
	"testing"

	"meoe.io/poly-csi/pkg/utils/fsutils"
)

func serialize(value any) string {
	result, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(result)
}

func TestLineParsing(t *testing.T) {
	const line = "29 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw,discard,errors=remount-ro"
	mi, err := fsutils.ParseMountinfoLine(line)
	if err != nil {
		t.Error("ParseMountinfoLine failed", err)
		t.FailNow()
	}
	t.Log("parsed  ", serialize(mi))
	expected := fsutils.MountInfo{
		Id:       29,
		ParentId: 1,
		MajorMinor: "8:1",
		Root: "/",
		Source: "/dev/sda1",
		MountPoint: "/",
		FsType: "ext4",
	}
	if mi != expected {
		t.Error("expected", serialize(expected))
	}
}

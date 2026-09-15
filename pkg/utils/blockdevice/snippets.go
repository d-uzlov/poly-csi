package blockdevice

import (
	"context"
	"fmt"

	"meoe.io/poly-csi/pkg/transport"
)

type Snippets struct {
	tr         transport.Transport
	deviceName string
}

func NewSnippets(tr transport.Transport, deviceName string) Snippets {
	return Snippets{
		tr:         tr,
		deviceName: deviceName,
	}
}

type IoLimit struct {
	ReadBW    string `json:"readBW"`
	WriteBW   string `json:"writeBW"`
	ReadIops  string `json:"readIops"`
	WriteIops string `json:"writeIops"`
}

func (s *Snippets) SetCgroupsIoLimit(ctx context.Context, cgroupPath string, limit IoLimit) error {
	id, err := s.tr.Cmds().Read(ctx, fmt.Sprintf("/sys/class/block/%s/dev", s.deviceName), false)
	if err != nil {
		return err
	}

	line := fmt.Sprintf("%s rbps=%s wbps=%s riops=%s wiops=%s", string(id), limit.ReadBW, limit.WriteBW, limit.ReadIops, limit.WriteIops)
	return s.tr.Cmds().Write(ctx, fmt.Sprintf("%s/io.max", cgroupPath), []byte(line))
}

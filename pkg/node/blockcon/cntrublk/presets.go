package cntrublk

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/utils/ublk"
)

type UblkPresetRaw struct {
	Queues     int    `yaml:"queues"`
	QueueDepth int    `yaml:"queueDepth"`
	BufSize    string `yaml:"bufSize"`
}

func ParsePreset(src yaml.Node) (ublk.Preset, error) {
	raw := UblkPresetRaw{
		Queues:     1,
		QueueDepth: 128,
		BufSize:    "512KiB",
	}
	err := src.Decode(&raw)
	if err != nil {
		return ublk.Preset{}, fmt.Errorf("decode yaml: %w", err)
	}
	bufSize, err := humanize.ParseBytes(raw.BufSize)
	if err != nil {
		return ublk.Preset{}, fmt.Errorf("parse bufSize '%s' as bytes: %w", raw.BufSize, err)
	}
	return ublk.Preset{
		Queues:     raw.Queues,
		QueueDepth: raw.QueueDepth,
		BufSize:    int(bufSize),
	}, nil
}

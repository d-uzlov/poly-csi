package cntrublk

import (
	"testing"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/utils/ublk"
)

func TestPresetParsing(t *testing.T) {
	src := `
queues: 4
queueDepth: 32
bufSize: 128Ki

`

	var node yaml.Node
	err := yaml.Unmarshal([]byte(src), &node)
	if err != nil {
		panic(err)
	}

	var preset UblkPresetRaw
	err = node.Decode(&preset)
	if err != nil {
		panic(err)
	}

	expectedRaw := UblkPresetRaw{
		Queues:     4,
		QueueDepth: 32,
		BufSize:    "128Ki",
	}
	if preset != expectedRaw {
		t.Error("preset != expectedRaw")
		t.Error(preset)
		t.FailNow()
	}

	parsed, err := ParsePreset(node)
	if err != nil {
		panic(err)
	}

	expected := ublk.Preset{
		Queues:     4,
		QueueDepth: 32,
		BufSize:    131072,
	}
	if parsed != expected {
		t.Error("parsed != expected")
		t.Error(parsed)
		t.FailNow()
	}
}

package ublk

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

func (s *Snippets) List(ctx context.Context, devId int) ([]DeviceInfo, error) {
	cmd := append(ublkCmd, "list")
	if devId != -1 {
		cmd = append(cmd, "-n", strconv.Itoa(devId))
	}
	rr, err := s.tr.RunCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("ublk list: %w", err)
	}
	if rr.ExitCode != 0 {
		return nil, fmt.Errorf("ublk list: %w", transport.FormatRunError(rr))
	}
	result, err := ParseUblkList(string(rr.Stdout))
	if err != nil {
		return nil, fmt.Errorf("ublk list: %w", err)
	}
	return result, nil
}

// example output to parse
// dev id 3: nr_hw_queues 1 queue_depth 128 block size 512 dev_capacity 1116738
//         max rq size 524288 daemon pid 137 state LIVE
//         flags 0x2605a [ URING_CMD_COMP_IN_TASK RECOVERY RECOVERY_REISSUE CMD_IOCTL_ENCODE PER_IO_DAEMON BUF_REG_OFF_DAEMON SAFE_STOP_DEV ]
//         ublkc: 240:3 ublkb: 259:5 owner: 0:0
//         queue 0: tid 139 affinity(3 )
//         target {"dev_size":571769856,"export_name":"export1","host":"nas-rose-13.k8s.lan","name":"nbd","port":"10809","send_zc":1,"type":0}

var (
	line1r = regexp.MustCompile(`^dev id (\d+): nr_hw_queues (\d+) queue_depth (\d+) block size (\d+) dev_capacity (\d+)$`)
	line2r = regexp.MustCompile(`^max rq size (\d+) daemon pid (-?\d+) state (\w+)$`)
	line3r = regexp.MustCompile(`^flags (0x[0-9a-fA-F]+) \[(.*)\]$`)
	line4r = regexp.MustCompile(`^ublkc: (\d+:\d+) ublkb: (\d+:\d+) owner: (\d+:\d+)$`)
	line5r = regexp.MustCompile(`^queue (\d+): tid (\d+) affinity\((.*)\)$`)

	line1p = "dev id"
	line2p = "max rq"
	line3p = "flags"
	line4p = "ublkc"
	line5p = "queue"
)

type QueueInfo struct {
	QueueIndex string `json:"queueIndex"`
	Tid        string `json:"tid"`
	Affinity   string `json:"affinity"`
}

type DeviceInfo struct {
	// line 1
	DevId       string `json:"devId"`
	HwQueues    string `json:"hwQueues"`
	QueueDepth  string `json:"queueDepth"`
	BlockSize   string `json:"blockSize"`
	DevCapacity string `json:"devCapacity"`

	// line 2
	MaxRqSize string `json:"maxRqSize"`
	DaemonPid string `json:"daemonPid"`
	State     string `json:"state"`

	// line 3
	Flags Flags `json:"flags"`

	// line 4
	CharMajMin  string `json:"charMajMin"`
	BlockMajMin string `json:"blockMajMin"`
	OwnerUidGid string `json:"ownerUidGid"`

	Queues []QueueInfo `json:"queues"`

	Target     string `json:"target"`
	TargetType string `json:"targetType"`
}

func ParseUblkOutput(humanString string) (DeviceInfo, error) {
	list, err := ParseUblkList(humanString)
	if err != nil {
		return DeviceInfo{}, err
	}
	if len(list) == 0 {
		return DeviceInfo{}, fmt.Errorf("empty input")
	}
	if len(list) > 1 {
		return DeviceInfo{}, fmt.Errorf("input has more than 1 ublk dev info")
	}
	return list[0], nil
}

type ListParser struct {
	state *DeviceInfo
}

func (p *ListParser) GetState() *DeviceInfo {
	return p.state
}

func (p *ListParser) IsDeviceInfoStart(line string) bool {
	return strings.HasPrefix(line, line1p)
}

// PushLine tries to parse a new line from `ublk list` output.
// If line contains target data, it returns device info and resets internal state.
// If the device does not have target line, device info will be returned on next line, when a new device description is detected.
// The last value can be obtained via [ListParser.GetState].
func (p *ListParser) PushLine(line string) (*DeviceInfo, error) {
	line = strings.TrimSpace(line)
	var result *DeviceInfo
	current := p.state
	switch {
	case strings.HasPrefix(line, line1p):
		result = current
		p.state = &DeviceInfo{}
		current := p.state
		match := line1r.FindStringSubmatch(line)
		if len(match) != 6 {
			return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", line, line1r.String())
		}
		current.DevId = match[1]
		current.HwQueues = match[2]
		current.QueueDepth = match[3]
		current.BlockSize = match[4]
		current.DevCapacity = match[5]

	case strings.HasPrefix(line, line2p):
		match := line2r.FindStringSubmatch(line)
		if len(match) != 4 {
			return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", line, line2r.String())
		}
		current.MaxRqSize = match[1]
		current.DaemonPid = match[2]
		current.State = match[3]

	case strings.HasPrefix(line, line3p):
		match := line3r.FindStringSubmatch(line)
		if len(match) != 3 {
			return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", line, line3r.String())
		}
		flags, err := ParseFlagsString(match[1])
		if err != nil {
			return nil, err
		}
		current.Flags = flags

	case strings.HasPrefix(line, line4p):
		match := line4r.FindStringSubmatch(line)
		if len(match) != 4 {
			return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", line, line4r.String())
		}
		current.CharMajMin = match[1]
		current.BlockMajMin = match[2]
		current.OwnerUidGid = match[3]

	case strings.HasPrefix(line, line5p):
		match := line5r.FindStringSubmatch(line)
		if len(match) != 4 {
			return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", line, line5r.String())
		}
		current.Queues = append(current.Queues, QueueInfo{
			QueueIndex: match[1],
			Tid:        match[2],
			Affinity:   match[3],
		})

	case strings.HasPrefix(line, "target"):
		current.Target = strings.TrimSpace(strings.TrimPrefix(line, "target"))
		var raw map[string]any
		err := json.Unmarshal([]byte(current.Target), &raw)
		if err != nil {
			return nil, fmt.Errorf("parse target: %w", err)
		}
		current.TargetType, _ = raw["name"].(string)
		result = current
		p.state = nil

	default:
		return nil, fmt.Errorf("line '%s': unknown line prefix", line)
	}

	return result, nil
}

func ParseUblkList(humanString string) ([]DeviceInfo, error) {
	parser := ListParser{}
	var result []DeviceInfo
	for l := range strings.Lines(humanString) {
		next, err := parser.PushLine(l)
		if err != nil {
			return nil, err
		}
		if next != nil {
			result = append(result, *next)
		}
	}
	if parser.GetState() != nil {
		result = append(result, *parser.GetState())
	}
	return result, nil
}

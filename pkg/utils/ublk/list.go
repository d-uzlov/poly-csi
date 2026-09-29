package ublk

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"meoe.io/poly-csi/pkg/transport"
)

func (s *Snippets) List(ctx context.Context, devId string) ([]UblkDeviceInfo, error) {
	cmd := append(ublkCmd, "list")
	if devId != "" {
		cmd = append(cmd, "-n", devId)
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
)

type UblkQueueInfo struct {
	QueueIndex string
	Tid        string
	Affinity   string
}

type UblkDeviceInfo struct {
	// line 1
	DevId       string
	HwQueues    string
	QueueDepth  string
	BlockSize   string
	DevCapacity string

	// line 2
	MaxRqSize string
	DaemonPid string
	State     string

	// line 3
	Flags Flags

	// line 4
	CharMajMin  string
	BlockMajMin string
	OwnerUidGid string

	Queues []UblkQueueInfo
	Target string
}

func ParseUblkOutput(humanString string) (UblkDeviceInfo, error) {
	list, err := ParseUblkList(humanString)
	if err != nil {
		return UblkDeviceInfo{}, err
	}
	if len(list) == 0 {
		return UblkDeviceInfo{}, fmt.Errorf("empty input")
	}
	if len(list) > 1 {
		return UblkDeviceInfo{}, fmt.Errorf("input has more than 1 ublk dev info")
	}
	return list[0], nil
}

func ParseUblkList(humanString string) ([]UblkDeviceInfo, error) {
	var result []UblkDeviceInfo
	var current *UblkDeviceInfo
	for l := range strings.Lines(humanString) {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "dev id"):
			if current != nil {
				result = append(result, *current)
			}
			current = &UblkDeviceInfo{}
			match := line1r.FindStringSubmatch(l)
			if len(match) != 6 {
				return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", l, line1r.String())
			}
			current.DevId = match[1]
			current.HwQueues = match[2]
			current.QueueDepth = match[3]
			current.BlockSize = match[4]
			current.DevCapacity = match[5]
		case strings.HasPrefix(l, "max rq size"):
			match := line2r.FindStringSubmatch(l)
			if len(match) != 4 {
				return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", l, line2r.String())
			}
			current.MaxRqSize = match[1]
			current.DaemonPid = match[2]
			current.State = match[3]
		case strings.HasPrefix(l, "flags"):
			match := line3r.FindStringSubmatch(l)
			if len(match) != 3 {
				return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", l, line3r.String())
			}
			flags, err := ParseFlagsString(match[1])
			if err != nil {
				return nil, err
			}
			current.Flags = flags
		case strings.HasPrefix(l, "ublkc"):
			match := line4r.FindStringSubmatch(l)
			if len(match) != 4 {
				return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", l, line4r.String())
			}
			current.CharMajMin = match[1]
			current.BlockMajMin = match[2]
			current.OwnerUidGid = match[3]
		case strings.HasPrefix(l, "queue"):
			match := line5r.FindStringSubmatch(l)
			if len(match) != 4 {
				return nil, fmt.Errorf("format mismatch: '%s' is not '%s'", l, line5r.String())
			}
			current.Queues = append(current.Queues, UblkQueueInfo{
				QueueIndex: match[1],
				Tid:        match[2],
				Affinity:   match[3],
			})
		case strings.HasPrefix(l, "target"):
			current.Target = strings.TrimSpace(strings.TrimPrefix(l, "target"))
		default:
			return nil, fmt.Errorf("could not parse line: '%s'", l)
		}
	}
	if current != nil {
		result = append(result, *current)
	}
	return result, nil
}

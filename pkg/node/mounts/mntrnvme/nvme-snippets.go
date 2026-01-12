package mntrnvme

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/euank/go-kmsg-parser/kmsgparser"
	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/transport"
)

type nvmeNodeInfo struct {
	hostNqn string
	hostId  string
}

func nvmeGetNodeInfo(ctx context.Context) (*nvmeNodeInfo, error) {
	tr := transport.FromContext(ctx)
	hostNqn, err := tr.Cmds().Read(ctx, "/etc/nvme/hostnqn", true)
	if err != nil {
		return nil, fmt.Errorf("read /etc/nvme/hostnqn: %w", err)
	}
	hostId, err := tr.Cmds().Read(ctx, "/etc/nvme/hostid", true)
	if err != nil {
		return nil, fmt.Errorf("read /etc/nvme/hostid: %w", err)
	}
	return &nvmeNodeInfo{
		hostNqn: string(hostNqn),
		hostId:  string(hostId),
	}, nil
}

func nvmeConnect(ctx context.Context, nqn string, port string, nvmeTransport string, address string, ioQueues string) error {
	hostInfo, err := nvmeGetNodeInfo(ctx)
	if err != nil {
		return err
	}
	kMsgReader, err := kmsgparser.NewParser()
	if err != nil {
		return err
	}
	defer kMsgReader.Close()
	err = kMsgReader.SeekEnd()
	if err != nil {
		return err
	}
	messages := kMsgReader.Parse()
	defer func() {
		logger := slogctx.FromCtx(ctx)
		for {
			select {
			case msg := <-messages:
				if strings.HasPrefix(msg.Message, "nvme") {
					logger.Info("new kernel message", "msg", msg.Message)
				}
			default:
				return
			}
		}
	}()
	// setting hostnqn explicitly, because by default linux uses hostnqn detected on boot
	// if you change it, `nvme show-hostnqn` and /etc/nvme/hostnqn show new value,
	// but nvme connect will still use old value until reboot
	cmd := []string{"nvme", "connect", "--hostnqn", hostInfo.hostNqn, "--hostid", hostInfo.hostId, "--transport", nvmeTransport, "--traddr", address, "--nqn", nqn}
	if ioQueues != "" {
		cmd = append(cmd, "--nr-io-queues", ioQueues)
	}
	_, err = transport.RelaxedRun(ctx, cmd, "already connected")
	if err != nil {
		return fmt.Errorf("nvme connect '%s': %w", nqn, err)
	}
	return nil
}

func nvmeDisconnect(ctx context.Context, nqn string) error {
	cmd := []string{"nvme", "disconnect", "--nqn", nqn}
	// nvme disconnect returns success when nqn is already disconnected
	err := transport.CheckedRun(ctx, cmd)
	if err != nil {
		return fmt.Errorf("nvme disconnect '%s': %w", nqn, err)
	}
	return nil
}

func nvmeGetNqnInfo(ctx context.Context, nqn string) (*nvmeSubsystemInfo, error) {
	devices, err := nvmeListSubsys(ctx)
	if err != nil {
		return nil, fmt.Errorf("nvmeListSubsys: %w", err)
	}
	for _, d := range devices {
		if len(d.Subsystems) == 0 {
			continue
		}
		for _, s := range d.Subsystems {
			if s.Nqn != nqn {
				continue
			}
			return &s, nil
		}
	}
	return nil, fmt.Errorf("nqn not found: %s", nqn)
}

// func nvmeFindController(ctx context.Context, tr transport.Transport, nqn string, config nvmeshare.NvmeConnectConfig) (string, error) {
// 	subsys, err := nvmeGetNqnInfo(ctx, tr, nqn)
// 	if err != nil {
// 		return "", err
// 	}
// 	nvmeAddress := fmt.Sprintf("traddr=%s", config.Address)
// 	nvmePort := fmt.Sprintf("trsvcid=%d", config.Port)

// 	if len(subsys.Controllers) == 0 {
// 		return "", fmt.Errorf("subsystem does not have any controllers")
// 	}
// 	for _, con := range subsys.Controllers {
// 		if strings.Contains(con.Address, nvmeAddress) && strings.Contains(con.Address, nvmePort) {
// 			return con.Controller, nil
// 		}
// 	}
// 	return "", fmt.Errorf("nqn=%s: controller not found", nqn)
// }

func nvmeFindNamespace(ctx context.Context, nqn string, nsid int) (string, error) {
	subsys, err := nvmeGetNqnInfo(ctx, nqn)
	if err != nil {
		return "", fmt.Errorf("nvmeGetNqnInfo: %w", err)
	}

	if len(subsys.Namespaces) == 0 {
		return "", fmt.Errorf("subsystem does not have any namespaces")
	}
	for _, ns := range subsys.Namespaces {
		if ns.NSID == nsid {
			return ns.NameSpace, nil
		}
	}
	return "", fmt.Errorf("nqn=%s: namespace %d not found", nqn, nsid)
}

type nvmeSubsystemInfo struct {
	SubsysName  string `json:"Subsystem"`
	Nqn         string `json:"SubsystemNQN"`
	Controllers []*struct {
		Controller   string `json:"Controller"`
		Cntlid       string `json:"Cntlid"`
		SerialNumber string `json:"SerialNumber"`
		ModelNumber  string `json:"ModelNumber"`
		Firmware     string `json:"Firmware"`
		Transport    string `json:"Transport"`
		Address      string `json:"Address"`
		State        string `json:"State"`
		Paths        []struct {
			Path     string `json:"Path"`
			ANAState string `json:"ANAState"`
		} `json:"Paths"`
	} `json:"Controllers"`
	Namespaces []struct {
		NameSpace    string `json:"NameSpace"`
		Generic      string `json:"Generic"`
		NSID         int    `json:"NSID"`
		PhysicalSize int    `json:"PhysicalSize"`
		UsedBytes    int    `json:"UsedBytes"`
		SectorSize   int    `json:"SectorSize"`
	} `json:"Namespaces"`
}

type nvmeInfo struct {
	HostNqn    string              `json:"HostNQN"`
	HostId     string              `json:"HostID"`
	Subsystems []nvmeSubsystemInfo `json:"Subsystems"`
}

func nvmeListSubsys(ctx context.Context) ([]nvmeInfo, error) {
	cmd := []string{"nvme", "list", "--verbose", "--output-format=json"}
	stdout, err := transport.CheckedRunStdout(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("nvme list: %w", err)
	}
	var result struct {
		Devices []nvmeInfo `yaml:"Devices"`
	}
	err = json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		slogctx.FromCtx(ctx).Error("failed to parse 'nvme list' output as json", "raw", stdout)
		return nil, fmt.Errorf("parse 'nvme list' output as json (see logs for full stdout reference): %w", err)
	}
	return result.Devices, nil
}

func nvmeRescan(ctx context.Context, nqn string) error {
	nqnInfo, err := nvmeGetNqnInfo(ctx, nqn)
	if err != nil {
		return fmt.Errorf("nvmeGetNqnInfo: %w", err)
	}
	if len(nqnInfo.Controllers) == 0 {
		return fmt.Errorf("nqn does not have any controllers connected: %s", nqn)
	}
	anySuccess := false
	err = nil
	for _, c := range nqnInfo.Controllers {
		devPath := "/dev/" + c.Controller
		cmd := []string{"nvme", "ns-rescan", "--verbose", devPath}
		err = transport.CheckedRun(ctx, cmd)
		if err != nil {
			err = fmt.Errorf("nvme ns-rescan '%s': %w", devPath, err)
			slogctx.FromCtx(ctx).Error("nvme ns-rescan failed", "error", err)
			continue
		}
		anySuccess = true
	}
	if !anySuccess {
		return fmt.Errorf("could not rescan any of nqn controllers: %s", nqn)
	}
	return nil
}

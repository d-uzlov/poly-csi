package cntrnvme

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/euank/go-kmsg-parser/kmsgparser"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
)

type NvmeSnippets struct {
	tr             transport.Transport
	cmd            []string
	devFolder      string
	parseKernelLog bool
}

func newNvmeSnippets(tr transport.Transport, cmd []string, devFolder string, parseKernelLog bool) NvmeSnippets {
	return NvmeSnippets{
		tr:             tr,
		cmd:            cmd,
		devFolder:      devFolder,
		parseKernelLog: parseKernelLog,
	}
}

type nvmeNodeInfo struct {
	hostNqn string
	hostId  string
}

func nvmeGetNodeInfo(ctx context.Context, tr transport.Transport) (*nvmeNodeInfo, error) {
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

func initKernelMessageParser(prefix string) (getMessages func() []string, err error) {
	kMsgReader, err := kmsgparser.NewParser()
	if err != nil {
		return nil, err
	}
	err = kMsgReader.SeekEnd()
	if err != nil {
		return nil, err
	}
	messages := kMsgReader.Parse()
	return func() []string {
		defer kMsgReader.Close()
		var result []string
		for {
			select {
			case msg := <-messages:
				if strings.HasPrefix(msg.Message, prefix) {
					result = append(result, msg.Message)
				}
			default:
				return result
			}
		}
	}, nil
}

func (snip *NvmeSnippets) Connect(ctx context.Context, targetNqn string, hostNqn string, hostId string, port string, nvmeTransport string, address string, ioQueues string, hostKey string, controllerKey string, keepAliveTimeout string, controllerLossTimeout string, reconnectDelay string) error {
	if snip.parseKernelLog {
		getMessages, err := initKernelMessageParser("nvme")
		if err != nil {
			return fmt.Errorf("failed to init kmsgparser: %w", err)
		}
		defer func() {
			// TODO this can repeat messages when requests are parallel
			logger := logs.FromCtx(ctx)
			for _, v := range getMessages() {
				logger.Info("new kernel message", "msg", v)
			}
		}()
	}

	args := map[string]string{
		"--hostnqn":            hostNqn,
		"--hostid":             hostId,
		"--transport":          nvmeTransport,
		"--traddr":             address,
		"--trsvcid":            port,
		"--nqn":                targetNqn,
		"--keep-alive-tmo":     keepAliveTimeout,
		"--ctrl-loss-tmo":      controllerLossTimeout,
		"--reconnect-delay":    reconnectDelay,
		"--nr-io-queues":       ioQueues,
		"--dhchap-secret":      hostKey,
		"--dhchap-ctrl-secret": controllerKey,
	}
	cmd := append(snip.cmd,
		"connect",
	)
	for k, v := range args {
		if v != "" {
			cmd = append(cmd, k+"="+v)
		}
	}
	_, err := transport.RelaxedRun(ctx, snip.tr, cmd, "already connected")
	if err != nil {
		return fmt.Errorf("nvme connect '%s': %w", targetNqn, err)
	}
	return nil
}

func (snip *NvmeSnippets) Disconnect(ctx context.Context, nqn string) error {
	cmd := append(snip.cmd, "disconnect", "--nqn", nqn)
	// nvme disconnect returns success when nqn is already disconnected
	err := transport.CheckedRun(ctx, snip.tr, cmd)
	if err != nil {
		return fmt.Errorf("nvme disconnect '%s': %w", nqn, err)
	}
	return nil
}

func (snip *NvmeSnippets) GetSubsysInfo(ctx context.Context, nqn string) (*nvmeSubsystemInfo, error) {
	devices, err := snip.ListSubsys(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListSubsys: %w", err)
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
	return nil, nil
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

func (snip *NvmeSnippets) FindNamespaceDevice(ctx context.Context, nqn string, nsid int) (string, error) {
	subsys, err := snip.GetSubsysInfo(ctx, nqn)
	if err != nil {
		return "", fmt.Errorf("nvmeGetNqnInfo: %w", err)
	}
	if subsys == nil {
		return "", fmt.Errorf("subsys not found: %s", nqn)
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

func (snip *NvmeSnippets) ListSubsys(ctx context.Context) ([]nvmeInfo, error) {
	cmd := append(snip.cmd, "list", "--verbose", "--output-format=json")
	stdout, err := transport.CheckedRunStdout(ctx, snip.tr, cmd)
	if err != nil {
		return nil, fmt.Errorf("nvme list: %w", err)
	}
	var result struct {
		Devices []nvmeInfo `yaml:"Devices"`
	}
	err = json.Unmarshal([]byte(stdout), &result)
	if err != nil {
		logs.FromCtx(ctx).Error("failed to parse 'nvme list' output as json", "raw", stdout)
		return nil, fmt.Errorf("parse 'nvme list' output as json (see logs for full stdout reference): %w", err)
	}
	return result.Devices, nil
}

func (snip *NvmeSnippets) Rescan(ctx context.Context, nqn string) error {
	subsys, err := snip.GetSubsysInfo(ctx, nqn)
	if err != nil {
		return fmt.Errorf("GetSubsysInfo: %w", err)
	}
	if subsys == nil {
		return fmt.Errorf("subsys not found: %s", nqn)
	}
	if len(subsys.Controllers) == 0 {
		return fmt.Errorf("nqn does not have any controllers connected: %s", nqn)
	}
	anySuccess := false
	err = nil
	for _, c := range subsys.Controllers {
		devPath := snip.devFolder + "/" + c.Controller
		cmd := append(snip.cmd, "ns-rescan", "--verbose", devPath)
		err = transport.CheckedRun(ctx, snip.tr, cmd)
		if err != nil {
			err = fmt.Errorf("nvme ns-rescan '%s': %w", devPath, err)
			logs.FromCtx(ctx).Error("nvme ns-rescan failed", "error", err.Error())
			continue
		}
		anySuccess = true
	}
	if !anySuccess {
		return fmt.Errorf("could not rescan any of nqn controllers: %s", nqn)
	}
	return nil
}

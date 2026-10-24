package nodeconfig

import (
	"crypto/hpke"
	"net"
	"strconv"

	"github.com/dustin/go-humanize"
	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/filecon"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/blockdevice"
)

type ParsedEncrypt struct {
	NodeKey hpke.PrivateKey
	Conf    NodeEncryptConfig
}

type ParsedNetworks struct {
	Include []net.IPNet
	Exclude []net.IPNet
}

type ParsedConfig struct {
	Encrypt                 ParsedEncrypt
	NodeInfoDb              NodeInfoDbConfig
	Multipath               mpfactory.ParsedMultipath
	BlockConnectors         map[string]blockapi.Manager
	FsConnectors            map[string]filecon.Manager
	BlockCgroupPresets      map[string]blockdevice.IoLimit
	Networks                []ParsedNetworks
	DevFolder               string
	PodCgroupRoot           string
	ExecConfig              localexec.Config
	Logs                    Logs
}

func ParseCgroupPreset(src BlockPreset) (blockdevice.IoLimit, error) {
	result := blockdevice.IoLimit{
		ReadBW:    "max",
		WriteBW:   "max",
		ReadIops:  "max",
		WriteIops: "max",
	}
	if src.ReadBw != "" {
		value, err := humanize.ParseBytes(src.ReadBw)
		if err != nil {
			return blockdevice.IoLimit{}, err
		}
		result.ReadBW = strconv.Itoa(int(value))
	}
	if src.WriteBw != "" {
		value, err := humanize.ParseBytes(src.WriteBw)
		if err != nil {
			return blockdevice.IoLimit{}, err
		}
		result.WriteBW = strconv.Itoa(int(value))
	}
	if src.ReadIops != "" {
		value, _, err := humanize.ParseSI(src.ReadIops)
		if err != nil {
			return blockdevice.IoLimit{}, err
		}
		result.ReadIops = strconv.Itoa(int(value))
	}
	if src.WriteIops != "" {
		value, _, err := humanize.ParseSI(src.WriteIops)
		if err != nil {
			return blockdevice.IoLimit{}, err
		}
		result.WriteIops = strconv.Itoa(int(value))
	}
	return result, nil
}

func (c *ParsedConfig) GetRpcLogConfig(rpcPath string) logs.Config {
	result := logs.Config{
		LogLevel:     "info",
		EchoCommands: false,
		ExtraFields: map[string]string{
			"scope":  "method",
			"method": rpcPath,
		},
	}
	result = result.Combine(c.Logs.Default)
	config, ok := c.Logs.Rpc[rpcPath]
	if ok {
		result = result.Combine(config)
	}
	return result
}

func (c *ParsedConfig) GetNodeInfoLogConfig() logs.Config {
	result := logs.Config{
		LogLevel:     "info",
		EchoCommands: false,
		ExtraFields: map[string]string{
			"scope": "node-info",
		},
	}
	result = result.Combine(c.Logs.Default)
	result = result.Combine(c.Logs.NodeInfo)
	return result
}

func (c *ParsedConfig) GetMultipath(tr transport.Transport) multipath.Multipath {
	if !c.Multipath.Valid {
		return nil
	}

	if c.Multipath.Inflate == nil {
		return nil
	}

	return c.Multipath.Inflate(tr)
}

package nodeconfig

import (
	"net"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/blockdevice"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type ParsedEncrypt struct {
	NodeKey encrypt.RsaPrivateKey
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
	Connectors              []*connectorfactory.ParsedConnector
	Presets                 map[string]map[string]string
	Networks                []ParsedNetworks
	DevFolder               string
	PodCgroupRoot           string
	ExecConfig              localexec.Config
	Logs                    Logs
	DisableCleanupOnFailure bool
}

func ParseCgroupIoLimits(preset map[string]string) (limit blockdevice.IoLimit, foundAny bool, err error) {
	result := blockdevice.IoLimit{
		ReadBW:    "max",
		WriteBW:   "max",
		ReadIops:  "max",
		WriteIops: "max",
	}
	const prefix = "cgroup_io_max."
	for k, v := range preset {
		type_, ok := strings.CutPrefix(k, prefix)
		if !ok {
			continue
		}
		switch type_ {
		case "readBw":
			value, err := humanize.ParseBytes(v)
			if err != nil {
				return blockdevice.IoLimit{}, false, err
			}
			foundAny = true
			result.ReadBW = strconv.Itoa(int(value))
		case "writeBw":
			value, err := humanize.ParseBytes(v)
			if err != nil {
				return blockdevice.IoLimit{}, false,err
			}
			foundAny = true
			result.WriteBW = strconv.Itoa(int(value))
		case "readIops":
			value, _, err := humanize.ParseSI(v)
			if err != nil {
				return blockdevice.IoLimit{}, false,err
			}
			foundAny = true
			result.ReadIops = strconv.Itoa(int(value))
		case "writeIops":
			value, _, err := humanize.ParseSI(v)
			if err != nil {
				return blockdevice.IoLimit{}, false,err
			}
			foundAny = true
			result.WriteIops = strconv.Itoa(int(value))
		}
	}
	return result, foundAny, nil
}

func (c *ParsedConfig) FindConnectorById(id string) connectorfactory.InflateFunc {
	for _, v := range c.Connectors {
		if v.Conf.Id == id {
			return v.Inflate
		}
	}

	return nil
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

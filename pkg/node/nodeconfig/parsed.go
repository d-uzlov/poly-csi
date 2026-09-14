package nodeconfig

import (
	"net"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
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
	DevFolder               string
	Multipath               mpfactory.ParsedMultipath
	Connectors              []*connectorfactory.ParsedConnector
	Presets                 map[string]map[string]string
	Networks                []ParsedNetworks
	ExecConfig              localexec.Config
	Logs                    Logs
	DisableCleanupOnFailure bool
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

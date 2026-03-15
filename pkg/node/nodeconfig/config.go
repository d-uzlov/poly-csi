package nodeconfig

import (
	"time"

	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/transport/localexec"
)

type NodeConfig struct {
	Encrypt    NodeEncryptConfig               `yaml:"encrypt" json:"encrypt"`
	NodeInfoDb NodeInfoDbConfig                `yaml:"nodeInfoDb" json:"nodeInfoDb"`
	Multipath  mpfactory.MultipathConfig       `yaml:"multipath" json:"multipath"`
	Connectors []*connectorfactory.MountConfig `yaml:"connectors" json:"connectors"`
	Networks   []NodeNetworkConfig             `yaml:"networks" json:"networks"`
	DevFolder  string                          `yaml:"devFolder" json:"devFolder"`
	ExecConfig localexec.Config                `yaml:"execConfig" json:"execConfig"`
}

type NodeInfoDbConfig struct {
	Address  string        `yaml:"address" json:"address"`
	Interval time.Duration `yaml:"interval" json:"interval"`
}

type NodeEncryptConfig struct {
	NodeKey             KeyInfoConfig `yaml:"nodeKey" json:"nodeKey"`
	ControllerPublicKey KeyInfoConfig `yaml:"controllerPublicKey" json:"controllerPublicKey"`
}

type NodeNetworkConfig struct {
	Include []string `yaml:"includeCidr" json:"includeCidr"`
	Exclude []string `yaml:"excludeCidr" json:"excludeCidr"`
}

type KeyInfoConfig struct {
	Path string `yaml:"path" json:"path"`
}

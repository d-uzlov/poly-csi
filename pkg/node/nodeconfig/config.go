package nodeconfig

import (
	"time"

	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
)

type NodeConfig struct {
	Encrypt    NodeEncryptConfig               `yaml:"encrypt"`
	NodeInfoDb NodeInfoDbConfig                `yaml:"nodeInfoDb"`
	Multipath  mpfactory.MultipathConfig       `yaml:"multipath"`
	Connectors []*connectorfactory.MountConfig `yaml:"connectors"`
	Networks   []NodeNetworkConfig             `yaml:"networks"`
	DevFolder  string                          `yaml:"devFolder"`
}

type NodeInfoDbConfig struct {
	Address  string        `yaml:"address"`
	Interval time.Duration `yaml:"interval"`
}

type NodeEncryptConfig struct {
	NodeKey             KeyInfoConfig `yaml:"nodeKey"`
	ControllerPublicKey KeyInfoConfig `yaml:"controllerPublicKey"`
}

type NodeNetworkConfig struct {
	Include []string `yaml:"includeCidr"`
	Exclude []string `yaml:"excludeCidr"`
}

type KeyInfoConfig struct {
	Path string `yaml:"path"`
}

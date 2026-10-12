package nodeconfig

import (
	"time"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/transport/localexec"
)

type NodeConfig struct {
	Encrypt                 NodeEncryptConfig         `yaml:"encrypt" json:"encrypt"`
	NodeInfoDb              NodeInfoDbConfig          `yaml:"nodeInfoDb" json:"nodeInfoDb"`
	Multipath               mpfactory.MultipathConfig `yaml:"multipath" json:"multipath"`
	BlockConnectors         []BlockSpec               `yaml:"blockConnectors" json:"blockConnectors"`
	Filesystems             []FsSpec                  `yaml:"filesystems" json:"filesystems"`
	Presets                 []Preset                  `yaml:"presets"`
	Networks                []NodeNetworkConfig       `yaml:"networks" json:"networks"`
	DevFolder               string                    `yaml:"devFolder" json:"devFolder"`
	PodCgroupRoot           string                    `yaml:"podCgroupRoot" json:"podCgroupRoot"`
	ExecConfig              localexec.Config          `yaml:"execConfig" json:"execConfig"`
	Logs                    Logs                      `yaml:"logs" json:"logs"`
	DisableCleanupOnFailure bool                      `yaml:"disableCleanupOnFailure" json:"disableCleanupOnFailure"`
}

type Preset struct {
	Name   string            `yaml:"name"`
	Params map[string]string `yaml:"params"`
}

type Logs struct {
	Default  logs.Config            `yaml:"default" json:"default"`
	Rpc      map[string]logs.Config `yaml:"rpc" json:"rpc"`
	NodeInfo logs.Config            `yaml:"nodeInfo" json:"nodeInfo"`
}

type NodeInfoDbConfig struct {
	Address  string        `yaml:"address" json:"address"`
	Interval time.Duration `yaml:"interval" json:"interval"`
}

type NodeEncryptConfig struct {
	NodeKey KeyInfoConfig `yaml:"nodeKey" json:"nodeKey"`
}

type NodeNetworkConfig struct {
	Include []string `yaml:"includeCidr" json:"includeCidr"`
	Exclude []string `yaml:"excludeCidr" json:"excludeCidr"`
}

type KeyInfoConfig struct {
	Path string `yaml:"path" json:"path"`
}

type BlockSpec struct {
	Id     string    `yaml:"id"`
	Type   string    `yaml:"type"`
	Config yaml.Node `yaml:"config"`
}

type FsSpec struct {
	Id     string    `yaml:"id"`
	Type   string    `yaml:"type"`
	Config yaml.Node `yaml:"config"`
}

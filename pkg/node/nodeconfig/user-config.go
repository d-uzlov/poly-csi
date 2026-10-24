package nodeconfig

import (
	"time"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/multipath/mpfactory"
	"meoe.io/poly-csi/pkg/transport/localexec"
)

type NodeConfig struct {
	Encrypt            NodeEncryptConfig         `yaml:"encrypt" json:"encrypt"`
	NodeInfoDb         NodeInfoDbConfig          `yaml:"nodeInfoDb" json:"nodeInfoDb"`
	Multipath          mpfactory.MultipathConfig `yaml:"multipath" json:"multipath"`
	BlockConnectors    []BlockSpec               `yaml:"blockConnectors" json:"blockConnectors"`
	Filesystems        []FsSpec                  `yaml:"filesystems" json:"filesystems"`
	BlockCgroupPresets map[string]BlockPreset    `yaml:"blockCgroupPresets"`
	Networks           []NodeNetworkConfig       `yaml:"networks" json:"networks"`
	DevFolder          string                    `yaml:"devFolder" json:"devFolder"`
	PodCgroupRoot      string                    `yaml:"podCgroupRoot" json:"podCgroupRoot"`
	ExecConfig         localexec.Config          `yaml:"execConfig" json:"execConfig"`
	Logs               Logs                      `yaml:"logs" json:"logs"`
}

type BlockPreset struct {
	ReadBw    string `yaml:"readBw"`
	WriteBw   string `yaml:"writeBw"`
	ReadIops  string `yaml:"readIops"`
	WriteIops string `yaml:"writeIops"`
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
	Id      string               `yaml:"id"`
	Type    string               `yaml:"type"`
	Config  yaml.Node            `yaml:"config"`
	Presets map[string]yaml.Node `yaml:"presets"`
}

type FsSpec struct {
	Id     string    `yaml:"id"`
	Type   string    `yaml:"type"`
	Config yaml.Node `yaml:"config"`
}

package connectorpluginconfig

import (
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
)

type Config struct {
	Socket     string                            `yaml:"socket"`
	Connectors []*connectorfactory.ConnectorSpec `yaml:"connector"`
	DevFolder  string                            `yaml:"devFolder"`
	Logs       logs.Config                       `yaml:"logs"`
	Supervisor SupervisorConfig                  `yaml:"supervisor"`
}

type SupervisorConfig struct {
	ConfigDir string `yaml:"configDir"`
	MinId     int    `yaml:"minId"`
	MaxId     int    `yaml:"maxId"`
}

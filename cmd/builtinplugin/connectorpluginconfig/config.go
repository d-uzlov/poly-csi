package connectorpluginconfig

import (
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
)

type Config struct {
	Socket    string                          `yaml:"socket"`
	Connector *connectorfactory.ConnectorSpec `yaml:"connector"`
	DevFolder string                          `yaml:"devFolder"`
	Logs      logs.Config                     `yaml:"logs"`
}

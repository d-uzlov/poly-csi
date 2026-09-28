package connectorfactory

import (
	"go.yaml.in/yaml/v3"
)

type ConnectorSpec struct {
	Id     string    `yaml:"id"`
	Type   string    `yaml:"type"`
	Config yaml.Node `yaml:"config"`
}

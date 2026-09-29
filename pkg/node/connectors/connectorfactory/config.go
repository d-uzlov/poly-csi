package connectorfactory

import (
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type ConnectorSpec struct {
	Id     string    `yaml:"id"`
	Type   string    `yaml:"type"`
	Config yaml.Node `yaml:"config"`
}

func ParseList(logger *logs.Instance, tr transport.Transport, devFolder string, connectorList []*ConnectorSpec, lookup func(mtype string) (connectors.InflateFunc, error)) map[string]connectors.Manager {
	cons := map[string]connectors.Manager{}
	logger.Info(fmt.Sprintf("connectors in config: %d", len(connectorList)))
	for i, v := range connectorList {
		if v.Id == "" {
			logger.Error(fmt.Sprintf("connector[i=%d]: missing id", i))
			continue
		}
		if _, ok := cons[v.Id]; ok {
			logger.Error(fmt.Sprintf("connector[i=%d/id=%s]: duplicate id", i, v.Id))
			continue
		}

		if v.Type == "" {
			logger.Error(fmt.Sprintf("connector[i=%d/id=%s]: missing type", i, v.Id))
			continue
		}
		inflate, err := lookup(v.Type)
		if err != nil {
			logger.Error(fmt.Sprintf("connector[i=%d/id=%s]: GetByType failed", i, v.Id), "error", err)
			continue
		}
		if inflate == nil {
			logger.Error(fmt.Sprintf("connector[i=%d/id=%s]: GetByType returned nil", i, v.Id))
			continue
		}
		con, err := inflate(connectors.CommonConfig{
			Tr:        tr,
			DevFolder: devFolder,
			Cfg:       v.Config,
		})
		if err != nil {
			logger.Error(fmt.Sprintf("connector[i=%d/id=%s]: inflate failed", i, v.Id), "error", err)
		}

		cons[v.Id] = con
	}
	return cons
}

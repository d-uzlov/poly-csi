package voldbfactory

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/config"
	"meoe.io/poly-csi/pkg/controller/voldb"
	"meoe.io/poly-csi/pkg/controller/voldb/jsonfiledb"
)

const (
	TypeJsonFile = "json-file"
)

type Inflate func(ctx context.Context) (voldb.VolumeDatabase, error)

func VolumeDatabaseFromType(transportType string, byteConfig []byte) (Inflate, config.ValidatableConfig, error) {
	switch transportType {
	case TypeJsonFile:
		var cfg jsonfiledb.JsonFileInfoManagerConfig
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, nil, fmt.Errorf("parse %s config: %w", transportType, err)
		}
		return func(ctx context.Context) (voldb.VolumeDatabase, error) {
			return jsonfiledb.NewJsonFileVolumeDatabase(cfg), nil
		}, cfg, nil
	}
	return nil, nil, fmt.Errorf("unknown database type: %s", transportType)
}

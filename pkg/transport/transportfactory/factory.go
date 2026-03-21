package transportfactory

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/config"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
	ssh_wrapper "meoe.io/poly-csi/pkg/transport/ssh"
)

const (
	TransportTypeLocal = "local"
	TransportTypeSsh   = "ssh"
)

type Inflate func(ctx context.Context) (transport.Transport, error)

func TransportFromType(transportType string, byteConfig []byte) (Inflate, config.ValidatableConfig, error) {
	switch transportType {
	case TransportTypeLocal:
		var cfg localexec.Config
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, nil, fmt.Errorf("parse %s config: %w", transportType, err)
		}
		return func(ctx context.Context) (transport.Transport, error) {
			return localexec.New(cfg), nil
		}, nil, nil
	case TransportTypeSsh:
		var cfg ssh_wrapper.SshConfig
		if err := yaml.Unmarshal(byteConfig, &cfg); err != nil {
			return nil, nil, fmt.Errorf("parse %s config: %w", transportType, err)
		}
		return func(ctx context.Context) (transport.Transport, error) {
			return ssh_wrapper.New(ctx, cfg)
		}, &cfg, nil
	case "":
		return nil, nil, fmt.Errorf("empty transport type")
	default:
		return nil, nil, fmt.Errorf("unknown transport type: %s", transportType)
	}
}

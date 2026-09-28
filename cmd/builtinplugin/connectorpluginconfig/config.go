package connectorpluginconfig

import (
	"bytes"
	"context"
	"fmt"

	slogctx "github.com/veqryn/slog-context"
	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors/cntrhttpjsonplugin"
	"meoe.io/poly-csi/pkg/node/connectors/connectorfactory"
	"meoe.io/poly-csi/pkg/transport"
)

type Config struct {
	Socket    string                          `yaml:"socket"`
	Connector *connectorfactory.ConnectorSpec `yaml:"connector"`
	DevFolder string                          `yaml:"devFolder"`
	Logs      logs.Config                     `yaml:"logs"`
}

type parsedConfig struct {
	Config    Config
	Srv       *cntrhttpjsonplugin.HttpServer
	Connector *connectorfactory.ParsedConnector
}

func ParseConfig(ctx context.Context, tr transport.Transport, raw []byte) (*parsedConfig, error) {
	logger := slogctx.FromCtx(ctx)

	dec := yaml.NewDecoder(bytes.NewBuffer(raw))
	dec.KnownFields(true)

	var config Config
	err := dec.Decode(&config)
	// err := yaml.UnmarshalStrict(raw, &result)
	if err != nil {
		logger.Error("config: initial parsing failed", "error", err.Error())
		return nil, err
	}

	pm := connectorfactory.ParseConnector(ctx, config.Connector)
	if pm == nil {
		return nil, fmt.Errorf("connectorfactory.ParseMount failed")
	}

	if pm.Valid {
		logger.Info(fmt.Sprintf("successfully parsed connector '%s'", pm.Conf.Id))
	} else {
		logger.Error(fmt.Sprintf("parsing connector '%s' failed", pm.Conf.Id))
	}

	dispatcher := cntrhttpjsonplugin.NewDispatcherServer(tr, config.DevFolder, pm.Inflate, config.Connector.Config)

	srv := cntrhttpjsonplugin.NewHttpServer(ctx, dispatcher, config.Logs)

	return &parsedConfig{
		Config:    config,
		Srv:       srv,
		Connector: pm,
	}, nil
}

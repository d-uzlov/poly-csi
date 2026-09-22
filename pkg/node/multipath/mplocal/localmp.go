package mplocal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/multipath"
	"meoe.io/poly-csi/pkg/transport"
)

type localMp struct {
	tr        transport.Transport
	config    Config
	devFolder string
}

type Config struct {
	MapperFolder  string   `yaml:"mapperFolder"`
	MultipathdCmd []string `yaml:"multipathdCmd"`
}

func (c *Config) Validate() error {
	if c.MapperFolder == "" {
		return fmt.Errorf("missing parameter mapperFolder")
	}
	if len(c.MultipathdCmd) == 0 {
		return fmt.Errorf("missing parameter multipathdCmd")
	}
	return nil
}

func New(tr transport.Transport, config Config, devFolder string) multipath.Multipath {
	return &localMp{
		tr:        tr,
		config:    config,
		devFolder: devFolder,
	}
}

func (l *localMp) Ready(ctx context.Context) error {
	s := newSnippets(l.tr, l.config.MultipathdCmd)
	return s.status(ctx)
}

func (l *localMp) Setup(ctx context.Context, devices []string) (serializedData string, err error) {
	if len(devices) == 0 {
		return "", fmt.Errorf("no devices as input")
	}

	var shortDevices []string
	for _, v := range devices {
		shortDevices = append(shortDevices, strings.TrimPrefix(strings.TrimPrefix(v, l.devFolder), "/"))
	}

	s := newSnippets(l.tr, l.config.MultipathdCmd)

	err = s.addDevices(ctx, shortDevices)
	if err != nil {
		return "", fmt.Errorf("addDevices: %w", err)
	}

	wwid, err := s.findWwid(ctx, shortDevices)
	if err != nil {
		return "", fmt.Errorf("findMapName: %w", err)
	}

	err = s.addMap(ctx, wwid)
	if err != nil {
		return "", fmt.Errorf("add map: %w", err)
	}

	encoded, err := json.Marshal(Data{
		MapName: wwid,
	})
	if err != nil {
		return "", fmt.Errorf("encode context data: %w", err)
	}

	return string(encoded), nil
}

func (l *localMp) GetDevice(ctx context.Context, serializedData string) (device string, err error) {
	data, err := parseData(serializedData)
	if err != nil {
		return "", fmt.Errorf("parse serialized data: %w", err)
	}

	deviceLink := l.config.MapperFolder + "/" + data.MapName
	return deviceLink, nil
}

func (l *localMp) Destroy(ctx context.Context, serializedData string) error {
	data, err := parseData(serializedData)
	if err != nil {
		return fmt.Errorf("parse serialized data: %w", err)
	}

	s := newSnippets(l.tr, l.config.MultipathdCmd)

	err = s.removeMap(ctx, data.MapName)
	if err != nil {
		return fmt.Errorf("removeMap: %w", err)
	}

	err = s.removeDevices(ctx, data.MapName)
	if err != nil {
		return fmt.Errorf("removeMapDevices: %w", err)
	}

	return nil
}

func (l *localMp) Expand(ctx context.Context, serializedData string) error {
	data, err := parseData(serializedData)
	if err != nil {
		return fmt.Errorf("parse serialized data: %w", err)
	}

	s := newSnippets(l.tr, l.config.MultipathdCmd)
	err = s.resizeMap(ctx, data.MapName)
	if err != nil {
		return fmt.Errorf("resizeMap: %w", err)
	}

	return nil
}

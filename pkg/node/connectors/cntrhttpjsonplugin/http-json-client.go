package cntrhttpjsonplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type client struct {
	config    ClientConfig
	hc        http.Client
	devFolder string
}

type ClientConfig struct {
	Socket string `yaml:"socket"`
}

func (c *ClientConfig) FillDefaults() {
}

func (c *ClientConfig) Validate() error {
	if len(c.Socket) == 0 {
		return fmt.Errorf("socket missing")
	}
	return nil
}

func NewHttpJsonConnectorPlugin(config ClientConfig, devFolder string) connectors.Connector {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", config.Socket)
			},
		},
	}
	return &client{
		config:    config,
		hc:        httpClient,
		devFolder: devFolder,
	}
}

func (m *client) runRequest(ctx context.Context, method string, req any) (string, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := m.hc.Post("http://unix"+method, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("post %s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code %d: %s", resp.StatusCode, body)
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(pr.Logs) != 0 {
		logs.FromCtx(ctx).MicroOp("printing logs from remote connector")
		logs.FromCtx(ctx).ConsumeCopy(pr.Logs, "remote-http-plugin")
	}

	switch pr.Status {
	case ResponseStatusOk:
		return pr.Result, nil
	case ResponseStatusError:
		return "", fmt.Errorf("remote error: %s", pr.Err)
	default:
		return "", fmt.Errorf("unknown response type: %s", pr.Status)
	}
}

func (m *client) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	result, err := m.runRequest(ctx, MethodGetInfo, nil)
	if err != nil {
		return nil, err
	}

	var mni connectors.ConnectorNodeInfo
	err = json.Unmarshal([]byte(result), &mni)
	if err != nil {
		return nil, err
	}

	return &mni, nil
}

func (m *client) Configure(ctx context.Context, _ transport.Transport, preset map[string]string) error {
	return nil
}

func (m *client) Capabilities(ctx context.Context) (connectors.Capabilities, error) {
	result, err := m.runRequest(ctx, MethodGetCapabilities, nil)
	if err != nil {
		return connectors.Capabilities{}, err
	}

	var caps connectors.Capabilities
	err = json.Unmarshal([]byte(result), &caps)
	if err != nil {
		return connectors.Capabilities{}, err
	}

	return caps, nil
}

func (m *client) Connect(ctx context.Context, req connectors.ConnectRequest) (*connectors.ConnectResult, error) {
	result, err := m.runRequest(ctx, MethodConnect, req)
	if err != nil {
		return nil, err
	}

	var mc connectors.ConnectResult
	err = json.Unmarshal([]byte(result), &mc)
	if err != nil {
		return nil, fmt.Errorf("http-json-client: unmarshal result: %w", err)
	}

	// server strips /dev equivalent prefix, so we need to append it back
	for k, v := range mc.Devices {
		if v[0] != '/' {
			v = "/" + v
		}
		mc.Devices[k] = m.devFolder + v
	}

	return &mc, nil
}

func (m *client) Disconnect(ctx context.Context, serializedConnectorData string) error {
	_, err := m.runRequest(ctx, MethodDisconnect, serializedConnectorData)
	if err != nil {
		return err
	}

	return nil
}

func (m *client) Expand(ctx context.Context, serializedConnectorData string) error {
	_, err := m.runRequest(ctx, MethodExpand, serializedConnectorData)
	if err != nil {
		return err
	}

	return nil
}

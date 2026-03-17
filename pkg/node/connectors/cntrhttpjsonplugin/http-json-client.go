package cntrhttpjsonplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
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

func (m *client) GetInfo(ctx context.Context) (*connectors.ConnectorNodeInfo, error) {
	resp, err := m.hc.Get("http://unix/get_info")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d: %s", resp.StatusCode, body)
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		return nil, err
	}

	if len(pr.Logs) != 0 {
		logs.FromCtx(ctx).MicroOp("printing logs from remote connector")
		logs.FromCtx(ctx).ConsumeCopy(pr.Logs)
	}

	switch pr.Status {
	case ResponseStatusOk:
		var mni connectors.ConnectorNodeInfo
		err = json.Unmarshal([]byte(pr.Result), &mni)
		if err != nil {
			return nil, err
		}

		return &mni, nil
	case ResponseStatusError:
		return nil, fmt.Errorf("http-json-client: remote error: %s", pr.Err)
	default:
		return nil, fmt.Errorf("http-json-client: unknown response type: %s", pr.Status)
	}
}

func (m *client) Connect(ctx context.Context, req connectors.ConnectRequest) (*connectors.ConnectResult, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("http-json-client: marshal request: %w", err)
	}

	resp, err := m.hc.Post("http://unix/connect", "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("http-json-client: post /connect: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http-json-client: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d: %s", resp.StatusCode, body)
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		return nil, fmt.Errorf("http-json-client: unmarshal response: %w", err)
	}

	if len(pr.Logs) != 0 {
		logs.FromCtx(ctx).MicroOp("printing logs from remote connector")
		logs.FromCtx(ctx).ConsumeCopy(pr.Logs)
	}

	switch pr.Status {
	case ResponseStatusOk:
		var mc connectors.ConnectResult
		err = json.Unmarshal([]byte(pr.Result), &mc)
		if err != nil {
			return nil, fmt.Errorf("http-json-client: unmarshal result: %w", err)
		}

		// http server in this package strips /dev equivalent prefix, so we need to append it back
		for k, v := range mc.Devices {
			if v[0] != '/' {
				v = "/" + v
			}
			mc.Devices[k] = m.devFolder + v
		}

		return &mc, nil
	case ResponseStatusError:
		return nil, fmt.Errorf("http-json-client: remote error: %s", pr.Err)
	default:
		return nil, fmt.Errorf("http-json-client: unknown response type: %s", pr.Status)
	}
}

func (m *client) Disconnect(ctx context.Context, serializedConnectorData string) error {
	resp, err := m.hc.Post("http://unix/disconnect", "application/text", strings.NewReader(serializedConnectorData))
	if err != nil {
		return fmt.Errorf("http-json-client: post /disconnect: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("http-json-client: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http-json-client: status code %d: %s", resp.StatusCode, body)
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		return fmt.Errorf("http-json-client: unmarshal response: %w", err)
	}

	if len(pr.Logs) != 0 {
		logs.FromCtx(ctx).MicroOp("printing logs from remote connector")
		logs.FromCtx(ctx).ConsumeCopy(pr.Logs)
	}

	switch pr.Status {
	case ResponseStatusOk:
		return nil
	case ResponseStatusError:
		return fmt.Errorf("http-json-client: remote error: %s", pr.Err)
	default:
		return fmt.Errorf("http-json-client: unknown response type: %s", pr.Status)
	}
}

func (m *client) Expand(ctx context.Context, serializedConnectorData string) error {
	resp, err := m.hc.Post("http://unix/expand", "application/json", strings.NewReader(serializedConnectorData))
	if err != nil {
		return fmt.Errorf("http-json-client: post /expand: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("http-json-client: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http-json-client: status code %d: %s", resp.StatusCode, body)
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		return fmt.Errorf("http-json-client: unmarshal response: %w", err)
	}

	if len(pr.Logs) != 0 {
		logs.FromCtx(ctx).MicroOp("printing logs from remote connector")
		logs.FromCtx(ctx).ConsumeCopy(pr.Logs)
	}

	switch pr.Status {
	case ResponseStatusOk:
		return nil
	case ResponseStatusError:
		return fmt.Errorf("http-json-client: remote error: %s", pr.Err)
	default:
		return fmt.Errorf("http-json-client: unknown response type: %s", pr.Status)
	}
}

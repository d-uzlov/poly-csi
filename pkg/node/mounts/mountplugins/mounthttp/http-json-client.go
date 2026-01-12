package mounthttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	"meoe.io/poly-csi/pkg/node/mounts"
)

type client struct {
	config ClientConfig
	hc     http.Client
}

type ClientConfig struct {
	Socket string `yaml:"socket"`
}

func NewHttpJsonMounterPlugin(config ClientConfig) mounts.NodeMounter {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", config.Socket)
			},
		},
	}
	return &client{
		config: config,
		hc:     httpClient,
	}
}

func (m *client) GetInfo(ctx context.Context) (*mounts.MounterNodeInfo, error) {
	resp, err := m.hc.Get("http://unix/get_info")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %d: %s", resp.StatusCode, body)
	}

	var res mounts.MounterNodeInfo
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *client) Connect(ctx context.Context, req mounts.ConnectRequest) (*mounts.MounterContext, error) {
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
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %d: %s", resp.StatusCode, body)
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		return nil, fmt.Errorf("http-json-client: unmarshal response: %w", err)
	}
	switch pr.Status {
	case "ok":
		mcBytes, err := json.Marshal(pr.Result)
		if err != nil {
			return nil, fmt.Errorf("http-json-client: marshal result: %w", err)
		}
		var mc mounts.MounterContext
		err = json.Unmarshal(mcBytes, &mc)
		if err != nil {
			return nil, fmt.Errorf("http-json-client: unmarshal result: %w", err)
		}
		return &mc, nil
	case "error":
		return nil, fmt.Errorf("http-json-client: remote error: %s", pr.Err)
	}
	return nil, fmt.Errorf("http-json-client: unknown response type: %s", pr.Status)
}

func (m *client) Disconnect(ctx context.Context, info mounts.MounterContext) error {
	b, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("http-json-client: marshal request: %w", err)
	}

	resp, err := m.hc.Post("http://unix/disconnect", "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("http-json-client: post /disconnect: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("http-json-client: read body: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("http-json-client: status code %d: %s", resp.StatusCode, body)
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		return fmt.Errorf("http-json-client: unmarshal response: %w", err)
	}
	switch pr.Status {
	case "ok":
		return nil
	case "error":
		return fmt.Errorf("http-json-client: remote error: %s", pr.Err)
	}
	return fmt.Errorf("http-json-client: unknown response type: %s", pr.Status)
}

func (m *client) Expand(ctx context.Context, info mounts.MounterContext) error {
	b, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("http-json-client: marshal request: %w", err)
	}

	resp, err := m.hc.Post("http://unix/expand", "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("http-json-client: post /expand: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("http-json-client: read body: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("http-json-client: status code %d: %s", resp.StatusCode, body)
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		return fmt.Errorf("http-json-client: unmarshal response: %w", err)
	}
	switch pr.Status {
	case "ok":
		return nil
	case "error":
		return fmt.Errorf("http-json-client: remote error: %s", pr.Err)
	}
	return fmt.Errorf("http-json-client: unknown response type: %s", pr.Status)
}

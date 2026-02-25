package mphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/node/multipath"
)

type client struct {
	config ClientConfig
	hc     http.Client
}

type ClientConfig struct {
	Socket string `yaml:"socket"`
}

func NewHttpJsonMultipathPlugin(config ClientConfig) multipath.Multipath {
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

func (m *client) Ready(ctx context.Context) bool {
	resp, err := m.hc.Post("http://unix/ready", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		slogctx.FromCtx(ctx).Error("multipath plugin: bad status code", "code", resp.StatusCode)
		return false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slogctx.FromCtx(ctx).Error("multipath plugin: failed to read body", "error", err)
		return false
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		slogctx.FromCtx(ctx).Error("multipath plugin: unmarshal failed", "error", err)
		return false
	}
	switch pr.Status {
	case "ok":
		return true
	case "error":
		return false
	}
	return false
}

func (m *client) Setup(ctx context.Context, devices []string) (*multipath.MultipathContext, error) {
	b, err := json.Marshal(devices)
	if err != nil {
		return nil, fmt.Errorf("http-json-client: marshal request: %w", err)
	}

	resp, err := m.hc.Post("http://unix/setup", "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("http-json-client: post /setup: %w", err)
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
		var mc multipath.MultipathContext
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

func (m *client) Destroy(ctx context.Context, serializedData string) error {
	resp, err := m.hc.Post("http://unix/destroy", "application/text", strings.NewReader(serializedData))
	if err != nil {
		return fmt.Errorf("http-json-client: post /destroy: %w", err)
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

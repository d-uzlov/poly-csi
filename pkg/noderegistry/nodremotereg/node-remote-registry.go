package nodremotereg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
)

type NodeRemoveRegistry struct {
	address string
}

func NewNodeRemoteRegistry(address string) NodeRemoveRegistry {
	return NodeRemoveRegistry{
		address: address,
	}
}

func (r *NodeRemoveRegistry) Get(id string) (*regnodeinfo.RegNodeInfo, error) {
	qp := make(url.Values)
	qp.Set("id", id)

	resp, err := http.Get(r.address + "/get?" + qp.Encode())
	if err != nil {
		return nil, fmt.Errorf("get node info: query '%s/get': %w", r.address, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("get node info: read body: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get node info: status code %d: %s", resp.StatusCode, body)
	}

	var ni regnodeinfo.RegNodeInfo
	err = json.Unmarshal(body, &ni)
	if err != nil {
		return nil, fmt.Errorf("get node info: unmarshal body: %w", err)
	}
	return &ni, nil
}

func (r *NodeRemoveRegistry) Set(ni regnodeinfo.RegNodeInfo) error {
	b, err := json.Marshal(ni)
	if err != nil {
		return fmt.Errorf("set node info: marshal: %w", err)
	}

	resp, err := http.Post(r.address+"/register", "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("set node info: post '%s/register': %w", r.address, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("set node info: read response: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("status code %d: %s", resp.StatusCode, body)
	}
	return nil
}

package noderegclient

import (
	"context"
	"fmt"
	"net/http"

	"meoe.io/poly-csi/pkg/api/node_registry"
	"meoe.io/poly-csi/pkg/utils/simplerpc"
)

type Client struct {
	client  http.Client
	address string
}

func New(address string) Client {
	return Client{
		client:  http.Client{},
		address: address,
	}
}

func (r *Client) Register(ctx context.Context, req node_registry.RegisterRequest) error {
	var result node_registry.RegisterResponse
	err := simplerpc.MakeQuery(ctx, r.client, r.address+node_registry.RegisterPath, req, &result)
	if err != nil {
		return err
	}
	return nil
}

func (r *Client) Get(ctx context.Context, id string) (node_registry.NodeInfo, error) {
	var result node_registry.GetResponse
	err := simplerpc.MakeQuery(ctx, r.client, r.address+node_registry.GetPath, node_registry.GetRequest{ID: id}, &result)
	if err != nil {
		return node_registry.NodeInfo{}, err
	}
	if !result.Found {
		return node_registry.NodeInfo{}, fmt.Errorf("node '%s' not found", id)
	}
	return result.Info, nil
}

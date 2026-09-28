package cntrhttpjsonplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type DispatcherServer struct {
	tr           transport.Transport
	devFolder    string
	inflate      connectors.InflateFunc
	opaqueConfig yaml.Node
}

func NewDispatcherServer(tr transport.Transport, devFolder string, inflate connectors.InflateFunc, opaqueConfig yaml.Node) *DispatcherServer {
	return &DispatcherServer{
		tr:           tr,
		devFolder:    devFolder,
		inflate:      inflate,
		opaqueConfig: opaqueConfig,
	}
}

func (d *DispatcherServer) Process(ctx context.Context, req PluginRequest) (result any, err error) {
	if d.inflate == nil {
		return nil, fmt.Errorf("inflate is not initialized")
	}
	con := d.inflate()
	if con == nil {
		return nil, fmt.Errorf("inflate returned nil")
	}
	conf := connectors.CommonConfig{
		Tr:        d.tr,
		DevFolder: d.devFolder,
	}
	err = con.Configure(ctx, conf, d.opaqueConfig)
	if err != nil {
		return nil, fmt.Errorf("con.Configure: %w", err)
	}

	switch req.Method {
	case MethodGetCapabilities:
		return con.Capabilities(ctx)
	case MethodGetInfo:
		return con.GetInfo(ctx)
	case MethodMakeHandle:
		var cr connectors.ConnectRequest
		err = json.Unmarshal([]byte(req.SerializedArg), &cr)
		if err != nil {
			return nil, fmt.Errorf("parse arg: %w", err)
		}
		state, err := con.GetHandle(ctx, cr)
		if err != nil {
			return nil, fmt.Errorf("con.GetHandle: %w", err)
		}
		serialized, err := state.Serialize()
		if err != nil {
			return nil, fmt.Errorf("state.Serialize: %w", err)
		}
		return serialized, nil
	case MethodConnectBlock, MethodConnectFolder, MethodGetDeviceList, MethodDisconnect, MethodExpand:
		state, err := con.ParseHandle(req.State)
		if err != nil {
			return nil, fmt.Errorf("con.ParseState: %w", err)
		}
		switch req.Method {
		case MethodConnectBlock:
			return nil, state.ConnectBlock(ctx)
		case MethodConnectFolder:
			var cr connectors.ConnectFolderRequest
			err = json.Unmarshal([]byte(req.SerializedArg), &cr)
			if err != nil {
				return nil, fmt.Errorf("parse arg: %w", err)
			}
			return nil, state.ConnectFolder(ctx, cr)
		case MethodGetDeviceList:
			index, err := strconv.Atoi(req.SerializedArg)
			if err != nil {
				return nil, err
			}
			devices, err := state.GetDevices(ctx, index)
			if err != nil {
				return nil, err
			}
			// allow for different /dev mount
			for k, v := range devices {
				devPath, ok := strings.CutPrefix(v, d.devFolder)
				if ok {
					devices[k] = devPath
				}
			}
			return devices, nil
		case MethodDisconnect:
			return nil, state.Disconnect(ctx)
		case MethodExpand:
			return nil, state.Expand(ctx)
		}
	}

	return nil, fmt.Errorf("method '%s' is not supported", req.Method)
}

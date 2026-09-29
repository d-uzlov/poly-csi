package cntrhttpjsonplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"meoe.io/poly-csi/pkg/node/connectors"
	"meoe.io/poly-csi/pkg/transport"
)

type DispatcherServer struct {
	tr        transport.Transport
	devFolder string
	cons      map[string]connectors.Manager
}

func NewDispatcherServer(tr transport.Transport, devFolder string, cons map[string]connectors.Manager) *DispatcherServer {
	return &DispatcherServer{
		tr:        tr,
		devFolder: devFolder,
		cons:      cons,
	}
}

func (d *DispatcherServer) Process(ctx context.Context, req PluginRequest) (result any, state string, err error) {
	if d.cons == nil {
		return nil, "", fmt.Errorf("connector is not initialized")
	}
	con, ok := d.cons[req.ConnectorId]
	if !ok {
		return nil, "", fmt.Errorf("connector with id '%s' is not found", req.ConnectorId)
	}

	switch req.Method {
	case MethodGetCapabilities:
		caps, err := con.Capabilities(ctx)
		return caps, "", err
	case MethodGetInfo:
		info, err := con.GetInfo(ctx)
		return info, "", err
	case MethodMakeHandle:
		var cr connectors.ConnectRequest
		err = json.Unmarshal([]byte(req.SerializedArg), &cr)
		if err != nil {
			return nil, "", fmt.Errorf("parse arg: %w", err)
		}
		state, err := con.GetHandle(ctx, cr)
		if err != nil {
			return nil, "", fmt.Errorf("con.GetHandle: %w", err)
		}
		serialized, err := state.Serialize()
		if err != nil {
			return nil, "", fmt.Errorf("state.Serialize: %w", err)
		}
		return nil, serialized, nil
	case MethodConnectBlock, MethodConnectFolder, MethodGetDeviceList, MethodDisconnect, MethodExpand:
		state, err := con.ParseHandle(req.State)
		if err != nil {
			return nil, "", fmt.Errorf("con.ParseState: %w", err)
		}
		result = nil
		switch req.Method {
		case MethodConnectBlock:
			err = state.ConnectBlock(ctx)
			if err != nil {
				return nil, "", err
			}
		case MethodConnectFolder:
			var cr connectors.ConnectFolderRequest
			err = json.Unmarshal([]byte(req.SerializedArg), &cr)
			if err != nil {
				return nil, "", fmt.Errorf("parse arg: %w", err)
			}
			err = state.ConnectFolder(ctx, cr)
			if err != nil {
				return nil, "", err
			}

		case MethodGetDeviceList:
			index, err := strconv.Atoi(req.SerializedArg)
			if err != nil {
				return nil, "", err
			}
			devices, err := state.GetDevices(ctx, index)
			if err != nil {
				return nil, "", err
			}
			// allow for different /dev mount
			for k, v := range devices {
				devPath, ok := strings.CutPrefix(v, d.devFolder)
				if ok {
					devices[k] = devPath
				}
			}
			result = devices
		case MethodDisconnect:
			err = state.Disconnect(ctx)
			if err != nil {
				return nil, "", err
			}
		case MethodExpand:
			err = state.Expand(ctx)
			if err != nil {
				return nil, "", err
			}
		}

		serialized, err := state.Serialize()
		if err != nil {
			return nil, "", fmt.Errorf("serialize after running: %w", err)
		}
		return result, serialized, nil
	}

	return nil, "", fmt.Errorf("method '%s' is not supported", req.Method)
}

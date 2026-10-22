package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/blockpluginapi"
	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/api/publishcontext"
	"meoe.io/poly-csi/pkg/controller/uuidctx"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/utils/simplerpc"
)

type DispatcherServer struct {
	tr        transport.Transport
	devFolder string
	cons      map[string]blockapi.Manager
	logConfig logs.Config
}

func New(tr transport.Transport, devFolder string, cons map[string]blockapi.Manager, logConfig logs.Config) *DispatcherServer {
	return &DispatcherServer{
		tr:        tr,
		devFolder: devFolder,
		cons:      cons,
		logConfig: logConfig,
	}
}

func (ds *DispatcherServer) inspectQuery(ctx context.Context, r *http.Request, data any, next func(context.Context) any) error {
	logConfig := ds.logConfig
	callUuid := uuid.NewString()
	if logConfig.ExtraFields == nil {
		logConfig.ExtraFields = map[string]string{}
	}
	logConfig.ExtraFields["request-uuid"] = callUuid
	ctx = uuidctx.NewCtx(ctx, callUuid)

	logger := logConfig.MakeInstance()
	defer func() {
		logger.Commit(true)
	}()
	ctx = logs.NewCtx(ctx, logger)

	logger.Info("new request", "path", r.URL.Path, "request", data)

	result := next(ctx)
	provider, ok := result.(blockpluginapi.ResponseProvider)
	if !ok {
		return fmt.Errorf("next did not return ResponseProvider, but instead %T", result)
	}
	resp := provider.Get()
	if resp.ErrorString == "" {
		logger.Info("request success", "result", result)
	} else {
		logger.Error("request fail", "error", resp.ErrorString)
	}

	return nil
}

func (ds *DispatcherServer) RegisterSelf(mux *http.ServeMux) {
	opts := []simplerpc.Option{simplerpc.WithMiddleware(ds.inspectQuery)}

	mux.HandleFunc(blockpluginapi.Capabilities, simplerpc.MakeHandler(ds.Capabilities, opts...))
	mux.HandleFunc(blockpluginapi.GetHandle, simplerpc.MakeHandler(ds.GetHandle, opts...))
	mux.HandleFunc(blockpluginapi.StateConnect, simplerpc.MakeHandler(ds.Connect, opts...))
	mux.HandleFunc(blockpluginapi.StateGetDevices, simplerpc.MakeHandler(ds.GetDevices, opts...))
	mux.HandleFunc(blockpluginapi.StateExpand, simplerpc.MakeHandler(ds.Expand, opts...))
	mux.HandleFunc(blockpluginapi.StateDisconnect, simplerpc.MakeHandler(ds.Disconnect, opts...))
}

func (ds *DispatcherServer) Capabilities(ctx context.Context, req blockpluginapi.InvokeRequest) blockpluginapi.GetCapabilitiesResponse {
	con := ds.cons[req.ConnectorId]
	if con == nil {
		return blockpluginapi.GetCapabilitiesResponse{ErrorString: fmt.Sprintf("connector not found: '%s'", req.ConnectorId)}
	}

	caps, err := con.Capabilities(ctx)
	if err != nil {
		return blockpluginapi.GetCapabilitiesResponse{ErrorString: fmt.Sprintf("connector error: %s", err)}
	}

	return blockpluginapi.GetCapabilitiesResponse{Caps: caps}
}

func (ds *DispatcherServer) GetHandle(ctx context.Context, req blockpluginapi.GetHandleRequest) blockpluginapi.InvokeStateResponse {
	con := ds.cons[req.ConnectorId]
	if con == nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("connector not found: '%s'", req.ConnectorId)}
	}

	reg := publishcontext.Registry
	proto, err := reg.CreateInstance(req.ProtoName)
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("protocol not found: %s", err)}
	}

	protoVal, ok := proto.Elem().Interface().(blockspec.Data)
	if !ok {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("protocol type mismatch: need '%s' got %T", req.ProtoName, proto)}
	}

	err = publishcontext.InterfaceUnmarshal(req.Protocol, &protoVal)
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("parse protocol: %s", err)}
	}

	req.Data.Proto = protoVal

	handle, err := con.GetHandle(ctx, req.Data)
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("con.GetHandle: %s", err)}
	}

	state, err := handle.Serialize()
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("handle.Serialize: %s", err)}
	}

	return blockpluginapi.InvokeStateResponse{State: state}
}

func (ds *DispatcherServer) inflateState(req blockpluginapi.InvokeStateRequest) (blockapi.Handle, error) {
	con := ds.cons[req.ConnectorId]
	if con == nil {
		return nil, fmt.Errorf("connector not found: '%s'", req.ConnectorId)
	}

	return con.ParseHandle(req.State)
}

func (ds *DispatcherServer) Connect(ctx context.Context, req blockpluginapi.InvokeStateRequest) blockpluginapi.InvokeStateResponse {
	handle, err := ds.inflateState(req)
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("inflateState: %s", err)}
	}

	err = handle.Connect(ctx)
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("handle.Connect: %s", err)}
	}

	state, err := handle.Serialize()
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("handle.Serialize: %s", err)}
	}

	return blockpluginapi.InvokeStateResponse{State: state}
}

func (ds *DispatcherServer) GetDevices(ctx context.Context, req blockpluginapi.InvokeStateRequest) blockpluginapi.GetDevicesResponse {
	handle, err := ds.inflateState(req)
	if err != nil {
		return blockpluginapi.GetDevicesResponse{ErrorString: fmt.Sprintf("inflateState: %s", err)}
	}

	devices, err := handle.GetDevices(ctx)
	if err != nil {
		return blockpluginapi.GetDevicesResponse{ErrorString: fmt.Sprintf("handle.Connect: %s", err)}
	}

	state, err := handle.Serialize()
	if err != nil {
		return blockpluginapi.GetDevicesResponse{ErrorString: fmt.Sprintf("handle.Serialize: %s", err)}
	}

	// allow for different /dev mount
	for k, v := range devices {
		devPath, ok := strings.CutPrefix(v, ds.devFolder)
		if ok {
			devices[k] = devPath
		}
	}

	return blockpluginapi.GetDevicesResponse{State: state, Devices: devices}
}

func (ds *DispatcherServer) Expand(ctx context.Context, req blockpluginapi.InvokeStateRequest) blockpluginapi.InvokeStateResponse {
	handle, err := ds.inflateState(req)
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("inflateState: %s", err)}
	}

	err = handle.Expand(ctx)
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("handle.Expand: %s", err)}
	}

	state, err := handle.Serialize()
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("handle.Serialize: %s", err)}
	}

	return blockpluginapi.InvokeStateResponse{State: state}
}

func (ds *DispatcherServer) Disconnect(ctx context.Context, req blockpluginapi.InvokeStateRequest) blockpluginapi.InvokeStateResponse {
	handle, err := ds.inflateState(req)
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("inflateState: %s", err)}
	}

	err = handle.Disconnect(ctx)
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("handle.Disconnect: %s", err)}
	}

	state, err := handle.Serialize()
	if err != nil {
		return blockpluginapi.InvokeStateResponse{ErrorString: fmt.Sprintf("handle.Serialize: %s", err)}
	}

	return blockpluginapi.InvokeStateResponse{State: state}
}

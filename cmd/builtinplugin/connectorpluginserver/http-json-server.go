package connectorpluginserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/logs"
	"meoe.io/poly-csi/pkg/node/connectors"
	cntr "meoe.io/poly-csi/pkg/node/connectors/cntrhttpjsonplugin"
)

type ConnectorPlugin interface {
	StartServe(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error))
}

type Server struct {
	config    ServerConfig
	connector connectors.Connector
	callCtx   context.Context
	handlers  map[string]handler
	devFolder string
	logs      logs.Config
}

type handlerFunc func(ctx context.Context, body []byte) (result any, err error)

type handler struct {
	httpMethodType string
	f              handlerFunc
}

type ServerConfig struct {
	Socket string `yaml:"socket"`
}

func NewServer(callCtx context.Context, config ServerConfig, connector connectors.Connector, devFolder string, logs logs.Config) ConnectorPlugin {
	return &Server{
		callCtx:   callCtx,
		config:    config,
		connector: connector,
		handlers:  map[string]handler{},
		devFolder: devFolder,
		logs:      logs,
	}
}

func (s *Server) StartServe(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error)) {
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	s.addHandler(mux, cntr.MethodGetCapabilities, s.handleGetCapabilities)
	s.addHandler(mux, cntr.MethodGetInfo, s.handleGetInfo)
	s.addHandler(mux, cntr.MethodConnect, s.handleConnect)
	s.addHandler(mux, cntr.MethodDisconnect, s.handleDisconnect)
	s.addHandler(mux, cntr.MethodExpand, s.handleExpand)

	serveDoneCh := make(chan struct{})

	go func() {
		defer close(serveDoneCh)
		unixListener, err := net.Listen("unix", s.config.Socket)
		if err != nil {
			doneCallback(err)
			return
		}
		err = srv.Serve(unixListener)
		doneCallback(err)
	}()
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()
			slogctx.FromCtx(ctx).Info("context done, running server shutdown")
			_ = srv.Shutdown(shutdownCtx)
		case <-serveDoneCh:
		}
	}()
}

func (s *Server) getLogger(path string) *logs.Instance {
	return s.logs.WithFields("method", path).MakeInstanceWithCopy(true)
}

func (s *Server) addHandler(mux *http.ServeMux, path string, f handlerFunc) {
	_, ok := s.handlers[path]
	if ok {
		logger := slogctx.FromCtx(s.callCtx)
		logger.Warn("trying to add handler for second time", "path", path)
		return
	}
	s.handlers[path] = handler{
		httpMethodType: http.MethodPost,
		f:              f,
	}
	mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		resp, err, errCode := s.handle(req)
		if err != nil {
			serializedError := fmt.Sprintf(`{"status":"%s", "error": "%s"}`, cntr.ResponseStatusError, err.Error())
			http.Error(w, serializedError, errCode)
			slogctx.FromCtx(s.callCtx).Error("handler failure", "error", err)
			return
		}
		if resp.Err != "" {
			slogctx.FromCtx(s.callCtx).Info("handler finished with error", "error", resp.Err)
		} else {
			slogctx.FromCtx(s.callCtx).Info("handler finished successfully")
		}
		res, err := json.Marshal(resp)
		if err != nil {
			serializedError := fmt.Sprintf(`{"status":"%s", "error": "json marshal plugin response: %s"}`, cntr.ResponseStatusError, err.Error())
			http.Error(w, serializedError, http.StatusInternalServerError)
			slogctx.FromCtx(s.callCtx).Error("json marshal failure", "error", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, writeErr := w.Write(res)
		if writeErr != nil {
			slogctx.FromCtx(s.callCtx).Error("failed to write result", "error", writeErr)
		}
	})
}

func (s *Server) handle(req *http.Request) (resp cntr.PluginResponse, err error, errCode int) {
	path := req.URL.Path
	h, ok := s.handlers[path]
	if !ok {
		return cntr.PluginResponse{}, fmt.Errorf("method %s is not found", path), http.StatusBadRequest
	}

	if req.Method != h.httpMethodType {
		return cntr.PluginResponse{}, fmt.Errorf("path %s needs method %s, requested %s is not supported", path, h.httpMethodType, req.Method), http.StatusMethodNotAllowed
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return cntr.PluginResponse{}, fmt.Errorf("read request body: %w", err), http.StatusInternalServerError
	}

	logger := s.getLogger(path)
	defer func() {
		logger.Commit(err == nil)
	}()
	ctx := logs.NewCtx(s.callCtx, logger)

	logger.EphemeralInfo("new request")

	callRes, err := h.f(ctx, bodyBytes)
	if err != nil {
		return cntr.PluginResponse{
			Status: cntr.ResponseStatusError,
			Err:    fmt.Sprintf("handler failed: %s", err.Error()),
			Logs:   logger.GetCopy(),
		}, nil, 0
	}
	logger.EphemeralInfo("handler call success", "data", callRes)

	serialized, err := json.Marshal(callRes)
	if err != nil {
		return cntr.PluginResponse{}, fmt.Errorf("json marshal call result: %w", err), http.StatusInternalServerError
	}

	return cntr.PluginResponse{
		Status: cntr.ResponseStatusOk,
		Result: string(serialized),
		Logs:   logger.GetCopy(),
	}, nil, 0
}

func (s *Server) handleGetCapabilities(ctx context.Context, body []byte) (result any, err error) {
	return s.connector.Capabilities(s.callCtx)
}

func (s *Server) handleGetInfo(ctx context.Context, body []byte) (result any, err error) {
	return s.connector.GetInfo(s.callCtx)
}

func (s *Server) handleConnect(ctx context.Context, body []byte) (result any, err error) {
	// TODO move logger to interface
	logger := logs.FromCtx(ctx)

	var cr connectors.ConnectRequest

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	if err = dec.Decode(&cr); err != nil {
		logger.EphemeralError("failed to parse request body", "req_body", body, "error", err.Error())
		return nil, fmt.Errorf("decode request body: %w", err)
	}
	logger.EphemeralInfo("parsed request", "request", cr)
	mc, err := s.connector.Connect(ctx, cr)
	if err != nil {
		return nil, err
	}
	if mc == nil {
		return nil, fmt.Errorf("connector returned nil result")
	}

	// allow for different /dev mount
	for k, v := range mc.Devices {
		devPath, ok := strings.CutPrefix(v, s.devFolder)
		if ok {
			mc.Devices[k] = devPath
		}
	}

	if mc.ConnectorData != nil {
		if mc.SerializedConnectorData != "" {
			return nil, fmt.Errorf("connector set both ConnectorData and serializedConnectorData")
		}
		encoded, err := json.Marshal(mc.ConnectorData)
		if err != nil {
			return nil, fmt.Errorf("serialize ConnectorData: %w", err)
		}
		mc.SerializedConnectorData = string(encoded)
		mc.ConnectorData = nil
	}

	return mc, nil
}

func (s *Server) handleDisconnect(ctx context.Context, body []byte) (result any, err error) {
	var serializedData string

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	if err = dec.Decode(&serializedData); err != nil {
		return nil, fmt.Errorf("decode request body: %w", err)
	}

	logs.FromCtx(ctx).EphemeralInfo("parsed request", "serialized-data", serializedData)
	err = s.connector.Disconnect(ctx, serializedData)
	return nil, err
}

func (s *Server) handleExpand(ctx context.Context, body []byte) (result any, err error) {
	var serializedData string

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	if err = dec.Decode(&serializedData); err != nil {
		return nil, fmt.Errorf("decode request body: %w", err)
	}

	logs.FromCtx(ctx).EphemeralInfo("parsed request", "serialized-data", serializedData)
	err = s.connector.Expand(ctx, serializedData)
	return nil, err
}

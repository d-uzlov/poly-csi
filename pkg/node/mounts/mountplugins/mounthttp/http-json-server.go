package mounthttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/node/mounts/mountplugins"
)

type Server struct {
	config   ServerConfig
	mounter  mounts.NodeMounter
	callCtx  context.Context
	handlers map[string]handler
}

const responseStatusError = "error"
const responseStatusOk = "ok"

type PluginResponse struct {
	Status string `json:"status"`
	Err    string `json:"error"`
	Result string `json:"result"`
}

type handlerFunc func(ctx context.Context, body string) (result any, err error)

type handler struct {
	httpMethodType string
	f              handlerFunc
}

type ServerConfig struct {
	Socket string `yaml:"socket"`
}

func NewServer(callCtx context.Context, config ServerConfig, mounter mounts.NodeMounter) mountplugins.MounterPlugin {
	return &Server{
		callCtx:  callCtx,
		config:   config,
		mounter:  mounter,
		handlers: map[string]handler{},
	}
}

func (s *Server) StartServe(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error)) {
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	s.addHandler(mux, "/get_info", http.MethodGet, s.handleGetInfo)
	s.addHandler(mux, "/connect", http.MethodPost, s.handleConnect)
	s.addHandler(mux, "/disconnect", http.MethodPost, s.handleDisconnect)
	s.addHandler(mux, "/expand", http.MethodPost, s.handleExpand)

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

func (s *Server) addHandler(mux *http.ServeMux, path string, method string, f handlerFunc) {
	_, ok := s.handlers[path]
	if ok {
		logger := slogctx.FromCtx(s.callCtx)
		logger.Warn("trying to add handler for second time", "path", path)
		return
	}
	s.handlers[path] = handler{
		httpMethodType: method,
		f:              f,
	}
	mux.HandleFunc(path, s.handle)
}

func (s *Server) handle(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	h, ok := s.handlers[path]
	if !ok {
		http.Error(w, fmt.Sprintf(`{"status":"%s", "error": "method %s is not found"}`, responseStatusError, path), http.StatusBadRequest)
		return
	}

	if req.Method != h.httpMethodType {
		http.Error(w, fmt.Sprintf("path %s needs method %s, requested %s is not supported", path, h.httpMethodType, req.Method), http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"%s", "error": "read request body: %s"}`, responseStatusError, err.Error()), http.StatusInternalServerError)
		return
	}

	callRes, err := h.f(s.callCtx, string(bodyBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"%s", "error": "%s"}`, responseStatusError, err.Error()), http.StatusBadRequest)
		return
	}

	serialized, err := json.Marshal(callRes)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"%s", "error": "json marshal call result: %s"}`, responseStatusError, err.Error()), http.StatusInternalServerError)
		return
	}

	res, err := json.Marshal(&PluginResponse{Status: responseStatusOk, Result: string(serialized)})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"%s", "error": "json marshal plugin response: %s"}`, responseStatusError, err.Error()), http.StatusInternalServerError)
		return
	}
	slogctx.FromCtx(s.callCtx).Info("call success", "method", req.URL.Path, "data", callRes)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(res)
	// TODO log write errors
}

func (s *Server) handleGetInfo(ctx context.Context, body string) (result any, err error) {
	return s.mounter.GetInfo(s.callCtx)
}

func (s *Server) handleConnect(ctx context.Context, body string) (result any, err error) {
	// TODO move logger to interface
	logger := slogctx.FromCtx(s.callCtx).With("method", "connect")

	var cr mounts.ConnectRequest

	dec := json.NewDecoder(bytes.NewReader([]byte(body)))
	dec.DisallowUnknownFields()

	if err = dec.Decode(&cr); err != nil {
		logger.Info("failed to parse request body", "req_body", body)
		return nil, fmt.Errorf("decode request body: %w", err)
	}
	logger.Info("new request", "request", cr)
	mc, err := s.mounter.Connect(s.callCtx, cr)
	if err != nil {
		return nil, err
	}

	if mc.MounterData != nil {
		if mc.SerializedMounterData != "" {
			return nil, fmt.Errorf("mounter set both MounterData and SerializedMounterData")
		}
		encoded, err := json.Marshal(mc.MounterData)
		if err != nil {
			return nil, fmt.Errorf("serialize MounterData: %w", err)
		}
		mc.SerializedMounterData = string(encoded)
		mc.MounterData = nil
	}

	return mc, nil
}

func (s *Server) handleDisconnect(ctx context.Context, body string) (result any, err error) {
	logger := slogctx.FromCtx(s.callCtx).With("method", "disconnect")

	logger.Info("new request", "request", body)
	err = s.mounter.Disconnect(s.callCtx, body)
	return nil, err
}

func (s *Server) handleExpand(ctx context.Context, body string) (result any, err error) {
	logger := slogctx.FromCtx(s.callCtx).With("method", "expand")

	logger.Info("new request", "request", body)
	err = s.mounter.Expand(s.callCtx, body)
	return nil, err
}

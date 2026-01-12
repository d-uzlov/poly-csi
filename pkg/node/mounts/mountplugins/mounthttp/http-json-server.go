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
	config  ServerConfig
	mounter mounts.NodeMounter
	callCtx context.Context
}

type ServerConfig struct {
	Socket string `yaml:"socket"`
}

func NewServer(callCtx context.Context, config ServerConfig, mounter mounts.NodeMounter) mountplugins.MounterPlugin {
	return &Server{
		callCtx: callCtx,
		config:  config,
		mounter: mounter,
	}
}

func (s *Server) StartServe(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error)) {
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/get_info", s.handleGetInfo)
	mux.HandleFunc("/connect", s.handleConnect)
	mux.HandleFunc("/disconnect", s.handleDisconnect)
	mux.HandleFunc("/expand", s.handleExpand)

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

func (s *Server) handleGetInfo(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, fmt.Sprintf("method not allowed: '%s' %s", req.Method, req.URL), http.StatusMethodNotAllowed)
		return
	}
	info, err := s.mounter.GetInfo(s.callCtx)
	if err != nil {
		http.Error(w, "encode error: "+err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(info); err != nil {
		http.Error(w, "encode error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleConnect(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, fmt.Sprintf("method not allowed: '%s' %s", req.Method, req.URL), http.StatusMethodNotAllowed)
		return
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"error", "error": "read request body: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	slogctx.FromCtx(s.callCtx).Info("new request", "req_body", string(bodyBytes))

	var cr mounts.ConnectRequest
	dec := json.NewDecoder(bytes.NewReader(bodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cr); err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"error", "error": "invalid json: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	logger := slogctx.FromCtx(s.callCtx)
	logger.Info("connect: got request", "request", cr)
	mc, err := s.mounter.Connect(s.callCtx, cr)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"error", "error": "connect: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	res, err := json.Marshal(PluginResponse{Status: "ok", Result: mc})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"error", "error": "connect: json marshal failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(res)
}

func (s *Server) handleDisconnect(w http.ResponseWriter, req *http.Request) {
	slogctx.FromCtx(s.callCtx).Info("new request", "req_body", req.Body)
	if req.Method != http.MethodPost {
		http.Error(w, fmt.Sprintf("method not allowed: '%s' %s", req.Method, req.URL), http.StatusMethodNotAllowed)
		return
	}
	var mc mounts.MounterContext
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&mc); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	logger := slogctx.FromCtx(s.callCtx)
	logger.Info("disconnect: got request", "request", mc)
	err := s.mounter.Disconnect(s.callCtx, mc)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"error", "error": "disconnect: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	res, err := json.Marshal(PluginResponse{Status: "ok"})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"error", "error": "disconnect: json marshal failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(res)
}

func (s *Server) handleExpand(w http.ResponseWriter, req *http.Request) {
	slogctx.FromCtx(s.callCtx).Info("new request", "req_body", req.Body)
	if req.Method != http.MethodPost {
		http.Error(w, fmt.Sprintf("method not allowed: '%s' %s", req.Method, req.URL), http.StatusMethodNotAllowed)
		return
	}
	var mc mounts.MounterContext
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&mc); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	logger := slogctx.FromCtx(s.callCtx)
	logger.Info("expand: got request", "request", mc)
	err := s.mounter.Expand(s.callCtx, mc)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"error", "error": "expand: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	res, err := json.Marshal(PluginResponse{Status: "ok"})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"error", "error": "expand: json marshal failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(res)
}

type PluginResponse struct {
	Status string `json:"status"`
	Err    string `json:"error"`
	Result any    `json:"result"`
}

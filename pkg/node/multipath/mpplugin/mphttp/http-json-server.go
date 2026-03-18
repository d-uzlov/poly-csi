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
	"time"

	slogctx "github.com/veqryn/slog-context"
	"meoe.io/poly-csi/pkg/node/mounts/mountplugins"
	"meoe.io/poly-csi/pkg/node/multipath"
)

type Server struct {
	config    ServerConfig
	mp        multipath.Multipath
	callCtx   context.Context
	devFolder string
}

type ServerConfig struct {
	Socket string `yaml:"socket"`
}

func NewServer(callCtx context.Context, config ServerConfig, mp multipath.Multipath, devFolder string) mountplugins.ConnectorPlugin {
	return &Server{
		callCtx:   callCtx,
		config:    config,
		mp:        mp,
		devFolder: devFolder,
	}
}

func (s *Server) StartServe(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error)) {
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) { s.handleRaw(w, r, s.handleReady) })
	mux.HandleFunc("/setup", func(w http.ResponseWriter, r *http.Request) { s.handleRaw(w, r, s.handleSetup) })
	mux.HandleFunc("/destroy", func(w http.ResponseWriter, r *http.Request) { s.handleRaw(w, r, s.handleDestroy) })

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

type Handler func(body []byte) (result any, err error)

func (s *Server) handleRaw(w http.ResponseWriter, req *http.Request, h Handler) {
	var err error
	var respBytes []byte
	errType := http.StatusInternalServerError
	defer func() {
		if err != nil {
			slogctx.FromCtx(s.callCtx).Error("request failed", "error", err.Error())
			http.Error(w, fmt.Sprintf(`{"status":"error", "error": "%s"}`, err.Error()), errType)
			return
		}
		if respBytes == nil {
			slogctx.FromCtx(s.callCtx).Error("respBytes is nil")
			return
		}
		_, err = w.Write(respBytes)
		if err != nil {
			slogctx.FromCtx(s.callCtx).Error("failed to send response", "error", err.Error(), "response", string(respBytes))
		} else {
			slogctx.FromCtx(s.callCtx).Info("sent response", "response", string(respBytes))
		}
	}()

	if req.Method != http.MethodPost {
		err = fmt.Errorf("method not allowed: '%s' %s", req.Method, req.URL)
		errType = http.StatusMethodNotAllowed
		return
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		err = fmt.Errorf("read request body: %s", err)
		return
	}
	slogctx.FromCtx(s.callCtx).Info("new request", "path", req.URL.Path, "req_body", string(bodyBytes))

	result, err := h(bodyBytes)
	if err != nil {
		return
	}

	resp := PluginResponse{Status: "ok", Result: result}
	respBytes, err = json.Marshal(resp)
	if err != nil {
		return
	}
}

func (s *Server) handleReady(body []byte) (any, error) {
	ok := s.mp.Ready(s.callCtx)
	if !ok {
		return nil, fmt.Errorf("not ready")
	}
	return nil, nil
}

func (s *Server) handleSetup(body []byte) (any, error) {
	var devices []string
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&devices); err != nil {
		return nil, fmt.Errorf("request body: invalid json: %s", err)
	}
	logger := slogctx.FromCtx(s.callCtx)
	logger.Info("connect: got request", "request", devices)

	// http client in this package strips /dev equivalent prefix, so we need to append it back
	for k, v := range devices {
		if v[0] != '/' {
			v = "/" + v
		}
		devices[k] = s.devFolder + v
	}

	mc, err := s.mp.Setup(s.callCtx, devices)
	if err != nil {
		return nil, fmt.Errorf("call setup: %s", err)
	}

	// allow for different /dev mount
	devPath, ok := strings.CutPrefix(mc.Device, s.devFolder)
	if ok {
		mc.Device = devPath
	}
	return mc, nil
}

func (s *Server) handleDestroy(body []byte) (any, error) {
	logger := slogctx.FromCtx(s.callCtx)
	logger.Info("connect: got request", "request", string(body))
	err := s.mp.Destroy(s.callCtx, string(body))
	if err != nil {
		return nil, fmt.Errorf("call setup: %s", err)
	}
	return nil, nil
}

type PluginResponse struct {
	Status string `json:"status"`
	Err    string `json:"error"`
	Result any    `json:"result"`
}

package cntrhttpjsonplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"uuid"

	"go.yaml.in/yaml/v3"
	"meoe.io/poly-csi/pkg/logs"
)

const ConnectorHttpPath = "/connector"

type client struct {
	config    ClientConfig
	hc        http.Client
}

type ClientConfig struct {
	Socket string `yaml:"socket"`
}

func (c *ClientConfig) FillDefaults() {
}

func (c *ClientConfig) Validate() error {
	if len(c.Socket) == 0 {
		return fmt.Errorf("socket missing")
	}
	return nil
}

func NewHttpClient() Sender {
	return &client{
	}
}

func (m *client) SetConfig(opaqueConfig []byte) error {
	if err := yaml.Unmarshal(opaqueConfig, &m.config); err != nil {
		return fmt.Errorf("parse opaqueConfig: %w", err)
	}
	m.hc = http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", m.config.Socket)
			},
		},
	}
	return nil
}

func (m *client) Send(ctx context.Context, req PluginRequest) (result string, err error) {
	b, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := m.hc.Post("http://unix/connector", "application/json", bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("post %s: %w", req.Method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code %d: %s", resp.StatusCode, body)
	}

	var pr PluginResponse
	err = json.Unmarshal(body, &pr)
	if err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(pr.Logs) != 0 {
		logs.FromCtx(ctx).MicroOp("printing logs from remote connector")
		logs.FromCtx(ctx).ConsumeCopy(pr.Logs, "remote-http-plugin")
	}

	switch pr.Status {
	case ResponseStatusOk:
		return pr.Result, nil
	case ResponseStatusError:
		return "", fmt.Errorf("remote error: %s", pr.Err)
	default:
		return "", fmt.Errorf("unknown response type: %s", pr.Status)
	}
}

type HttpServer struct {
	callCtx    context.Context
	dispatcher *DispatcherServer
	logs       logs.Config
}

func NewHttpServer(callCtx context.Context, dispatcher *DispatcherServer, logs logs.Config) *HttpServer {
	return &HttpServer{
		callCtx:    callCtx,
		dispatcher: dispatcher,
		logs:       logs,
	}
}

func (s *HttpServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	requestUuid := uuid.NewV7().String()
	logger := s.logs.WithFields("local-uuid", requestUuid).MakeInstanceWithCopy(true)
	ctx := logs.NewCtx(s.callCtx, logger)

	requestSuccess := false
	defer func() {
		logger.Commit(requestSuccess)
	}()

	resp, err, errCode := s.handle(ctx, logger, req)
	var pluginResp PluginResponse
	if err != nil {
		logger.EphemeralError("handler finished with error", "error", err)
		pluginResp = PluginResponse{
			Status: ResponseStatusError,
			Err:    err.Error(),
			Result: "",
			Logs:   logger.GetCopy(),
		}
	} else {
		logger.EphemeralInfo("handler finished successfully")
		pluginResp = PluginResponse{
			Status: ResponseStatusOk,
			Err:    "",
			Result: resp,
			Logs:   logger.GetCopy(),
		}
	}
	res, err := json.Marshal(pluginResp)
	if err != nil {
		logger.Error("failed to json marshal plugin response", "error", err)
		// in the case we are unable to create json automatically
		// fall back to simple manually-written error
		http.Error(w, fmt.Sprintf(`{"status": "%s", "error": "failed to json marshal plugin response"}`, ResponseStatusError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errCode)
	_, writeErr := w.Write(res)
	if writeErr != nil {
		logger.Error("failed to write result", "error", writeErr)
		return
	}

	requestSuccess = err == nil
}

func (s *HttpServer) handle(ctx context.Context, logger *logs.Instance, req *http.Request) (resp string, err error, errCode int) {
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return "", fmt.Errorf("read request body: %w", err), http.StatusInternalServerError
	}

	var parsedRequest PluginRequest
	err = json.Unmarshal(bodyBytes, &parsedRequest)
	if err != nil {
		return "", fmt.Errorf("parse request: %w", err), http.StatusBadRequest
	}

	logger.EphemeralInfo("new request")

	callRes, err := s.dispatcher.Process(ctx, parsedRequest)
	if err != nil {
		return "", err, http.StatusInternalServerError
	}
	logger.EphemeralInfo("handler call success", "data", callRes)

	serialized, err := json.Marshal(callRes)
	if err != nil {
		return "", fmt.Errorf("json marshal call result: %w", err), http.StatusInternalServerError
	}

	return string(serialized), nil, http.StatusOK
}

package noderegsrv

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"sync"
	"time"

	"meoe.io/poly-csi/pkg/noderegistry/regnodeinfo"
)

type NodeRegistry struct {
	srv  *http.Server
	mu   sync.RWMutex
	data map[string]regnodeinfo.RegNodeInfo

	logger *slog.Logger
}

func NewNodeRegistry(logger *slog.Logger) *NodeRegistry {
	return &NodeRegistry{
		data:   map[string]regnodeinfo.RegNodeInfo{},
		logger: logger,
	}
}

func (nr *NodeRegistry) Set(ni regnodeinfo.RegNodeInfo) (changed bool) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	same := maps.Equal(nr.data[ni.Id].Data, ni.Data)
	nr.data[ni.Id] = ni
	return !same
}

func (nr *NodeRegistry) Get(id string) *regnodeinfo.RegNodeInfo {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	res, ok := nr.data[id]
	if !ok {
		return nil
	}
	return &res
}

func (nr *NodeRegistry) handleRegister(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var ni regnodeinfo.RegNodeInfo
	dec := json.NewDecoder(req.Body)
	if err := dec.Decode(&ni); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if ni.Id == "" {
		http.Error(w, "node_id required", http.StatusBadRequest)
		return
	}
	changed := nr.Set(ni)
	if changed {
		nr.logger.Info("registered node", "id", ni.Id, "data", ni.Data)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (nr *NodeRegistry) handleGet(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := req.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "query params must contain id", http.StatusBadRequest)
	}
	node := nr.Get(id)
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(node); err != nil {
		http.Error(w, "encode error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (nr *NodeRegistry) handleGetAll(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	nr.mu.Lock()
	defer nr.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(nr.data); err != nil {
		http.Error(w, "encode error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (nr *NodeRegistry) StartNodeRegistry(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error)) {
	mux := http.NewServeMux()

	nr.srv = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/register", nr.handleRegister)
	mux.HandleFunc("/get", nr.handleGet)
	mux.HandleFunc("/get-all", nr.handleGetAll)

	serveDoneCh := make(chan struct{})

	go func() {
		_ = nr.srv.ListenAndServe()
		close(serveDoneCh)
	}()
	go func() {
		select {
		case <-ctx.Done():
		case <-serveDoneCh:
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		err := nr.srv.Shutdown(shutdownCtx)
		doneCallback(err)
	}()
}

func (nr *NodeRegistry) Shutdown(ctx context.Context) error {
	return nr.srv.Shutdown(ctx)
}

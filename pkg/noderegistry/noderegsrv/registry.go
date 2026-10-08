package noderegsrv

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"meoe.io/poly-csi/pkg/api/node_registry"
)

type Registry struct {
	srv  *http.Server
	mu   sync.RWMutex
	data map[string]node_registry.NodeInfo

	logger *slog.Logger
}

func NewNodeRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		data:   map[string]node_registry.NodeInfo{},
		logger: logger,
	}
}

func (nr *Registry) Set(ctx context.Context, req node_registry.RegisterRequest) node_registry.RegisterResponse {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	nr.data[req.ID] = req.Info
	return node_registry.RegisterResponse{}
}

func (nr *Registry) Get(ctx context.Context, req node_registry.GetRequest) node_registry.GetResponse {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	res, ok := nr.data[req.ID]
	if !ok {
		return node_registry.GetResponse{
			Found: false,
		}
	}
	return node_registry.GetResponse{
		Found: true,
		Info:  res,
	}
}

func (nr *Registry) handleDump(w http.ResponseWriter, req *http.Request) {
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

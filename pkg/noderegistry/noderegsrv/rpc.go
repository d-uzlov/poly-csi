package noderegsrv

import (
	"context"
	"net/http"
	"time"

	"meoe.io/poly-csi/pkg/api/node_registry"
	"meoe.io/poly-csi/pkg/utils/simplerpc"
)

type Server struct {
	srv *http.Server
	Reg *Registry
}

func (nr *Server) Start(ctx context.Context, shutdownTimeout time.Duration, doneCallback func(error)) {
	mux := http.NewServeMux()

	nr.srv = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc(node_registry.RegisterPath, simplerpc.MakeHandler(nr.Reg.Set))
	mux.HandleFunc(node_registry.GetPath, simplerpc.MakeHandler(nr.Reg.Get))
	mux.HandleFunc("/dump", nr.Reg.handleDump)

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

func (nr *Server) Shutdown(ctx context.Context) error {
	return nr.srv.Shutdown(ctx)
}

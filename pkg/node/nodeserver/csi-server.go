package nodeserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/grpc"
	"meoe.io/poly-csi/pkg/node/nodedispatcher"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
)

type Server struct {
	logger *slog.Logger
	tr     transport.Transport
}

func NewNodeServer(ctx context.Context, logger *slog.Logger) *Server {
	tr, err := localexec.New()
	if err != nil {
		panic(err)
	}
	return &Server{
		logger: logger,
		tr:     tr,
	}
}

func Listen(ctx context.Context, address string) (net.Listener, func(), error) {
	pa := strings.SplitN(address, "://", 2)
	if len(pa) != 2 {
		return nil, nil, fmt.Errorf("address format is incorrect, expected schema:// prefix: %s", address)
	}
	proto := pa[0]
	addr := pa[1]
	switch proto {
	case "unix":
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%s: %q", addr, err)
		}
		cleanup := func() {
			_ = os.Remove(addr)
		}
		l, err := net.Listen(proto, addr)
		return l, cleanup, err
	case "tcp":
		l, err := net.Listen(proto, addr)
		return l, nil, err
	default:
		return nil, nil, fmt.Errorf("unknown address protocol: %s", proto)
	}
}

func (s *Server) GoListenAndServe(ctx context.Context, address string, node *nodedispatcher.Node, doneCallback func()) error {
	listener, cleanup, err := Listen(ctx, address)
	if err != nil {
		// s.logger.Error("Failed to listen", "error", err)
		return fmt.Errorf("failed to listen: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(s.logGRPC),
	}
	server := grpc.NewServer(opts...)

	csi.RegisterIdentityServer(server, node)
	csi.RegisterNodeServer(server, node)

	// s.logger.Info("Listening for connections", "address", listener.Addr())

	serveDoneCh := make(chan struct{})

	go func() {
		defer close(serveDoneCh)
		slogctx.FromCtx(ctx).Info("starting to serve", "address", listener.Addr())
		_ = server.Serve(listener)
	}()
	go func() {
		select {
		case <-ctx.Done():
		case <-serveDoneCh:
		}
		server.GracefulStop()
		if cleanup != nil {
			cleanup()
		}
		doneCallback()
	}()

	return nil
}

func (s *Server) logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	logger := s.logger.With("method", info.FullMethod)
	logMethod := logger.Info
	debugLogsByMethod := []string{
		"/csi.v1.Identity/Probe",
		"/csi.v1.Identity/GetPluginInfo",
		"/csi.v1.Node/NodeGetInfo",
		"/csi.v1.Node/NodeGetCapabilities",
		"/csi.v1.Node/NodeGetVolumeStats",
	}
	if slices.Contains(debugLogsByMethod, info.FullMethod) {
		logMethod = logger.Debug
	}

	logMethod("GRPC request", "request", req)
	// protosanitizer doesn't seem to work with slog handlers
	// logMethod("GRPC request", "request", protosanitizer.StripSecrets(req))

	ctx = slogctx.NewCtx(ctx, logger)

	resp, err := handler(ctx, req)
	if err != nil {
		logger.Error("GRPC error", "error", err)
		return nil, fmt.Errorf("%s: %w", info.FullMethod, err)
	}

	logMethod("GRPC success", "response", resp)
	// protosanitizer doesn't seem to work with slog handlers
	// logMethod("GRPC response", "response", protosanitizer.StripSecrets(resp).String())

	return resp, nil
}

package nodeserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"meoe.io/poly-csi/pkg/controller/uuidctx"
	"meoe.io/poly-csi/pkg/logs"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/grpc"
	"meoe.io/poly-csi/pkg/node/nodedispatcher"
)

type Server struct {
	node *nodedispatcher.Node
}

func NewNodeServer(ctx context.Context, node *nodedispatcher.Node) *Server {
	return &Server{
		node: node,
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

func (s *Server) GoListenAndServe(ctx context.Context, address string, doneCallback func()) error {
	listener, cleanup, err := Listen(ctx, address)
	if err != nil {
		// s.logger.Error("Failed to listen", "error", err.Error())
		return fmt.Errorf("failed to listen: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(s.logGRPC),
	}
	server := grpc.NewServer(opts...)

	csi.RegisterIdentityServer(server, s.node)
	csi.RegisterNodeServer(server, s.node)

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

func (s *Server) logGRPC(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (res any, err error) {
	currentConfig := s.node.GetConfig()
	logConfig := currentConfig.GetRpcLogConfig(info.FullMethod)

	callUuid := uuid.NewString()
	logConfig.ExtraFields["uuid"] = callUuid
	ctx = uuidctx.NewCtx(ctx, callUuid)

	logger := logConfig.MakeInstance()
	defer func() {
		logger.Commit(err == nil)
	}()
	ctx = logs.NewCtx(ctx, logger)
	ctx = nodedispatcher.NewCtx(ctx, currentConfig)

	logger.Info("GRPC request", "request", req)

	resp, err := handler(ctx, req)
	if err != nil {
		logger.Error("GRPC error", "error", err.Error())
		return nil, fmt.Errorf("%s: %w", info.FullMethod, err)
	}

	logger.Info("GRPC success", "response", resp)

	return resp, nil
}

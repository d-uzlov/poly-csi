package controllerserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/grpc"
	"meoe.io/poly-csi/pkg/controller"
	"meoe.io/poly-csi/pkg/controller/uuidctx"
	"meoe.io/poly-csi/pkg/logs"
)

type Server struct {
	controller *controller.Controller
}

func NewControllerServer(controller *controller.Controller) *Server {
	return &Server{
		controller: controller,
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

func (s *Server) ListenAndServe(ctx context.Context, address string, doneCallback func()) error {
	listener, cleanup, err := Listen(ctx, address)
	if err != nil {
		// s.logger.Error("Failed to listen", "error", err.Error())
		return fmt.Errorf("failed to listen: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(s.logGRPC),
	}
	server := grpc.NewServer(opts...)

	csi.RegisterIdentityServer(server, s.controller)
	csi.RegisterControllerServer(server, s.controller)
	// csi.RegisterGroupControllerServer(server, controller)

	// s.logger.Info("Listening for connections", "address", listener.Addr())

	serveCh := make(chan struct{})

	go func() {
		slogctx.FromCtx(ctx).Info("starting to serve", "address", listener.Addr())
		err := server.Serve(listener)
		if err != nil {
			slogctx.FromCtx(ctx).Error("serve finished with error", "error", err.Error())
		} else {
			slogctx.FromCtx(ctx).Info("serve finished without errors")
		}
		close(serveCh)
	}()
	go func() {
		select {
		case <-ctx.Done():
		case <-serveCh:
		}
		server.GracefulStop()
		if cleanup != nil {
			cleanup()
		}
		doneCallback()
	}()

	return nil
}

func (s *Server) logGRPC(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	logConfig := s.controller.GetRpcLogConfig(info.FullMethod)

	callUuid := uuid.NewString()
	logConfig.ExtraFields["uuid"] = callUuid
	ctx = uuidctx.NewCtx(ctx, callUuid)

	logger := logConfig.MakeInstance()
	ctx = logs.NewCtx(ctx, logger)

	logger.Info("GRPC request", "request", req)

	resp, err := handler(ctx, req)
	if err != nil {
		logger.Error("GRPC error", "error", err.Error())
		logger.Commit(false)

		return nil, fmt.Errorf("%s: %w", info.FullMethod, err)
	}

	logger.Info("GRPC success", "response", resp)
	logger.Commit(true)

	return resp, nil
}

package controllerserver

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
	"meoe.io/poly-csi/pkg/controller"
)

type Server struct {
	logger *slog.Logger
}

func NewControllerServer(logger *slog.Logger) *Server {
	return &Server{logger: logger}
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

func (s *Server) ListenAndServe(ctx context.Context, address string, controller *controller.Controller, doneCallback func()) error {
	listener, cleanup, err := Listen(ctx, address)
	if err != nil {
		// s.logger.Error("Failed to listen", "error", err)
		return fmt.Errorf("failed to listen: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(s.logGRPC),
	}
	server := grpc.NewServer(opts...)

	csi.RegisterIdentityServer(server, controller)
	csi.RegisterControllerServer(server, controller)
	// csi.RegisterNodeServer(server, ns)
	// csi.RegisterGroupControllerServer(server, controller)
	// csi.RegisterSnapshotMetadataServer(server, sms)

	// s.logger.Info("Listening for connections", "address", listener.Addr())

	serveCh := make(chan struct{})

	go func() {
		slogctx.FromCtx(ctx).Info("starting to serve", "address", listener.Addr())
		err := server.Serve(listener)
		if err != nil {
			slogctx.FromCtx(ctx).Error("serve finished with error", "error", err)
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

func (s *Server) logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	logger := s.logger.With("method", info.FullMethod)
	logMethod := logger.Info
	debugLogsByMethod := []string{
		"/csi.v1.Identity/Probe",
		"/csi.v1.Identity/GetPluginInfo",
		"/csi.v1.Identity/GetPluginCapabilities",
		"/csi.v1.Controller/ControllerGetCapabilities",
		"/csi.v1.Controller/GetCapacity",
	}
	if slices.Contains(debugLogsByMethod, info.FullMethod) {
		logMethod = logger.Debug
	}

	logMethod("GRPC request", "request", req)
	// protosanitizer doesn't seem to work with slog handlers
	// logMethod("GRPC request", "request", protosanitizer.StripSecrets(req))

	resp, err := handler(slogctx.NewCtx(ctx, logger), req)
	if err != nil {
		logger.Error("GRPC error", "error", err)
		return nil, fmt.Errorf("%s: %w", info.FullMethod, err)
	}

	logMethod("GRPC success", "response", resp)
	// protosanitizer doesn't seem to work with slog handlers
	// logMethod("GRPC response", "response", protosanitizer.StripSecrets(resp).String())

	return resp, nil
}

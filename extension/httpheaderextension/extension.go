package httpheaderextension

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type httpheaderExtension struct {
	cfg               *Config
	unaryInterceptor  configauth.GRPCUnaryInterceptorFunc
	streamInterceptor configauth.GRPCStreamInterceptorFunc
	httpInterceptor   configauth.HTTPInterceptorFunc

	logger *zap.Logger
}

var (
	_ configauth.ServerAuthenticator = (*httpheaderExtension)(nil)
)

func newExtension(cfg *Config, logger *zap.Logger) (*httpheaderExtension, error) {
	return &httpheaderExtension{
		cfg:               cfg,
		logger:            logger,
		unaryInterceptor:  configauth.DefaultGRPCUnaryServerInterceptor,
		streamInterceptor: configauth.DefaultGRPCStreamServerInterceptor,
		httpInterceptor:   configauth.DefaultHTTPInterceptor,
	}, nil
}

func (e *httpheaderExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown is invoked during service shutdown.
func (e *httpheaderExtension) Shutdown(context.Context) error {
	return nil
}

// Authenticate checks whether the given context contains valid auth data. Successfully authenticated calls will always return a nil error and a context with the auth data.
func (e *httpheaderExtension) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	cl := client.FromContext(ctx)
	cl.Auth = &authData{
		headers: headers,
	}
	return client.NewContext(ctx, cl), nil
}

// GRPCUnaryServerInterceptor is a helper method to provide a gRPC-compatible UnaryInterceptor, typically calling the authenticator's Authenticate method.
func (e *httpheaderExtension) GRPCUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return e.unaryInterceptor(ctx, req, info, handler, e.Authenticate)
}

// GRPCStreamServerInterceptor is a helper method to provide a gRPC-compatible StreamInterceptor, typically calling the authenticator's Authenticate method.
func (e *httpheaderExtension) GRPCStreamServerInterceptor(srv interface{}, str grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return e.streamInterceptor(srv, str, info, handler, e.Authenticate)
}

// GRPCStreamServerInterceptor is a helper method to provide a gRPC-compatible StreamInterceptor, typically calling the authenticator's Authenticate method.
func (e *httpheaderExtension) HTTPInterceptor(next http.Handler) http.Handler {
	return e.httpInterceptor(next, e.Authenticate)
}

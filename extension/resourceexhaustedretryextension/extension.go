// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/extension/extensionmiddleware"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/resourceexhaustedretryextension/internal/metadata"
)

var (
	_ extensionmiddleware.GRPCServer = (*resourceExhaustedRetryExtension)(nil)
	_ extensionmiddleware.HTTPServer = (*resourceExhaustedRetryExtension)(nil)
)

type resourceExhaustedRetryExtension struct {
	cfg *Config
	tel *metadata.TelemetryBuilder
}

func newExtension(cfg *Config, settings component.TelemetrySettings) (*resourceExhaustedRetryExtension, error) {
	tel, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		return nil, err
	}
	return &resourceExhaustedRetryExtension{cfg: cfg, tel: tel}, nil
}

func (*resourceExhaustedRetryExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *resourceExhaustedRetryExtension) Shutdown(_ context.Context) error {
	e.tel.Shutdown()
	return nil
}

func (e *resourceExhaustedRetryExtension) wrapGRPCError(err error) error {
	if err == nil || e.cfg.RetryDelay == 0 {
		return err
	}
	if consumererror.IsPermanent(err) {
		e.tel.RecordRetryNotSet(metadata.ReasonPermanent)
		return err
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.ResourceExhausted {
		e.tel.RecordRetryNotSet(metadata.ReasonWrongCode)
		return err
	}
	for _, d := range st.Details() {
		if _, ok := d.(*errdetails.RetryInfo); ok {
			e.tel.RecordRetryNotSet(metadata.ReasonRetryInfoPresent)
			return err
		}
	}
	delay := e.cfg.CalculateDelay()
	newSt, err := st.WithDetails(&errdetails.RetryInfo{
		RetryDelay: durationpb.New(delay),
	})
	if err != nil {
		return st.Err()
	}
	e.tel.RecordRetrySet(delay)
	return newSt.Err()
}

// GetGRPCServerOptions implements extensionmiddleware.GRPCServer.
func (e *resourceExhaustedRetryExtension) GetGRPCServerOptions(_ context.Context) ([]grpc.ServerOption, error) {
	if e.cfg.RetryDelay == 0 {
		return nil, nil
	}
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			resp, err := handler(ctx, req)
			return resp, e.wrapGRPCError(err)
		}),
		grpc.ChainStreamInterceptor(func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return e.wrapGRPCError(handler(srv, ss))
		}),
	}, nil
}

// GetHTTPHandler implements extensionmiddleware.HTTPServer.
func (e *resourceExhaustedRetryExtension) GetHTTPHandler(_ context.Context) (extensionmiddleware.WrapHTTPHandlerFunc, error) {
	if e.cfg.RetryDelay == 0 {
		return func(_ context.Context, base http.Handler) (http.Handler, error) {
			return base, nil
		}, nil
	}
	return func(_ context.Context, base http.Handler) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &retryResponseWriter{ResponseWriter: w, delay: e.cfg.CalculateDelay(), resource: e}
			base.ServeHTTP(rw, r)
		}), nil
	}, nil
}

type retryResponseWriter struct {
	http.ResponseWriter
	delay       time.Duration
	resource    *resourceExhaustedRetryExtension
	wroteHeader bool
}

func (rw *retryResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	if code == http.StatusTooManyRequests {
		seconds := int(rw.delay.Seconds())
		if float64(seconds) < rw.delay.Seconds() {
			seconds++
		}
		if seconds < 1 {
			seconds = 1
		}
		rw.Header().Set("Retry-After", strconv.Itoa(seconds))
		rw.resource.tel.RecordRetrySet(rw.delay)
	} else {
		rw.resource.tel.RecordRetryNotSet(metadata.ReasonWrongCode)
	}
	rw.ResponseWriter.WriteHeader(code)
}

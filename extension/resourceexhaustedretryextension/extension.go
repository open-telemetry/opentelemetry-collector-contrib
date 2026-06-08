// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

type resourceExhaustedRetryExtension struct {
	cfg *Config
}

func newExtension(cfg *Config) *resourceExhaustedRetryExtension {
	return &resourceExhaustedRetryExtension{cfg: cfg}
}

func (e *resourceExhaustedRetryExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *resourceExhaustedRetryExtension) Shutdown(_ context.Context) error {
	return nil
}

func (e *resourceExhaustedRetryExtension) effectiveDelay() time.Duration {
	if e.cfg.Jitter == 0 {
		return e.cfg.RetryDelay
	}
	return e.cfg.RetryDelay + time.Duration(rand.Int63n(int64(e.cfg.Jitter)+1))
}

func (e *resourceExhaustedRetryExtension) wrapGRPCError(err error) error {
	if err == nil || e.cfg.RetryDelay == 0 {
		return err
	}
	if consumererror.IsPermanent(err) {
		return err
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.ResourceExhausted {
		return err
	}
	for _, d := range st.Details() {
		if _, ok := d.(*errdetails.RetryInfo); ok {
			return err
		}
	}
	newSt, err := st.WithDetails(&errdetails.RetryInfo{
		RetryDelay: durationpb.New(e.effectiveDelay()),
	})
	if err != nil {
		return st.Err()
	}
	return newSt.Err()
}

// GetGRPCServerOptions implements extensionmiddleware.GRPCServer.
func (e *resourceExhaustedRetryExtension) GetGRPCServerOptions() ([]grpc.ServerOption, error) {
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
func (e *resourceExhaustedRetryExtension) GetHTTPHandler(base http.Handler) (http.Handler, error) {
	if e.cfg.RetryDelay == 0 {
		return base, nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &retryResponseWriter{ResponseWriter: w, delay: e.effectiveDelay()}
		base.ServeHTTP(rw, r)
	}), nil
}

type retryResponseWriter struct {
	http.ResponseWriter
	delay       time.Duration
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
	}
	rw.ResponseWriter.WriteHeader(code)
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/grpc"

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

type Server struct {
	healthpb.UnimplementedHealthServer
	grpcServer            *grpc.Server
	aggregator            *status.Aggregator
	config                *Config
	componentHealthConfig *common.ComponentHealthConfig
	telemetry             component.TelemetrySettings
	doneCh                chan struct{}
	doneOnce              sync.Once
}

var _ component.Component = (*Server)(nil)

func NewServer(
	config *Config,
	componentHealthConfig *common.ComponentHealthConfig,
	telemetry component.TelemetrySettings,
	aggregator *status.Aggregator,
) *Server {
	srv := &Server{
		config:                config,
		componentHealthConfig: componentHealthConfig,
		telemetry:             telemetry,
		aggregator:            aggregator,
		doneCh:                make(chan struct{}),
	}
	if srv.componentHealthConfig == nil {
		srv.componentHealthConfig = &common.ComponentHealthConfig{}
	}
	return srv
}

// Start implements the component.Component interface.
func (s *Server) Start(ctx context.Context, host component.Host) error {
	var err error
	s.grpcServer, err = s.config.ToServer(ctx, host.GetExtensions(), s.telemetry)
	if err != nil {
		return err
	}

	healthpb.RegisterHealthServer(s.grpcServer, s)
	ln, err := s.config.NetAddr.Listen(context.Background())
	if err != nil {
		// Server never started, ensure doneCh is closed so shutdown doesn't block
		s.doneOnce.Do(func() { close(s.doneCh) })
		return err
	}

	go func() {
		defer s.doneOnce.Do(func() { close(s.doneCh) })

		if err = s.grpcServer.Serve(ln); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			componentstatus.ReportStatus(host, componentstatus.NewPermanentErrorEvent(err))
		}
	}()

	return nil
}

// Shutdown implements the component.Component interface.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.grpcServer == nil {
		return nil
	}
	// Stop the server - this will eventually release the port even if context times out
	s.grpcServer.GracefulStop()
	select {
	case <-s.doneCh:
		return nil
	case <-ctx.Done():
		// Context timed out, but server is stopping. Force stop to ensure port is released.
		s.grpcServer.Stop()
		return ctx.Err()
	}
}

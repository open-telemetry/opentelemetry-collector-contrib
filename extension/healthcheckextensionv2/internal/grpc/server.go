// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/grpc"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
)

type Server struct {
	healthpb.UnimplementedHealthServer
	grpcServer            *grpc.Server
	aggregator            *status.Aggregator
	config                *Config
	componentHealthConfig *common.ComponentHealthConfig
	telemetry             component.TelemetrySettings
	doneCh                chan struct{}
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
func (s *Server) Start(_ context.Context, host component.Host) error {
	var err error
	s.grpcServer, err = s.config.ToServer(host, s.telemetry)
	if err != nil {
		return err
	}

	healthpb.RegisterHealthServer(s.grpcServer, s)
	ln, err := s.config.ToListenerContext(context.Background())

	go func() {
		defer close(s.doneCh)

		if err = s.grpcServer.Serve(ln); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			s.telemetry.ReportStatus(component.NewPermanentErrorEvent(err))
		}
	}()

	return nil
}

// Shutdown implements the component.Component interface.
func (s *Server) Shutdown(context.Context) error {
	if s.grpcServer == nil {
		return nil
	}
	s.grpcServer.GracefulStop()
	<-s.doneCh
	return nil
}

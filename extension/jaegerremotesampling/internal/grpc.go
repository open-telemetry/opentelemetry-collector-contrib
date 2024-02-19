// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/jaegertracing/jaeger/cmd/collector/app/sampling"
	"github.com/jaegertracing/jaeger/cmd/collector/app/sampling/strategystore"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var _ component.Component = (*SamplingGRPCServer)(nil)

var errGRPCServerNotRunning = errors.New("gRPC server is not running")

type grpcServer interface {
	Serve(lis net.Listener) error
	GracefulStop()
	Stop()
}

// NewGRPC returns a new sampling gRPC Server.
func NewGRPC(
	telemetry component.TelemetrySettings,
	settings configgrpc.ServerConfig,
	strategyStore strategystore.StrategyStore,
) (*SamplingGRPCServer, error) {
	if strategyStore == nil {
		return nil, errMissingStrategyStore
	}

	return &SamplingGRPCServer{
		telemetry:     telemetry,
		settings:      settings,
		strategyStore: strategyStore,
	}, nil
}

// SamplingGRPCServer implements component.Component to make the life cycle easy to manage.
type SamplingGRPCServer struct {
	telemetry     component.TelemetrySettings
	settings      configgrpc.ServerConfig
	strategyStore strategystore.StrategyStore

	grpcServer grpcServer
	shutdownWG sync.WaitGroup
}

func (s *SamplingGRPCServer) Start(ctx context.Context, host component.Host) error {
	server, err := s.settings.ToServer(host, s.telemetry)
	if err != nil {
		return err
	}
	reflection.Register(server)
	s.grpcServer = server

	api_v2.RegisterSamplingManagerServer(server, sampling.NewGRPCHandler(s.strategyStore))

	healthServer := health.NewServer()
	healthServer.SetServingStatus("jaeger.api_v2.SamplingManager", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	listener, err := s.settings.ToListenerContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port: %w", err)
	}

	s.shutdownWG.Add(1)
	go func() {
		defer s.shutdownWG.Done()

		if err := s.grpcServer.Serve(listener); err != nil {
			s.telemetry.Logger.Error("could not launch gRPC service", zap.Error(err))
		}
	}()

	return nil
}

// Shutdown stops the grps server gracefully.
func (s *SamplingGRPCServer) Shutdown(_ context.Context) error {
	if s.grpcServer == nil {
		return errGRPCServerNotRunning
	}

	s.grpcServer.GracefulStop()

	s.shutdownWG.Wait()

	return nil
}

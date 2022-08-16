// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/jaegertracing/jaeger/cmd/collector/app/sampling"
	"github.com/jaegertracing/jaeger/cmd/collector/app/sampling/strategystore"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
	settings configgrpc.GRPCServerSettings,
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
	settings      configgrpc.GRPCServerSettings
	strategyStore strategystore.StrategyStore

	grpcServer grpcServer
}

func (s *SamplingGRPCServer) Start(_ context.Context, host component.Host) error {
	opts, err := s.settings.ToServerOption(host, s.telemetry)
	if err != nil {
		return err
	}
	server := grpc.NewServer(opts...)
	reflection.Register(server)
	s.grpcServer = server

	api_v2.RegisterSamplingManagerServer(server, sampling.NewGRPCHandler(s.strategyStore))

	healthServer := health.NewServer()
	healthServer.SetServingStatus("jaeger.api_v2.SamplingManager", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	listener, err := s.settings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port: %w", err)
	}

	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			s.telemetry.Logger.Error("could not launch gRPC service", zap.Error(err))
		}
	}()

	return nil
}

// Shutdown tries to terminate connections gracefully as long as the passed context is valid.
func (s *SamplingGRPCServer) Shutdown(ctx context.Context) error {
	if s.grpcServer == nil {
		return errGRPCServerNotRunning
	}

	ch := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		ch <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		s.grpcServer.Stop()
	case <-ch:
	}

	return nil
}

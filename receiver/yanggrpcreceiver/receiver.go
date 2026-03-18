// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/metadata"
	pb "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/proto/generated/proto"
)

type yangReceiver struct {
	config           *Config
	settings         receiver.Settings
	consumer         consumer.Metrics
	server           *grpc.Server
	wg               sync.WaitGroup
	telemetryBuilder *metadata.TelemetryBuilder
	securityManager  *internal.SecurityManager
}

func createMetricsReceiver(_ context.Context, settings receiver.Settings, cfg component.Config, next consumer.Metrics) (receiver.Metrics, error) {
	tb, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return &yangReceiver{
		config:           cfg.(*Config),
		settings:         settings,
		consumer:         next,
		telemetryBuilder: tb,
		wg:               sync.WaitGroup{},
	}, nil
}

func (y *yangReceiver) Start(ctx context.Context, host component.Host) error {
	listener, err := y.config.NetAddr.Listen(ctx)
	if err != nil {
		return err
	}

	y.securityManager = internal.NewSecurityManager(
		y.config.Security.AllowedClients,
		y.settings.Logger,
		y.config.Security.RateLimiting.Enabled,
		y.config.Security.RateLimiting.RequestsPerSecond,
		y.config.Security.RateLimiting.BurstSize,
		y.config.Security.RateLimiting.CleanupInterval,
	)
	server, err := y.config.ToServer(ctx, host.GetExtensions(), y.settings.TelemetrySettings,
		configgrpc.WithGrpcServerOption(grpc.UnaryInterceptor(y.securityManager.CreateSecurityInterceptor())))
	if err != nil {
		return err
	}
	y.server = server

	// Initialize YANG parser with builtin modules
	yangParser := internal.NewYANGParser()
	yangParser.LoadBuiltinModules()

	// Initialize RFC 6020/7950 compliant YANG parser
	rfcYangParser := internal.NewRFC6020Parser()

	// Register the gRPC service for Cisco telemetry
	service := &grpcService{
		receiver:      y,
		yangParser:    yangParser,
		rfcYangParser: rfcYangParser,
	}
	pb.RegisterGRPCMdtDialoutServer(y.server, service)

	y.wg.Go(func() {
		if err := y.server.Serve(listener); err != nil {
			y.settings.Logger.Error("gRPC server error", zap.Error(err))
		}
	})

	return nil
}

func (y *yangReceiver) Shutdown(_ context.Context) error {
	if y.server != nil {
		y.server.GracefulStop()
	}
	y.wg.Wait()

	// Clean up security manager
	if y.securityManager != nil {
		y.securityManager.Shutdown()
	}
	return nil
}

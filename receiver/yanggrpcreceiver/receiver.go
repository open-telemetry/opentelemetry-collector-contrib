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
	pb "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/proto/generated/proto"
)

type yangReceiver struct {
	config          *Config
	settings        receiver.Settings
	logger          *zap.Logger
	consumer        consumer.Metrics
	server          *grpc.Server
	wg              sync.WaitGroup
	securityManager *internal.SecurityManager
}

func createMetricsReceiver(_ context.Context, settings receiver.Settings, cfg component.Config, next consumer.Metrics) receiver.Metrics {
	return &yangReceiver{
		config:   cfg.(*Config),
		settings: settings,
		logger:   settings.Logger,
		consumer: next,
		wg:       sync.WaitGroup{},
	}
}

func (y *yangReceiver) Start(ctx context.Context, host component.Host) error {
	// 1. Setup Network Listener
	listener, err := y.config.NetAddr.Listen(ctx)
	if err != nil {
		return err
	}

	// 2. Initialize Security Management (Rate Limiting & Allowlist)
	y.securityManager = internal.NewSecurityManager(
		y.config.Security.AllowedClients,
		y.settings.Logger,
		y.config.Security.RateLimiting.Enabled,
		y.config.Security.RateLimiting.RequestsPerSecond,
		y.config.Security.RateLimiting.BurstSize,
		y.config.Security.RateLimiting.CleanupInterval,
	)

	// 3. Configure gRPC Server with Security Interceptors
	server, err := y.config.ToServer(ctx, host.GetExtensions(), y.settings.TelemetrySettings,
		configgrpc.WithGrpcServerOption(grpc.UnaryInterceptor(y.securityManager.CreateSecurityInterceptor())))
	if err != nil {
		return err
	}
	y.server = server

	// 4. Initialize YANG Parsers
	// Standard Parser for structural analysis
	yangParser := internal.NewYANGParser()
	yangParser.LoadBuiltinModules()

	// Load external Cisco/IETF modules from configured paths
	// This enables the "No Omission" guarantee for dimensions
	for _, path := range y.config.YANG.ModulePaths {
		y.settings.Logger.Info("Loading YANG modules from path", zap.String("path", path))
		if err := yangParser.ExtractYANGFromFiles(path); err != nil {
			y.settings.Logger.Error("Failed to load YANG modules", zap.String("path", path), zap.Error(err))
		}
	}

	// 5. Register the Dial-out Service
	service := &grpcService{
		receiver:   y,
		yangParser: yangParser,
	}
	pb.RegisterGRPCMdtDialoutServer(y.server, service)

	// 6. Start Serving
	y.wg.Go(func() {
		y.settings.Logger.Info("Starting YANG gRPC receiver",
			zap.String("endpoint", y.config.NetAddr.Endpoint))
		if err := y.server.Serve(listener); err != nil {
			y.settings.Logger.Error("gRPC server error", zap.Error(err))
		}
	})

	return nil
}

func (y *yangReceiver) Shutdown(_ context.Context) error {
	if y.server != nil {
		y.settings.Logger.Info("Stopping YANG gRPC receiver")
		y.server.GracefulStop()
	}
	y.wg.Wait()

	// Clean up security manager
	if y.securityManager != nil {
		y.securityManager.Shutdown()
	}
	return nil
}

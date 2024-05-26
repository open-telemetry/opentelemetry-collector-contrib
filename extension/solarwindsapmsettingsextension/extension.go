// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/solarwindscloud/apm-proto/go/collectorpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type solarwindsapmSettingsExtension struct {
	logger *zap.Logger
	config *Config
	cancel context.CancelFunc
	conn   *grpc.ClientConn
	client collectorpb.TraceCollectorClient
}

func newSolarwindsApmSettingsExtension(extensionCfg *Config, logger *zap.Logger) (extension.Extension, error) {
	settingsExtension := &solarwindsapmSettingsExtension{
		config: extensionCfg,
		logger: logger,
	}
	return settingsExtension, nil
}

func (extension *solarwindsapmSettingsExtension) Start(_ context.Context, _ component.Host) error {
	extension.logger.Info("Starting up solarwinds apm settings extension")
	ctx := context.Background()
	ctx, extension.cancel = context.WithCancel(ctx)
	var err error
	extension.conn, err = grpc.NewClient(extension.config.Endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return err
	}
	extension.logger.Info("Dailed to endpoint", zap.String("endpoint", extension.config.Endpoint))
	extension.client = collectorpb.NewTraceCollectorClient(extension.conn)

	// initial refresh
	refresh(extension)

	go func() {
		ticker := time.NewTicker(extension.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refresh(extension)
			case <-ctx.Done():
				extension.logger.Info("Received ctx.Done() from ticker")
				return
			}
		}
	}()

	return nil
}

func (extension *solarwindsapmSettingsExtension) Shutdown(_ context.Context) error {
	extension.logger.Info("Shutting down solarwinds apm settings extension")
	if extension.cancel != nil {
		extension.cancel()
	}
	if extension.conn != nil {
		return extension.conn.Close()
	}
	return nil
}

func refresh(extension *solarwindsapmSettingsExtension) {
	// Concrete implementation will be available in later PR
	extension.logger.Info("refresh task")
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

import (
	"context"
	"crypto/tls"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"time"

	"github.com/solarwindscloud/apm-proto/go/collectorpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

const (
	RawOutputFile   = "/tmp/solarwinds-apm-settings-raw"
	JSONOutputFile  = "/tmp/solarwinds-apm-settings.json"
	MinimumInterval = "5s"
	MaximumInterval = "60s"
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

func (extension *solarwindsapmSettingsExtension) Start(ctx context.Context, _ component.Host) error {
	extension.logger.Debug("Starting up solarwinds apm settings extension")
	ctx = context.Background()
	ctx, extension.cancel = context.WithCancel(ctx)

	var err error
	extension.conn, err = grpc.Dial(extension.config.Endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return errors.New("Failed to dial: " + err.Error())
	} else {
		extension.logger.Info("Dailed to " + extension.config.Endpoint)
	}
	extension.client = collectorpb.NewTraceCollectorClient(extension.conn)

	task(extension)

	// setup lightweight thread to refresh
	interval, _ := time.ParseDuration(extension.config.Interval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				task(extension)
			case <-ctx.Done():
				extension.logger.Info("Received ctx.Done() from ticker")
				return
			}
		}
	}()

	return nil
}

func (extension *solarwindsapmSettingsExtension) Shutdown(_ context.Context) error {
	extension.logger.Debug("Shutting down solarwinds apm settings extension")
	err := extension.conn.Close()
	if err != nil {
		return errors.New("Failed to close the gRPC connection to solarwinds APM collector " + err.Error())
	}
	return nil
}

func task(extension *solarwindsapmSettingsExtension) {
	if validateSolarwindsApmSettingsExtensionConfiguration(extension.config, extension.logger) {
		refresh(extension)
	} else {
		extension.logger.Warn("No refresh due to invalid config value")
	}
}

func validateSolarwindsApmSettingsExtensionConfiguration(extensionCfg *Config, logger *zap.Logger) bool {
	// Concrete implementation will be available in next PR
	return false
}

func refresh(extension *solarwindsapmSettingsExtension) {
	// Concrete implementation will be available in next PR
}

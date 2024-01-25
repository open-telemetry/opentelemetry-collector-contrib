// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

import (
	"context"
	"crypto/tls"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"time"

	"github.com/solarwindscloud/apm-proto/go/collectorpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
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
	configOk := validateSolarwindsApmSettingsExtensionConfiguration(extension.config, extension.logger)
	if configOk {
		extension.conn, _ = grpc.Dial(extension.config.Endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
		extension.logger.Info("Dailed to " + extension.config.Endpoint)
		extension.client = collectorpb.NewTraceCollectorClient(extension.conn)
		go func() {
			ticker := newTicker(extension.config.Interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					refresh(extension)
				case <-ctx.Done():
					extension.logger.Debug("Received ctx.Done() from ticker")
					return
				}
			}
		}()
	} else {
		extension.logger.Warn("solarwindsapmsettingsextension is in noop. Please check config")
	}
	return nil
}

func (extension *solarwindsapmSettingsExtension) Shutdown(_ context.Context) error {
	extension.logger.Debug("Shutting down solarwinds apm settings extension")
	if extension.conn != nil {
		return extension.conn.Close()
	} else {
		return nil
	}
}

func validateSolarwindsApmSettingsExtensionConfiguration(extensionCfg *Config, logger *zap.Logger) bool {
	// Concrete implementation will be available in later PR
	return true
}

func refresh(extension *solarwindsapmSettingsExtension) {
	// Concrete implementation will be available in later PR
	extension.logger.Info("refresh task")
}

// Start ticking immediately.
// Ref: https://stackoverflow.com/questions/32705582/how-to-get-time-tick-to-tick-immediately
func newTicker(repeat time.Duration) *time.Ticker {
	ticker := time.NewTicker(repeat)
	oc := ticker.C
	nc := make(chan time.Time, 1)
	go func() {
		nc <- time.Now()
		for tm := range oc {
			nc <- tm
		}
	}()
	ticker.C = nc
	return ticker
}

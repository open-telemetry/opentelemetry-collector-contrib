// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter"

import (
	"context"
	"errors"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// BmcHelixExporter is responsible for exporting metrics to BMC Helix
type BmcHelixExporter struct {
	config            *Config
	logger            *zap.Logger
	version           string
	telemetrySettings component.TelemetrySettings
	metricsProducer   *MetricsProducer
	metricsClient     *MetricsClient
}

// newBmcHelixExporter instantiates a new BMC Helix Exporter
func newBmcHelixExporter(config *Config, createSettings exporter.Settings) (*BmcHelixExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}

	return &BmcHelixExporter{
		config:            config,
		version:           createSettings.BuildInfo.Version,
		logger:            createSettings.Logger,
		telemetrySettings: createSettings.TelemetrySettings,
	}, nil
}

// pushMetrics is invoked by the OpenTelemetry Collector to push metrics to BMC Helix
func (be *BmcHelixExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	be.logger.Info("Building BMC Helix payload")
	helixMetrics, err := be.metricsProducer.ProduceHelixPayload(md)
	if err != nil {
		be.logger.Error("Failed to build BMC Helix payload", zap.Error(err))
		return err
	}

	be.logger.Info("Sending BMC Helix payload")
	err = be.metricsClient.SendHelixPayload(ctx, helixMetrics)
	if err != nil {
		be.logger.Error("Failed to send BMC Helix payload", zap.Error(err))
		return err
	}

	return nil
}

// start is invoked during service start
func (be *BmcHelixExporter) start(ctx context.Context, host component.Host) error {
	be.logger.Info("Starting BMC Helix Exporter")

	// Get the hostname reported by the kernel
	osHostname, err := os.Hostname()
	if err != nil {
		be.logger.Error("Failed to get OS hostname", zap.Error(err))
		return err
	}

	// Initialize and store the MetricsProducer
	be.metricsProducer = newMetricsProducer(osHostname, be.logger)

	// Initialize and store the MetricsClient
	metricsClient, err := newMetricsClient(ctx, be.config, host, be.telemetrySettings, be.logger)
	if err != nil {
		be.logger.Error("Failed to create MetricsClient", zap.Error(err))
		return err
	}
	be.metricsClient = metricsClient

	be.logger.Info("Initialized BMC Helix Exporter")
	return nil
}

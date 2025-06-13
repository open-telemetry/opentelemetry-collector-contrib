// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	om "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter/internal/operationsmanagement"
)

// metricsExporter is responsible for exporting metrics to BMC Helix
type metricsExporter struct {
	config            *Config
	logger            *zap.Logger
	version           string
	telemetrySettings component.TelemetrySettings
	producer          *om.MetricsProducer
	client            *om.MetricsClient
}

// newMetricsExporter instantiates a new metrics exporter for BMC Helix
func newMetricsExporter(config *Config, createSettings exporter.Settings) (*metricsExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}

	return &metricsExporter{
		config:            config,
		version:           createSettings.BuildInfo.Version,
		logger:            createSettings.Logger,
		telemetrySettings: createSettings.TelemetrySettings,
	}, nil
}

// pushMetrics is invoked by the OpenTelemetry Collector to push metrics to BMC Helix
func (me *metricsExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	helixMetrics, err := me.producer.ProduceHelixPayload(md)
	if err != nil {
		me.logger.Error("Failed to build BMC Helix Metrics payload", zap.Error(err))
		return err
	}

	err = me.client.SendHelixPayload(ctx, helixMetrics)
	if err != nil {
		me.logger.Error("Failed to send BMC Helix Metrics payload", zap.Error(err))
		return err
	}

	return nil
}

// start is invoked during service start
func (me *metricsExporter) start(ctx context.Context, host component.Host) error {
	me.logger.Info("Starting BMC Helix Metrics Exporter")

	// Initialize and store the MetricsProducer
	me.producer = om.NewMetricsProducer(me.logger)

	// Initialize and store the MetricsClient
	client, err := om.NewMetricsClient(ctx, me.config.ClientConfig, me.config.APIKey, host, me.telemetrySettings, me.logger)
	if err != nil {
		me.logger.Error("Failed to create MetricsClient", zap.Error(err))
		return err
	}
	me.client = client

	me.logger.Info("Initialized BMC Helix Metrics Exporter")
	return nil
}

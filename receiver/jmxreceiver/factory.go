// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/metadata"
)

const (
	otlpEndpoint = "0.0.0.0:0"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
		CollectionInterval: 10 * time.Second,
		OTLPExporterConfig: otlpExporterConfig{
			Endpoint: otlpEndpoint,
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 5 * time.Second,
			},
		},
	}
}

func createReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	jmxConfig := cfg.(*Config)
	return newJMXMetricReceiver(params, jmxConfig, consumer), nil
}

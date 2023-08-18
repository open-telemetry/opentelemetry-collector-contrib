// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"context"

	promconfig "github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install" // init() of this package registers service discovery impl.
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

// This file implements config for Prometheus receiver.
var useCreatedMetricGate = featuregate.GlobalRegistry().MustRegister(
	"receiver.prometheusreceiver.UseCreatedMetric",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the Prometheus receiver will"+
		" retrieve the start time for Summary, Histogram and Sum metrics from _created metric"),
)

// NewFactory creates a new Prometheus receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		PrometheusConfig: &promconfig.Config{
			GlobalConfig: promconfig.DefaultGlobalConfig,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	configWarnings(set.Logger, cfg.(*Config))
	return newPrometheusReceiver(set, cfg.(*Config), nextConsumer), nil
}

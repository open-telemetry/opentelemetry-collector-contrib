// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr        = "awscloudwatchmetrics"
	stabilityLevel = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for awscloudwatchmetrics receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stabilityLevel),
	)
}

func createMetricsReceiver(_ context.Context, params receiver.CreateSettings, baseCfg component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)
	rcvr := newMetricReceiver(cfg, params.Logger, consumer)
	return rcvr, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		PollInterval: defaultPollInterval,
		Metrics:      &MetricsConfig{},
	}
}

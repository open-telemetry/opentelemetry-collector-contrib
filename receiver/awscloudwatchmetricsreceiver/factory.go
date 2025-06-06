// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver/internal/metadata"
)

const (
	typeStr   = "awscloudwatchmetrics"
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for awscloudwatchmetrics receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	config := cfg.(*Config)
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	receiver, err := newMetricReceiver(config, settings.Logger, consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics receiver: %w", err)
	}

	return receiver, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		PollInterval: defaultPollInterval,
		Metrics:      &MetricsConfig{},
	}
}

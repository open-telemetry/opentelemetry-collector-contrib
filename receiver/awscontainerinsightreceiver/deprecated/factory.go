// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deprecated // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/deprecated"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"
)

const (
	// The value of "type" key in configuration for the deprecated receiver name.
	deprecatedTypeStr = "awscontainerinsightreceiver"
)

var (
	deprecatedType = component.MustNewType(deprecatedTypeStr)
)

// NewFactory creates a factory for the deprecated AWS container insight receiver.
// Deprecated: [v0.140.0] Use awscontainerinsight instead.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		deprecatedType,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelBeta))
}

// createDefaultConfig returns a default config for the receiver.
func createDefaultConfig() component.Config {
	// Use the createDefaultConfig from the parent package
	return awscontainerinsightreceiver.CreateDefaultConfig()
}

// createMetricsReceiver creates a deprecated AWS Container Insight receiver that logs a deprecation warning on start.
func createMetricsReceiver(
	ctx context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	// Create the receiver using the parent package's implementation
	recv, err := awscontainerinsightreceiver.CreateMetricsReceiver(ctx, params, baseCfg, consumer)
	if err != nil {
		return nil, err
	}

	// Wrap it to add deprecation warning on start
	return &deprecatedReceiverWrapper{
		Metrics: recv,
		logger:  params.Logger,
	}, nil
}

// deprecatedReceiverWrapper wraps the receiver to log deprecation warning on start.
type deprecatedReceiverWrapper struct {
	receiver.Metrics
	logger *zap.Logger
}

// Start logs the deprecation warning and then starts the wrapped receiver.
func (w *deprecatedReceiverWrapper) Start(ctx context.Context, host component.Host) error {
	w.logger.Warn("The receiver type 'awscontainerinsightreceiver' is deprecated and will be removed in a future version. Use 'awscontainerinsight' instead.")
	return w.Metrics.Start(ctx, host)
}

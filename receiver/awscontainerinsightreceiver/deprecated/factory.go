// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deprecated // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/deprecated"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"
)

const (
	// The value of "type" key in configuration for the deprecated receiver name.
	deprecatedTypeStr = "awscontainerinsightreceiver"

	// Default collection interval. Every 60s the receiver will collect metrics
	defaultCollectionInterval = 60 * time.Second

	// Default container orchestrator service is aws eks
	defaultContainerOrchestrator = "eks"

	// Metrics is tagged with service name by default
	defaultTagService = true

	// Don't use pod full name by default (as the full names contain suffix with random characters)
	defaultPrefFullPodName = false

	// Don't tag pod full name by default
	defaultAddFullPodNameMetricLabel = false
)

var deprecatedType = component.MustNewType(deprecatedTypeStr)

// NewFactory creates a factory for the deprecated AWS container insight receiver.
// Deprecated: [v0.140.0] Use awscontainerinsight instead.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		deprecatedType,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelBeta))
}

// createDefaultConfig returns a default config for the receiver.
// This duplicates the config creation from the main package.
func createDefaultConfig() component.Config {
	return &awscontainerinsightreceiver.Config{
		CollectionInterval:        defaultCollectionInterval,
		ContainerOrchestrator:     defaultContainerOrchestrator,
		TagService:                defaultTagService,
		PrefFullPodName:           defaultPrefFullPodName,
		AddFullPodNameMetricLabel: defaultAddFullPodNameMetricLabel,
	}
}

// createMetricsReceiver creates a deprecated AWS Container Insight receiver that logs a deprecation warning on start.
// This creates the receiver directly to avoid type mismatch issues.
func createMetricsReceiver(
	ctx context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	// Create the receiver using the main package's factory, but we need to ensure
	// the params have the correct type. The issue is that the main factory creates
	// receivers that check the component ID type matches the factory type.
	// Since we can't access the unexported receiver struct, we use the main factory
	// but update the params to have the new type temporarily, then wrap it.
	mainFactory := awscontainerinsightreceiver.NewFactory()

	// Temporarily change the component ID type to match the main factory's type
	// This is needed because the receiver created by the main factory validates
	// that the component ID type matches the factory type.
	originalID := params.ID
	params.ID = component.NewID(mainFactory.Type())

	recv, err := mainFactory.CreateMetrics(ctx, params, baseCfg, consumer)
	if err != nil {
		return nil, err
	}

	// Restore the original ID (though it won't be used after this)
	params.ID = originalID

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

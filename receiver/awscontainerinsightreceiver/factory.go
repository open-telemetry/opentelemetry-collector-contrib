// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/metadata"
)

// Factory for awscontainerinsightreceiver
const (
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

// NewFactory creates a factory for AWS container insight receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

// NewDeprecatedFactory creates a factory for the deprecated AWS container insight receiver.
// This function is exported so the builder can discover it and include it in components.go.
// Deprecated: [v0.140.0] Use awscontainerinsight instead.
func NewDeprecatedFactory() receiver.Factory {
	deprecatedType := component.MustNewType("awscontainerinsightreceiver")
	return receiver.NewFactory(
		deprecatedType,
		createDefaultConfig,
		receiver.WithMetrics(createDeprecatedMetricsReceiver, component.StabilityLevelBeta))
}

// createDeprecatedMetricsReceiver creates a deprecated AWS Container Insight receiver that logs a deprecation warning on start.
func createDeprecatedMetricsReceiver(
	ctx context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	// Create the receiver using the standard implementation
	recv, err := CreateMetricsReceiver(ctx, params, baseCfg, consumer)
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

// createDefaultConfig returns a default config for the receiver.
func createDefaultConfig() component.Config {
	return CreateDefaultConfig()
}

// CreateDefaultConfig returns a default config for the receiver.
// This is exported for use by the deprecated subpackage.
func CreateDefaultConfig() component.Config {
	return &Config{
		CollectionInterval:        defaultCollectionInterval,
		ContainerOrchestrator:     defaultContainerOrchestrator,
		TagService:                defaultTagService,
		PrefFullPodName:           defaultPrefFullPodName,
		AddFullPodNameMetricLabel: defaultAddFullPodNameMetricLabel,
	}
}

// createMetricsReceiver creates an AWS Container Insight receiver.
func createMetricsReceiver(
	ctx context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	return CreateMetricsReceiver(ctx, params, baseCfg, consumer)
}

// CreateMetricsReceiver creates an AWS Container Insight receiver.
// This is exported for use by the deprecated subpackage.
func CreateMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	rCfg := baseCfg.(*Config)
	return newAWSContainerInsightReceiver(params.TelemetrySettings, rCfg, consumer)
}

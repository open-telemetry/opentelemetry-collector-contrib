// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// Factory for awscontainerinsightreceiver
const (
	// Key to invoke this receiver
	typeStr = "awscontainerinsightreceiver"

	// The stability of this receiver
	stability = component.StabilityLevelBeta

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
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

// createDefaultConfig returns a default config for the receiver.
func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval:        defaultCollectionInterval,
		ContainerOrchestrator:     defaultContainerOrchestrator,
		TagService:                defaultTagService,
		PrefFullPodName:           defaultPrefFullPodName,
		AddFullPodNameMetricLabel: defaultAddFullPodNameMetricLabel,
	}
}

// CreateMetricsReceiver creates an AWS Container Insight receiver.
func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {

	rCfg := baseCfg.(*Config)
	return newAWSContainerInsightReceiver(params.TelemetrySettings, rCfg, consumer)
}

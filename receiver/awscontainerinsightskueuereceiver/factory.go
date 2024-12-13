// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightskueuereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	kueueMetricsStability = component.StabilityLevelDevelopment
)

var receiverType component.Type = component.MustNewType("awscontainerinsightskueuereceiver")

// Factory for awscontainerinsightreceiver
const (
	// Default collection interval. Every 60s the receiver will collect metrics
	defaultCollectionInterval = 60 * time.Second

	// Rely on EC2 tags to auto-detect cluster name by default
	defaultClusterName = ""
)

// NewFactory creates a factory for AWS container insight receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		receiverType,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, kueueMetricsStability))
}

// createDefaultConfig returns a default config for the receiver.
func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval: defaultCollectionInterval,
		ClusterName:        defaultClusterName,
	}
}

// CreateMetricsReceiver creates an AWS Container Insight receiver.
func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	rCfg := baseCfg.(*Config)
	return newAWSContainerInsightReceiver(params.TelemetrySettings, rCfg, consumer)
}

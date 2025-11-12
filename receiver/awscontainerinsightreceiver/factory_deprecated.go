// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/metadata"
)

var (
	deprecatedType = component.MustNewType("awscontainerinsightreceiver")
	deprecationOnce sync.Once
)

// NewDeprecatedFactory creates a factory for the deprecated awscontainerinsightreceiver type.
// This factory logs a deprecation warning and delegates to the main factory implementation.
// Deprecated: Use NewFactory() instead. The component type 'awscontainerinsightreceiver' is deprecated
// in favor of 'awscontainerinsight' and will be removed in a future version.
//
// NOTE: This factory must be registered in the builder configuration or components.go file
// to enable backward compatibility with existing configurations using 'awscontainerinsightreceiver'.
func NewDeprecatedFactory() receiver.Factory {
	return receiver.NewFactory(
		deprecatedType,
		createDefaultConfig,
		receiver.WithMetrics(createDeprecatedMetricsReceiver, metadata.MetricsStability))
}

func createDeprecatedMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	deprecationOnce.Do(func() {
		params.Logger.Warn(
			"The component type 'awscontainerinsightreceiver' is deprecated and will be removed in a future version. " +
				"Please use 'awscontainerinsight' instead.",
			zap.String("old_type", "awscontainerinsightreceiver"),
			zap.String("new_type", "awscontainerinsight"),
		)
	})

	// Delegate to the main implementation
	rCfg := baseCfg.(*Config)
	return newAWSContainerInsightReceiver(params.TelemetrySettings, rCfg, consumer)
}


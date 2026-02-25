// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/metadata"
)

var useNewTypeNameGate = featuregate.GlobalRegistry().MustRegister(
	"receiver.awscontainerinsightreceiver.useNewTypeName",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the receiver uses the new component type name 'awscontainerinsight' instead of 'awscontainerinsightreceiver'."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/44052"),
)

// Factory for awscontainerinsight
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
	var componentType component.Type
	if useNewTypeNameGate.IsEnabled() {
		componentType = component.MustNewType("awscontainerinsight")
	} else {
		componentType = component.MustNewType("awscontainerinsightreceiver")
	}

	return receiver.NewFactory(
		componentType,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
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

// createMetricsReceiver creates an AWS Container Insight receiver.
func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	if !useNewTypeNameGate.IsEnabled() {
		params.Logger.Warn(
			"The component type name 'awscontainerinsightreceiver' is deprecated and will be changed to 'awscontainerinsight' in a future release. " +
				"Please enable the feature gate 'receiver.awscontainerinsightreceiver.useNewTypeName' to use the new component type name. " +
				"See: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/44052",
		)
	}
	rCfg := baseCfg.(*Config)
	return newAWSContainerInsightReceiver(params.TelemetrySettings, rCfg, consumer)
}

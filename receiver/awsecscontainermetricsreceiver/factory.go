// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/endpoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/metadata"
)

// Factory for awscontainermetrics
const (
	// Default collection interval. Every 20s the receiver will collect metrics from Amazon ECS Task Metadata Endpoint
	defaultCollectionInterval = 20 * time.Second
)

// NewFactory creates a factory for AWS ECS Container Metrics receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

// createDefaultConfig returns a default config for the receiver.
func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval: defaultCollectionInterval,
	}
}

// CreateMetricsReceiver creates an AWS ECS Container Metrics receiver.
func createMetricsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	endpoint, err := endpoints.GetTMEV4FromEnv()
	if err != nil || endpoint == nil {
		return nil, fmt.Errorf("unable to detect task metadata endpoint: %w", err)
	}
	clientSettings := confighttp.HTTPClientSettings{}
	rest, err := ecsutil.NewRestClient(*endpoint, clientSettings, params.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	rCfg := baseCfg.(*Config)
	logger := params.Logger
	return newAWSECSContainermetrics(logger, rCfg, consumer, rest)
}

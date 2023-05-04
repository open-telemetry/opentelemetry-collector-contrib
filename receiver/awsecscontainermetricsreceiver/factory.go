// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
)

// Factory for awscontainermetrics
const (
	// Key to invoke this receiver (awsecscontainermetrics)
	typeStr = "awsecscontainermetrics"

	// Stability level of the receiver
	stability = component.StabilityLevelBeta

	// Default collection interval. Every 20s the receiver will collect metrics from Amazon ECS Task Metadata Endpoint
	defaultCollectionInterval = 20 * time.Second
)

// NewFactory creates a factory for AWS ECS Container Metrics receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
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

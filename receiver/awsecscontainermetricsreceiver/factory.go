// Copyright 2020, OpenTelemetry Authors
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

package awsecscontainermetricsreceiver

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"
)

// Factory for awscontainermetrics
const (
	// Key to invoke this receiver (awsecscontainermetrics)
	typeStr = "awsecscontainermetrics"

	// Default collection interval. Every 20s the receiver will collect metrics from Amazon ECS Task Metadata Endpoint
	defaultCollectionInterval = 20 * time.Second
)

// NewFactory creates a factory for AWS ECS Container Metrics receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

// createDefaultConfig returns a default config for the receiver.
func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
		CollectionInterval: defaultCollectionInterval,
	}
}

// CreateMetricsReceiver creates an AWS ECS Container Metrics receiver.
func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateSettings,
	baseCfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	ecsTaskMetadataEndpointV4 := os.Getenv(awsecscontainermetrics.EndpointEnvKey)
	if ecsTaskMetadataEndpointV4 == "" {
		return nil, fmt.Errorf("no environment variable found for %s", awsecscontainermetrics.EndpointEnvKey)
	}

	endpoint, err := url.ParseRequestURI(ecsTaskMetadataEndpointV4)
	if err != nil {
		return nil, err
	}
	rest := restClient(params.Logger, *endpoint)

	rCfg := baseCfg.(*Config)
	logger := params.Logger
	return newAWSECSContainermetrics(logger, rCfg, consumer, rest)
}

func restClient(logger *zap.Logger, endpoint url.URL) awsecscontainermetrics.RestClient {
	clientProvider := awsecscontainermetrics.NewClientProvider(endpoint, logger)

	client := clientProvider.BuildClient()
	rest := awsecscontainermetrics.NewRestClient(client)

	return rest
}

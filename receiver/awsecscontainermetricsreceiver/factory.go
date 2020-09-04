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
	"os"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/awsecscontainermetrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

// Factory for awscontainermetrics
const (
	// Key to invoke this receiver (awsecscontainermetrics)
	typeStr = "awsecscontainermetrics"

	// Default collection interval. In every 20s the receiver will collect metrics from Amazon ECS Task Metadata Endpoint
	defaultCollectionInterval = 20 * time.Second
)

// NewFactory creates a factory for Aws ECS Container Metrics receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

// CreateDefaultConfig returns a default config for the receiver.
func createDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		CollectionInterval: defaultCollectionInterval,
	}
}

// CreateTraceReceiver returns error as trace receiver is not applicable to aws ecs container metrics receiver.
func createTraceReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	nextConsumer consumer.TraceConsumer,
) (component.TraceReceiver, error) {
	// Amazon ECS Task Metadata Endpoint does not support traces.
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsReceiver creates an AWS ECS Container Metrics receiver.
func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateParams,
	baseCfg configmodels.Receiver,
	consumer consumer.MetricsConsumer,
) (component.MetricsReceiver, error) {
	ecsTaskMetadataEndpointV4 := os.Getenv("ECS_CONTAINER_METADATA_URI_V4")
	if ecsTaskMetadataEndpointV4 == "" {
		return nil, fmt.Errorf("Could not get environment variable- ECS_CONTAINER_METADATA_URI_V4")
	}

	rest, err := restClient(params.Logger, ecsTaskMetadataEndpointV4)
	if err != nil {
		return nil, err
	}
	rCfg := baseCfg.(*Config)
	return New(params.Logger, rCfg, consumer, rest)
}

func restClient(logger *zap.Logger, endpoint string) (awsecscontainermetrics.RestClient, error) {
	clientProvider, err := awsecscontainermetrics.NewClientProvider(endpoint, logger)
	if err != nil {
		return nil, err
	}
	client, err := clientProvider.BuildClient()
	if err != nil {
		return nil, err
	}
	rest := awsecscontainermetrics.NewRestClient(client)
	return rest, nil
}

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
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/awsecscontainermetrics"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

const (
	typeStr = "awsecscontainermetrics"

	// Default collection interval
	defaultCollectionInterval = 20 * time.Second
)

// NewFactory creates a factory for Aws ECS Container Metrics receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

var _ component.ReceiverFactoryBase = (*Factory)(nil)

// Factory creates an aws ecs container metrics receiver.
type Factory struct {
}

// Type returns the type of this factory, "awsecscontainermetrics".
func (f *Factory) Type() configmodels.Type {
	return typeStr
}

// CustomUnmarshaler returns nil because we don't need custom unmarshaling for this config.
func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

// CreateDefaultConfig creates a default config.
func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		CollectionInterval: defaultCollectionInterval,
	}
}

// CreateTraceReceiver returns error as trace receiver is not applicable to aws ecs container metrics receiver.
func (f *Factory) CreateTraceReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	nextConsumer consumer.TraceConsumerOld,
) (component.TraceReceiver, error) {
	// Amazon ECS Task Metadata Endpoint does not support traces.
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsReceiver creates an aws ecs container metrics receiver.
func (f *Factory) CreateMetricsReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	nextConsumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {
	rest, err := f.restClient(logger, "https://jsonplaceholder.typicode.com/todos/1")
	if err != nil {
		return nil, err
	}
	rCfg := cfg.(*Config)
	return New(logger, rCfg, nextConsumer, rest)
}

func (f *Factory) restClient(logger *zap.Logger, endpoint string) (awsecscontainermetrics.RestClient, error) {
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

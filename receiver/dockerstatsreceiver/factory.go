// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dockerstatsreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

const (
	typeStr = "docker_stats"
)

var _ component.ReceiverFactoryOld = (*Factory)(nil)

type Factory struct{}

func (f *Factory) Type() configmodels.Type {
	return typeStr
}

func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		Endpoint:           "unix:///var/run/docker.sock",
		CollectionInterval: 10 * time.Second,
		Timeout:            5 * time.Second,
	}
}

func (f *Factory) CreateTraceReceiver(
	ctx context.Context,
	logger *zap.Logger,
	config configmodels.Receiver,
	nextConsumer consumer.TraceConsumerOld,
) (component.TraceReceiver, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

func (f *Factory) CreateMetricsReceiver(
	ctx context.Context,
	logger *zap.Logger,
	config configmodels.Receiver,
	nextConsumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {
	cfg := config.(*Config)

	dsr, err := NewReceiver(ctx, cfg, logger, nextConsumer)
	if err != nil {
		return nil, err
	}

	return dsr, nil
}

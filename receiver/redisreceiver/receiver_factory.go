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

package redisreceiver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configerror"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"go.uber.org/zap"
)

const (
	typeStr = "redis"
)

var _ component.ReceiverFactoryOld = (*Factory)(nil)

// Factory creates a RedisReceiver.
type Factory struct {
}

func (f Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

// Type returns the type of this factory, "redis".
func (f Factory) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

// CreateDefaultConfig creates a default config.
func (f Factory) CreateDefaultConfig() configmodels.Receiver {
	return &config{}
}

// CreateTraceReceiver creates a trace Receiver. Not supported for now.
func (f Factory) CreateTraceReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	nextConsumer consumer.TraceConsumerOld,
) (component.TraceReceiver, error) {
	// No trace receiver for now.
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsReceiver creates a Redis metrics receiver.
func (f Factory) CreateMetricsReceiver(
	logger *zap.Logger,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {
	return newRedisReceiver(logger, cfg.(*config), consumer), nil
}

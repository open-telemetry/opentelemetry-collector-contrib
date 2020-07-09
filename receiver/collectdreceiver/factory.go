// Copyright 2019, OpenTelemetry Authors
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

package collectdreceiver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

// This file implements factory for CollectD receiver.

const (
	typeStr               = "collectd"
	defaultBindEndpoint   = "localhost:8081"
	defaultTimeout        = time.Duration(time.Second * 30)
	defaultEncodingFormat = "json"
)

// Factory is the factory for collectd receiver.
type Factory struct {
}

var _ component.ReceiverFactoryOld = &Factory{}

// Type gets the type of the Receiver config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

// CustomUnmarshaler returns nil because we don't need custom unmarshaling for this config.
func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

// CreateDefaultConfig creates the default configuration for CollectD receiver.
func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		Endpoint: defaultBindEndpoint,
		Timeout:  defaultTimeout,
		Encoding: defaultEncodingFormat,
	}
}

// CreateTraceReceiver creates a trace receiver based on provided config.
func (f *Factory) CreateTraceReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	nextConsumer consumer.TraceConsumerOld,
) (component.TraceReceiver, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsReceiver creates a metrics receiver based on provided config.
func (f *Factory) CreateMetricsReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	nextConsumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {
	c := cfg.(*Config)
	c.Encoding = strings.ToLower(c.Encoding)
	// CollectD receiver only supports JSON encoding. We expose a config option
	// to make it explicit and obvious to the users.
	if c.Encoding != defaultEncodingFormat {
		return nil, fmt.Errorf(
			"CollectD only support JSON encoding format. %s is not supported",
			c.Encoding,
		)
	}
	return New(logger, c.Endpoint, c.Timeout, c.AttributesPrefix, nextConsumer)
}

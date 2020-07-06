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

package cudareceiver

import (
	"context"
	"errors"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

// This file implements config for CUDA receiver.

const (
	// The value of "type" key in configuration.
	typeStr = "cudametrics"
)

// Factory is the Factory for receiver.
type Factory struct{}

// Type gets the type of the Receiver config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return typeStr
}

// CustomUnmarshaler returns custom unmarshaler for this config.
func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

// CreateDefaultConfig creates the default configuration for receiver.
func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
	}
}

// CreateTraceReceiver creates a trace receiver based on provided config.
func (f *Factory) CreateTraceReceiver(ctx context.Context, logger *zap.Logger, cfg configmodels.Receiver, nextConsumer consumer.TraceConsumerOld) (component.TraceReceiver, error) {
	// CUDA metrics does not support traces
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsReceiver creates a metrics receiver based on provided config.
//
// TODO: ConsumeMetricsOld will be deprecated and should be replaced with ConsumerMetrics.
// NOTE: The 2nd argument type of ConsumerMetrics#ConsumeMetrics is pdata.Metrics, which can NOT
// be handled from out of core repository, so keep using ConsumerMetricsOld here until
// it'll be accessible from contrib repo.
// https://pkg.go.dev/go.opentelemetry.io/collector/consumer/pdata?tab=doc#Metrics
func (f *Factory) CreateMetricsReceiver(ctx context.Context, logger *zap.Logger, cfg configmodels.Receiver, nextConsumer consumer.MetricsConsumerOld) (component.MetricsReceiver, error) {
	if runtime.GOOS != "linux" {
		// TODO: consider the support for other platforms.
		return nil, errors.New("cudametrics receiver is only supported on linux")
	}
	rcfg := cfg.(*Config)

	cudac, err := NewCUDAMetricsCollector(rcfg.ScrapeInterval, rcfg.MetricPrefix, logger, nextConsumer)
	if err != nil {
		return nil, err
	}

	cudar := &Receiver{
		c:      cudac,
		logger: logger,
	}
	return cudar, nil
}

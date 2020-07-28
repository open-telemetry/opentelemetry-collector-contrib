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

package prometheusexecreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

// Factory for prometheusexec
const (
	// Key to invoke this receiver (prometheus_exec)
	typeStr = "prometheus_exec"

	defaultMetricsPath        = "/metrics"
	defaultCollectionInterval = 60 * time.Second
	defaultScrapeTimeout      = 10 * time.Second
)

// Factory is the factory struct for prometheusexec
type Factory struct{}

// Type returns the type of receiver config this factory creates
func (f *Factory) Type() configmodels.Type {
	return typeStr
}

// CustomUnmarshaler returns nil since there is no need for a custom unmarshaler
func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return nil
}

// CreateDefaultConfig returns a default config
func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		ScrapeInterval: defaultCollectionInterval,
		SubprocessConfig: subprocessmanager.SubprocessConfig{
			Command: "",
			Env:     []subprocessmanager.EnvConfig{},
		},
	}
}

// CreateTraceReceiver creates a trace receiver based on provided Config, BUT in this case it returns nil since this receiver only support metrics
func (f *Factory) CreateTraceReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	consumer consumer.TraceConsumerOld,
) (component.TraceReceiver, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

// CreateMetricsReceiver creates a metrics receiver based on provided Config.
func (f *Factory) CreateMetricsReceiver(
	ctx context.Context,
	logger *zap.Logger,
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {
	rCfg := cfg.(*Config)
	return new(logger, rCfg, consumer), nil
}

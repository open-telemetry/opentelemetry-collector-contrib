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

package prometheusexecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

// Factory for prometheusexec
const (
	// Key to invoke this receiver (prometheus_exec)
	typeStr = "prometheus_exec"

	defaultCollectionInterval = 60 * time.Second
	defaultTimeoutInterval    = 10 * time.Second
)

// NewFactory creates a factory for the prometheusexec receiver
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver))
}

// createDefaultConfig returns a default config
func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		ScrapeInterval:   defaultCollectionInterval,
		ScrapeTimeout:    defaultTimeoutInterval,
		SubprocessConfig: subprocessmanager.SubprocessConfig{
			Env: []subprocessmanager.EnvConfig{},
		},
	}
}

// createMetricsReceiver creates a metrics receiver based on provided Config.
func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	rCfg := cfg.(*Config)
	return newPromExecReceiver(params, rCfg, nextConsumer)
}

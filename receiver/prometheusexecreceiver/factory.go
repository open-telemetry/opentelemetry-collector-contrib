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

package prometheusexecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

// Factory for prometheusexec
const (
	// Key to invoke this receiver (prometheus_exec)
	typeStr   = "prometheus_exec"
	stability = component.StabilityLevelDeprecated

	defaultCollectionInterval = 60 * time.Second
	defaultTimeoutInterval    = 10 * time.Second
)

var once sync.Once

// NewFactory creates a factory for the prometheusexec receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

func logDeprecation(logger *zap.Logger) {
	once.Do(func() {
		logger.Warn("prometheus_exec receiver is deprecated and will be removed in future versions.")
	})
}

// createDefaultConfig returns a default config
func createDefaultConfig() component.Config {
	return &Config{
		ScrapeInterval: defaultCollectionInterval,
		ScrapeTimeout:  defaultTimeoutInterval,
		SubprocessConfig: subprocessmanager.SubprocessConfig{
			Env: []subprocessmanager.EnvConfig{},
		},
	}
}

// createMetricsReceiver creates a metrics receiver based on provided Config.
func createMetricsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	logDeprecation(params.Logger)
	rCfg := cfg.(*Config)
	return newPromExecReceiver(params, rCfg, nextConsumer), nil
}

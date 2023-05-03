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

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr      = "jmx"
	stability    = component.StabilityLevelAlpha
	otlpEndpoint = "0.0.0.0:0"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createReceiver, stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
		CollectionInterval: 10 * time.Second,
		OTLPExporterConfig: otlpExporterConfig{
			Endpoint: otlpEndpoint,
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 5 * time.Second,
			},
		},
	}
}

func createReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	jmxConfig := cfg.(*Config)
	return newJMXMetricReceiver(params, jmxConfig, consumer), nil
}

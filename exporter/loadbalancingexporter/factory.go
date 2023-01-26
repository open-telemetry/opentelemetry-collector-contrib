// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
)

const (
	// The value of "type" key in configuration.
	typeStr = "loadbalancing"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for the exporter.
func NewFactory() exporter.Factory {
	_ = view.Register(MetricViews()...)

	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
		exporter.WithLogs(createLogsExporter, stability),
	)
}

func createDefaultConfig() component.Config {
	otlpFactory := otlpexporter.NewFactory()
	otlpDefaultCfg := otlpFactory.CreateDefaultConfig().(*otlpexporter.Config)

	return &Config{
		Protocol: Protocol{
			OTLP: *otlpDefaultCfg,
		},
	}
}

func createTracesExporter(_ context.Context, params exporter.CreateSettings, cfg component.Config) (exporter.Traces, error) {
	return newTracesExporter(params, cfg)
}

func createLogsExporter(_ context.Context, params exporter.CreateSettings, cfg component.Config) (exporter.Logs, error) {
	return newLogsExporter(params, cfg)
}

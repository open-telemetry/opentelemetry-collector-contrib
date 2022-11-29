// Copyright  The OpenTelemetry Authors
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

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "logicmonitor"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a LogicMonitor exporter factory
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesExporter(createTracesExporter, stability),
		component.WithLogsExporter(createLogsExporter, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
	}
}

func createTracesExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	c component.Config,
) (component.TracesExporter, error) {
	cfg := c.(*Config)
	// TODO: Lines commented out until implementation is available
	// lmexpCfg, err := newTracesExporter(cfg, set)
	// if err != nil {
	// 	return nil, err
	// }
	var pushConvertedTraces consumer.ConsumeTracesFunc

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		// TODO: Lines commented out until implementation is available
		// lmexpCfg.pushTraces,
		pushConvertedTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings))
}

func createLogsExporter(ctx context.Context, set component.ExporterCreateSettings, cfg component.Config) (component.LogsExporter, error) {
	// TODO: Lines commented out until implementation is available
	// lmexpCfg, err := newLogsExporter(cfg, set.Logger)
	// if err != nil {
	// 	return nil, err
	// }
	c := cfg.(*Config)
	var pushConvertedLogs consumer.ConsumeLogsFunc
	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		// TODO: Lines commented out until implementation is available
		// lmexpCfg.PushLogData,
		pushConvertedLogs,
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.RetrySettings),
	)
}

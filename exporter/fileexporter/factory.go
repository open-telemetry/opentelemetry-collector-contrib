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

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

const (
	// The value of "type" key in configuration.
	typeStr = "file"
	// The stability level of the exporter.
	stability = component.StabilityLevelAlpha
	// the number of old log files to retain
	defaultMaxBackups = 100
)

// NewFactory creates a factory for OTLP exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesExporter(createTracesExporter, stability),
		component.WithMetricsExporter(createMetricsExporter, stability),
		component.WithLogsExporter(createLogsExporter, stability))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		Rotation:         Rotation{MaxBackups: defaultMaxBackups},
		FormatType:       formatTypeJSON,
	}
}

func createTracesExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.TracesExporter, error) {
	fe := exporters.GetOrAdd(cfg, func() component.Component {
		conf := cfg.(*Config)
		return &fileExporter{
			path:       conf.Path,
			formatType: conf.FormatType,
			file: &lumberjack.Logger{
				Filename:   conf.Path,
				MaxSize:    conf.Rotation.MaxMegabytes,
				MaxAge:     conf.Rotation.MaxDays,
				MaxBackups: conf.Rotation.MaxBackups,
				LocalTime:  conf.Rotation.LocalTime,
			},
			tracesMarshaler: tracesMarshalers[conf.FormatType],
			exporter:        buildExportFunc(conf),
		}
	})
	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		fe.Unwrap().(*fileExporter).ConsumeTraces,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.MetricsExporter, error) {
	fe := exporters.GetOrAdd(cfg, func() component.Component {
		conf := cfg.(*Config)
		return &fileExporter{
			path:       conf.Path,
			formatType: conf.FormatType,
			file: &lumberjack.Logger{
				Filename:   conf.Path,
				MaxSize:    conf.Rotation.MaxMegabytes,
				MaxAge:     conf.Rotation.MaxDays,
				MaxBackups: conf.Rotation.MaxBackups,
				LocalTime:  conf.Rotation.LocalTime,
			},
			metricsMarshaler: metricsMarshalers[conf.FormatType],
			exporter:         buildExportFunc(conf),
		}
	})
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		fe.Unwrap().(*fileExporter).ConsumeMetrics,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
	)
}

func createLogsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.LogsExporter, error) {
	fe := exporters.GetOrAdd(cfg, func() component.Component {
		conf := cfg.(*Config)
		return &fileExporter{
			path:       conf.Path,
			formatType: conf.FormatType,
			file: &lumberjack.Logger{
				Filename:   conf.Path,
				MaxSize:    conf.Rotation.MaxMegabytes,
				MaxAge:     conf.Rotation.MaxDays,
				MaxBackups: conf.Rotation.MaxBackups,
				LocalTime:  conf.Rotation.LocalTime,
			},
			logsMarshaler: logsMarshalers[conf.FormatType],
			exporter:      buildExportFunc(conf),
		}
	})
	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		fe.Unwrap().(*fileExporter).ConsumeLogs,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
	)
}

// This is the map of already created File exporters for particular configurations.
// We maintain this map because the Factory is asked trace and metric receivers separately
// when it gets CreateTracesReceiver() and CreateMetricsReceiver() but they must not
// create separate objects, they must use one Receiver object per configuration.
var exporters = sharedcomponent.NewSharedComponents()

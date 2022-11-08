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
	"io"
	"os"

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

	// the format of encoded telemetry data
	formatTypeJSON  = "json"
	formatTypeProto = "proto"

	// the type of compression codec
	compressionZSTD = "zstd"
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

func createDefaultConfig() component.ExporterConfig {
	return &Config{
		ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
		FormatType:       formatTypeJSON,
		Rotation:         &Rotation{MaxBackups: defaultMaxBackups},
	}
}

func createTracesExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	cfg component.ExporterConfig,
) (component.TracesExporter, error) {
	conf := cfg.(*Config)
	writer, err := buildFileWriter(conf)
	if err != nil {
		return nil, err
	}
	fe := exporters.GetOrAdd(cfg, func() component.Component {
		return &fileExporter{
			path:            conf.Path,
			formatType:      conf.FormatType,
			file:            writer,
			tracesMarshaler: tracesMarshalers[conf.FormatType],
			exporter:        buildExportFunc(conf),
			compression:     conf.Compression,
			compressor:      buildCompressor(conf.Compression),
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
	cfg component.ExporterConfig,
) (component.MetricsExporter, error) {
	conf := cfg.(*Config)
	writer, err := buildFileWriter(conf)
	if err != nil {
		return nil, err
	}
	fe := exporters.GetOrAdd(cfg, func() component.Component {
		return &fileExporter{
			path:             conf.Path,
			formatType:       conf.FormatType,
			file:             writer,
			metricsMarshaler: metricsMarshalers[conf.FormatType],
			exporter:         buildExportFunc(conf),
			compression:      conf.Compression,
			compressor:       buildCompressor(conf.Compression),
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
	cfg component.ExporterConfig,
) (component.LogsExporter, error) {
	conf := cfg.(*Config)
	writer, err := buildFileWriter(conf)
	if err != nil {
		return nil, err
	}
	fe := exporters.GetOrAdd(cfg, func() component.Component {
		return &fileExporter{
			path:          conf.Path,
			formatType:    conf.FormatType,
			file:          writer,
			logsMarshaler: logsMarshalers[conf.FormatType],
			exporter:      buildExportFunc(conf),
			compression:   conf.Compression,
			compressor:    buildCompressor(conf.Compression),
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

func buildFileWriter(cfg *Config) (io.WriteCloser, error) {
	if cfg.Rotation == nil {
		return os.OpenFile(cfg.Path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	}
	return &lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.Rotation.MaxMegabytes,
		MaxAge:     cfg.Rotation.MaxDays,
		MaxBackups: cfg.Rotation.MaxBackups,
		LocalTime:  cfg.Rotation.LocalTime,
	}, nil
}

// This is the map of already created File exporters for particular configurations.
// We maintain this map because the Factory is asked trace and metric receivers separately
// when it gets CreateTracesReceiver() and CreateMetricsReceiver() but they must not
// create separate objects, they must use one Receiver object per configuration.
var exporters = sharedcomponent.NewSharedComponents()

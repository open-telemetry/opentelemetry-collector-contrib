// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"
	"io"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

const (
	// the number of old log files to retain
	defaultMaxBackups = 100

	// the format of encoded telemetry data
	formatTypeJSON  = "json"
	formatTypeProto = "proto"

	// the type of compression codec
	compressionZSTD = "zstd"
)

type FileExporter interface {
	component.Component
	consumeTraces(_ context.Context, td ptrace.Traces) error
	consumeMetrics(_ context.Context, md pmetric.Metrics) error
	consumeLogs(_ context.Context, ld plog.Logs) error
}

// NewFactory creates a factory for OTLP exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		FormatType: formatTypeJSON,
		Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Traces, error) {
	fe, err := getOrCreateFileExporter(cfg)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		fe.consumeTraces,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	fe, err := getOrCreateFileExporter(cfg)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		fe.consumeMetrics,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	fe, err := getOrCreateFileExporter(cfg)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		fe.consumeLogs,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
	)
}

// getOrCreateFileExporter creates a FileExporter and caches it for a particular configuration,
// or returns the already cached one. Caching is required because the factory is asked trace and
// metric receivers separately when it gets CreateTracesReceiver() and CreateMetricsReceiver()
// but they must not create separate objects, they must use one Exporter object per configuration.
func getOrCreateFileExporter(cfg component.Config) (FileExporter, error) {
	conf := cfg.(*Config)
	fe := exporters.GetOrAdd(cfg, func() component.Component {
		e, err := newFileExporter(conf)
		if err != nil {
			return &errorComponent{err: err}
		}

		return e
	})

	component := fe.Unwrap()
	if errComponent, ok := component.(*errorComponent); ok {
		return nil, errComponent.err
	}

	return component.(FileExporter), nil
}

func newFileExporter(conf *Config) (FileExporter, error) {
	marshaller := &marshaller{
		formatType:       conf.FormatType,
		tracesMarshaler:  tracesMarshalers[conf.FormatType],
		metricsMarshaler: metricsMarshalers[conf.FormatType],
		logsMarshaler:    logsMarshalers[conf.FormatType],
		compression:      conf.Compression,
		compressor:       buildCompressor(conf.Compression),
	}
	export := buildExportFunc(conf)

	writer, err := newFileWriter(conf.Path, conf.Rotation, conf.FlushInterval, export)
	if err != nil {
		return nil, err
	}

	return &fileExporter{
		marshaller: marshaller,
		writer:     writer,
	}, nil
}

func newFileWriter(path string, rotation *Rotation, flushInterval time.Duration, export exportFunc) (*fileWriter, error) {
	var wc io.WriteCloser
	if rotation == nil {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return nil, err
		}
		wc = newBufferedWriteCloser(f)
	} else {
		wc = &lumberjack.Logger{
			Filename:   path,
			MaxSize:    rotation.MaxMegabytes,
			MaxAge:     rotation.MaxDays,
			MaxBackups: rotation.MaxBackups,
			LocalTime:  rotation.LocalTime,
		}
	}

	return &fileWriter{
		path:          path,
		file:          wc,
		exporter:      export,
		flushInterval: flushInterval,
	}, nil
}

// This is the map of already created File exporters for particular configurations.
// We maintain this map because the Factory is asked trace and metric receivers separately
// when it gets CreateTracesReceiver() and CreateMetricsReceiver() but they must not
// create separate objects, they must use one Exporter object per configuration.
var exporters = sharedcomponent.NewSharedComponents()

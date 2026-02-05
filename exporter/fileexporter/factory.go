// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/DeRuina/timberjack"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

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

	defaultMaxOpenFiles = 100

	defaultResourceAttribute = "fileexporter.path_segment"
)

type FileExporter interface {
	component.Component
	consumeTraces(_ context.Context, td ptrace.Traces) error
	consumeMetrics(_ context.Context, md pmetric.Metrics) error
	consumeLogs(_ context.Context, ld plog.Logs) error
	consumeProfiles(_ context.Context, pd pprofile.Profiles) error
}

// NewFactory creates a factory for OTLP exporter.
func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithTraces(createTracesExporter, metadata.TracesStability),
		xexporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithProfiles(createProfilesExporter, metadata.ProfilesStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		FormatType: formatTypeJSON,
		Rotation:   &Rotation{MaxBackups: defaultMaxBackups},
		GroupBy: &GroupBy{
			ResourceAttribute: defaultResourceAttribute,
			MaxOpenFiles:      defaultMaxOpenFiles,
		},
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	fe := getOrCreateFileExporter(cfg, set.Logger)
	return exporterhelper.NewTraces(
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
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	fe := getOrCreateFileExporter(cfg, set.Logger)
	return exporterhelper.NewMetrics(
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
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	fe := getOrCreateFileExporter(cfg, set.Logger)
	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		fe.consumeLogs,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
	)
}

func createProfilesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (xexporter.Profiles, error) {
	fe := getOrCreateFileExporter(cfg, set.Logger)
	return xexporterhelper.NewProfiles(
		ctx,
		set,
		cfg,
		fe.consumeProfiles,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
	)
}

// getOrCreateFileExporter creates a FileExporter and caches it for a particular configuration,
// or returns the already cached one. Caching is required because the factory is asked trace and
// metric receivers separately when it gets CreateTraces() and CreateMetrics()
// but they must not create separate objects, they must use one Exporter object per configuration.
func getOrCreateFileExporter(cfg component.Config, logger *zap.Logger) FileExporter {
	conf := cfg.(*Config)
	fe := exporters.GetOrAdd(cfg, func() component.Component {
		return newFileExporter(conf, logger)
	})

	c := fe.Unwrap()
	return c.(FileExporter)
}

func newFileExporter(conf *Config, logger *zap.Logger) FileExporter {
	if conf.GroupBy == nil || !conf.GroupBy.Enabled {
		return &fileExporter{
			conf: conf,
		}
	}

	return &groupingFileExporter{
		conf:   conf,
		logger: logger,
	}
}

func newFileWriter(path string, shouldAppend bool, rotation *Rotation, flushInterval time.Duration, export exportFunc) (*fileWriter, error) {
	var wc io.WriteCloser
	if rotation == nil {
		fileFlags := os.O_RDWR | os.O_CREATE
		if shouldAppend {
			fileFlags |= os.O_APPEND
		} else {
			fileFlags |= os.O_TRUNC
		}
		f, err := os.OpenFile(path, fileFlags, 0o644)
		if err != nil {
			return nil, err
		}
		wc = newBufferedWriteCloser(f)
	} else {
		wc = &timberjack.Logger{
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
// when it gets CreateTraces() and CreateMetrics() but they must not
// create separate objects, they must use one Exporter object per configuration.
var exporters = sharedcomponent.NewSharedComponents()

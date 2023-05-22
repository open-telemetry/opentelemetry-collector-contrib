// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package influxdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"

import (
	"context"
	"fmt"

	"github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/influxdb-observability/otel2influx"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

type tracesExporter struct {
	logger    common.Logger
	writer    *influxHTTPWriter
	converter *otel2influx.OtelTracesToLineProtocol
	started   bool
}

func newTracesExporter(config *Config, settings exporter.CreateSettings) (*tracesExporter, error) {
	logger := newZapInfluxLogger(settings.Logger)

	writer, err := newInfluxHTTPWriter(logger, config, settings.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	converter, err := otel2influx.NewOtelTracesToLineProtocol(logger, writer)
	if err != nil {
		return nil, err
	}

	return &tracesExporter{
		logger:    logger,
		writer:    writer,
		converter: converter,
	}, nil
}

func (e *tracesExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	err := e.converter.WriteTraces(ctx, td)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	return nil
}

func (e *tracesExporter) Start(ctx context.Context, host component.Host) error {
	e.logger.Debug("starting traces exporter")
	e.started = true
	return multierr.Combine(
		e.writer.Start(ctx, host),
		e.converter.Start(ctx, host))
}

func (e *tracesExporter) Shutdown(ctx context.Context) error {
	if !e.started {
		return nil
	}
	return e.converter.Shutdown(ctx)
}

type metricsExporter struct {
	logger    common.Logger
	writer    *influxHTTPWriter
	converter *otel2influx.OtelMetricsToLineProtocol
}

func newMetricsExporter(config *Config, params exporter.CreateSettings) (*metricsExporter, error) {
	logger := newZapInfluxLogger(params.Logger)
	schema, found := common.MetricsSchemata[config.MetricsSchema]
	if !found {
		return nil, fmt.Errorf("schema '%s' not recognized", config.MetricsSchema)
	}

	writer, err := newInfluxHTTPWriter(logger, config, params.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	converter, err := otel2influx.NewOtelMetricsToLineProtocol(logger, writer, schema)
	if err != nil {
		return nil, err
	}

	return &metricsExporter{
		logger:    logger,
		writer:    writer,
		converter: converter,
	}, nil
}

func (e *metricsExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	err := e.converter.WriteMetrics(ctx, md)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	return nil
}

func (e *metricsExporter) Start(ctx context.Context, host component.Host) error {
	return e.writer.Start(ctx, host)
}

type logsExporter struct {
	logger    common.Logger
	writer    *influxHTTPWriter
	converter *otel2influx.OtelLogsToLineProtocol
}

func newLogsExporter(config *Config, params exporter.CreateSettings) (*logsExporter, error) {
	logger := newZapInfluxLogger(params.Logger)

	writer, err := newInfluxHTTPWriter(logger, config, params.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	converter := otel2influx.NewOtelLogsToLineProtocol(logger, writer)

	return &logsExporter{
		logger:    logger,
		writer:    writer,
		converter: converter,
	}, nil
}

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	err := e.converter.WriteLogs(ctx, ld)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	return nil
}

func (e *logsExporter) Start(ctx context.Context, host component.Host) error {
	return e.writer.Start(ctx, host)
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// NOTE: If you are making changes to this file, consider whether you want to make similar changes
// to the Logging exporter in /exporter/internal/common/logging_exporter.go, which has similar logic.
// This is especially important for security issues.

package opsrampdebugexporter // import "go.opentelemetry.io/collector/exporter/debugexporter"

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opsrampdebugexporter/internal/normal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opsrampdebugexporter/internal/otlptext"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var (
	LogsOpsRampChannel   = make(chan plog.Logs, 1000)
	EventsOpsRampChannel = make(chan plog.Logs, 100)
)

type debugExporter struct {
	verbosity        configtelemetry.Level
	logger           *zap.Logger
	logsMarshaler    plog.Marshaler
	metricsMarshaler pmetric.Marshaler
	tracesMarshaler  ptrace.Marshaler
}

func newDebugExporter(logger *zap.Logger, verbosity configtelemetry.Level) *debugExporter {
	var logsMarshaler plog.Marshaler
	var metricsMarshaler pmetric.Marshaler
	var tracesMarshaler ptrace.Marshaler
	if verbosity == configtelemetry.LevelDetailed {
		logsMarshaler = otlptext.NewTextLogsMarshaler()
		metricsMarshaler = otlptext.NewTextMetricsMarshaler()
		tracesMarshaler = otlptext.NewTextTracesMarshaler()
	} else {
		logsMarshaler = normal.NewNormalLogsMarshaler()
		metricsMarshaler = normal.NewNormalMetricsMarshaler()
		tracesMarshaler = normal.NewNormalTracesMarshaler()
	}
	return &debugExporter{
		verbosity:        verbosity,
		logger:           logger,
		logsMarshaler:    logsMarshaler,
		metricsMarshaler: metricsMarshaler,
		tracesMarshaler:  tracesMarshaler,
	}
}

func (s *debugExporter) pushTraces(_ context.Context, td ptrace.Traces) error {
	s.logger.Info("TracesExporter",
		zap.Int("resource spans", td.ResourceSpans().Len()),
		zap.Int("spans", td.SpanCount()))
	if s.verbosity == configtelemetry.LevelBasic {
		return nil
	}

	buf, err := s.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return err
	}
	s.logger.Info(string(buf))
	return nil
}

func (s *debugExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	s.logger.Info("MetricsExporter",
		zap.Int("resource metrics", md.ResourceMetrics().Len()),
		zap.Int("metrics", md.MetricCount()),
		zap.Int("data points", md.DataPointCount()))
	if s.verbosity == configtelemetry.LevelBasic {
		return nil
	}

	buf, err := s.metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}
	s.logger.Info(string(buf))
	return nil
}

func (s *debugExporter) pushLogs(_ context.Context, ld plog.Logs) error {
	s.logger.Info("LogsExporter",
		zap.Int("resource logs", ld.ResourceLogs().Len()),
		zap.Int("log records", ld.LogRecordCount()))

	eventsSlice := plog.NewResourceLogsSlice()
	logsSlice := plog.NewResourceLogsSlice()

	rlSlice := ld.ResourceLogs()
	for i := 0; i < rlSlice.Len(); i++ {
		rl := rlSlice.At(i)
		resource := rl.Resource()

		if val, found := resource.Attributes().Get("type"); found && val.Str() == "event" {
			rl.CopyTo(eventsSlice.AppendEmpty())
		} else {
			rl.CopyTo(logsSlice.AppendEmpty())
		}
	}

	if logsSlice.Len() != 0 {
		logs := plog.NewLogs()
		logsSlice.CopyTo(logs.ResourceLogs())
		select {
		case LogsOpsRampChannel <- logs:
			s.logger.Info("#######LogsExporter: Successfully sent to logs channel")
		default:
			s.logger.Info("#######LogsExporter: failed sent to logs channel")
		}
	}

	if eventsSlice.Len() != 0 {
		eventLogs := plog.NewLogs()
		eventsSlice.CopyTo(eventLogs.ResourceLogs())
		select {
		case EventsOpsRampChannel <- eventLogs:
			s.logger.Info("#######LogsExporter: Successfully sent to events channel")
		default:
			s.logger.Info("#######LogsExporter: failed sent to eventschannel")
		}
	}

	if s.verbosity == configtelemetry.LevelBasic {
		return nil
	}

	buf, err := s.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}
	s.logger.Info(string(buf))
	return nil
}

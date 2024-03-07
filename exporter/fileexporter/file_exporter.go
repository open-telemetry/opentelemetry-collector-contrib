// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// fileExporter is the implementation of file exporter that writes telemetry data to a file
type fileExporter struct {
	conf       *Config
	marshaller *marshaller
	writer     *fileWriter
}

func (e *fileExporter) consumeTraces(_ context.Context, td ptrace.Traces) error {
	buf, err := e.marshaller.marshalTraces(td)
	if err != nil {
		return err
	}
	return e.writer.export(buf)
}

func (e *fileExporter) consumeMetrics(_ context.Context, md pmetric.Metrics) error {
	buf, err := e.marshaller.marshalMetrics(md)
	if err != nil {
		return err
	}
	return e.writer.export(buf)
}

func (e *fileExporter) consumeLogs(_ context.Context, ld plog.Logs) error {
	buf, err := e.marshaller.marshalLogs(ld)
	if err != nil {
		return err
	}
	return e.writer.export(buf)
}

// Start starts the flush timer if set.
func (e *fileExporter) Start(_ context.Context, _ component.Host) error {
	e.marshaller = &marshaller{
		formatType:       e.conf.FormatType,
		tracesMarshaler:  tracesMarshalers[e.conf.FormatType],
		metricsMarshaler: metricsMarshalers[e.conf.FormatType],
		logsMarshaler:    logsMarshalers[e.conf.FormatType],
		compression:      e.conf.Compression,
		compressor:       buildCompressor(e.conf.Compression),
	}
	export := buildExportFunc(e.conf)

	var err error
	e.writer, err = newFileWriter(e.conf.Path, e.conf.Append, e.conf.Rotation, e.conf.FlushInterval, export)
	if err != nil {
		return err
	}
	e.writer.start()
	return nil
}

// Shutdown stops the exporter and is invoked during shutdown.
// It stops the flush ticker if set.
func (e *fileExporter) Shutdown(context.Context) error {
	if e.writer == nil {
		return nil
	}
	w := e.writer
	e.writer = nil
	return w.shutdown()
}

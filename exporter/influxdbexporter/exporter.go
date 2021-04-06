// Copyright 2021, OpenTelemetry Authors
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

package influxdbexporter

import (
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/influxdata/influxdb-observability/otel2influx"
	otlpcollectorlogs "github.com/influxdata/influxdb-observability/otlp/collector/logs/v1"
	otlpcollectormetrics "github.com/influxdata/influxdb-observability/otlp/collector/metrics/v1"
	otlpcollectortrace "github.com/influxdata/influxdb-observability/otlp/collector/trace/v1"
	otlplogs "github.com/influxdata/influxdb-observability/otlp/logs/v1"
	otlpmetrics "github.com/influxdata/influxdb-observability/otlp/metrics/v1"
	otlptrace "github.com/influxdata/influxdb-observability/otlp/trace/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type exporter struct {
	influxWriter    *influxHTTPWriter
	influxConverter *otel2influx.OpenTelemetryToInfluxConverter

	logger *zap.Logger
}

func newExporter(config *Config, params component.ExporterCreateParams) (*exporter, error) {
	switch config.Protocol {
	case protocolLineProtocol:
		influxLogger := newZapInfluxLogger(params.Logger)
		converter := otel2influx.NewOpenTelemetryToInfluxConverter(influxLogger)
		influxWriter, err := newInfluxHTTPWriter(influxLogger, config.LineProtocolServerURL, config.LineProtocolAuthToken, config.Org, config.Bucket, config.Timeout)
		if err != nil {
			return nil, err
		}

		return &exporter{
			influxWriter:    influxWriter,
			influxConverter: converter,
			logger:          params.Logger,
		}, nil

	case protocolGrpc:
		// TODO
		fallthrough

	default:
		return nil, fmt.Errorf("protocol %q not supported", config.Protocol)
	}
}

func (e *exporter) pushTraces(ctx context.Context, td pdata.Traces) error {
	var spans []*otlptrace.ResourceSpans
	{
		var r otlpcollectortrace.ExportTraceServiceRequest
		if protoBytes, err := td.ToOtlpProtoBytes(); err != nil {
			return err
		} else if err = proto.Unmarshal(protoBytes, &r); err != nil {
			return err
		}
		spans = r.ResourceSpans
	}

	batch := e.influxWriter.newBatch()
	_ = e.influxConverter.WriteTraces(ctx, spans, batch)
	if err := batch.flushAndClose(ctx); err != nil {
		return err
	}

	return nil
}

func (e *exporter) pushMetrics(ctx context.Context, md pdata.Metrics) error {
	var metrics []*otlpmetrics.ResourceMetrics
	{
		var r otlpcollectormetrics.ExportMetricsServiceRequest
		if protoBytes, err := md.ToOtlpProtoBytes(); err != nil {
			return err
		} else if err = proto.Unmarshal(protoBytes, &r); err != nil {
			return err
		}
		metrics = r.ResourceMetrics
	}

	batch := e.influxWriter.newBatch()
	_ = e.influxConverter.WriteMetrics(ctx, metrics, batch)
	if err := batch.flushAndClose(ctx); err != nil {
		return err
	}

	return nil
}

func (e *exporter) pushLogs(ctx context.Context, ld pdata.Logs) error {
	var logRecords []*otlplogs.ResourceLogs
	{
		var r otlpcollectorlogs.ExportLogsServiceRequest
		if protoBytes, err := ld.ToOtlpProtoBytes(); err != nil {
			return err
		} else if err = proto.Unmarshal(protoBytes, &r); err != nil {
			return err
		}
		logRecords = r.ResourceLogs
	}

	batch := e.influxWriter.newBatch()
	_ = e.influxConverter.WriteLogs(ctx, logRecords, batch)
	if err := batch.flushAndClose(ctx); err != nil {
		return err
	}

	return nil
}

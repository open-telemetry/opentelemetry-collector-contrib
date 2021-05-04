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

	"github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/influxdb-observability/otel2influx"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type tracesExporter struct {
	writer    *influxHTTPWriter
	converter *otel2influx.OtelTracesToLineProtocol
}

func newTracesExporter(config *Config, params component.ExporterCreateParams) (*tracesExporter, error) {
	influxLogger := newZapInfluxLogger(params.Logger)
	converter := otel2influx.NewOtelTracesToLineProtocol(influxLogger)
	writer, err := newInfluxHTTPWriter(influxLogger, config)
	if err != nil {
		return nil, err
	}

	return &tracesExporter{
		writer:    writer,
		converter: converter,
	}, nil
}

func (e *tracesExporter) pushTraces(ctx context.Context, td pdata.Traces) error {
	batch := e.writer.newBatch()

	protoBytes, err := td.ToOtlpProtoBytes()
	if err != nil {
		return consumererror.Permanent(err)
	}
	err = e.converter.WriteTracesFromRequestBytes(ctx, protoBytes, batch)
	if err != nil {
		return consumererror.Permanent(err)
	}
	return batch.flushAndClose(ctx)
}

type metricsExporter struct {
	writer    *influxHTTPWriter
	converter *otel2influx.OtelMetricsToLineProtocol
}

var metricsSchemata = map[string]common.MetricsSchema{
	"telegraf-prometheus-v1": common.MetricsSchemaTelegrafPrometheusV1,
	"telegraf-prometheus-v2": common.MetricsSchemaTelegrafPrometheusV2,
}

func newMetricsExporter(config *Config, params component.ExporterCreateParams) (*metricsExporter, error) {
	influxLogger := newZapInfluxLogger(params.Logger)
	schema, found := metricsSchemata[config.MetricsSchema]
	if !found {
		return nil, fmt.Errorf("schema '%s' not recognized", config.MetricsSchema)
	}

	converter, err := otel2influx.NewOtelMetricsToLineProtocol(influxLogger, schema)
	if err != nil {
		return nil, err
	}
	writer, err := newInfluxHTTPWriter(influxLogger, config)
	if err != nil {
		return nil, err
	}

	return &metricsExporter{
		writer:    writer,
		converter: converter,
	}, nil
}

func (e *metricsExporter) pushMetrics(ctx context.Context, md pdata.Metrics) error {
	batch := e.writer.newBatch()

	protoBytes, err := md.ToOtlpProtoBytes()
	if err != nil {
		return consumererror.Permanent(err)
	}
	err = e.converter.WriteMetricsFromRequestBytes(ctx, protoBytes, batch)
	if err != nil {
		return consumererror.Permanent(err)
	}
	return batch.flushAndClose(ctx)
}

type logsExporter struct {
	writer    *influxHTTPWriter
	converter *otel2influx.OtelLogsToLineProtocol
}

func newLogsExporter(config *Config, params component.ExporterCreateParams) (*logsExporter, error) {
	influxLogger := newZapInfluxLogger(params.Logger)
	converter := otel2influx.NewOtelLogsToLineProtocol(influxLogger)
	writer, err := newInfluxHTTPWriter(influxLogger, config)
	if err != nil {
		return nil, err
	}

	return &logsExporter{
		writer:    writer,
		converter: converter,
	}, nil
}

func (e *logsExporter) pushLogs(ctx context.Context, ld pdata.Logs) error {
	batch := e.writer.newBatch()

	protoBytes, err := ld.ToOtlpProtoBytes()
	if err != nil {
		return consumererror.Permanent(err)
	}
	err = e.converter.WriteLogsFromRequestBytes(ctx, protoBytes, batch)
	if err != nil {
		return consumererror.Permanent(err)
	}
	return batch.flushAndClose(ctx)
}

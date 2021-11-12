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
	"go.opentelemetry.io/collector/model/pdata"
)

type tracesExporter struct {
	logger    common.Logger
	cfg       *Config
	writer    *influxHTTPWriter
	converter *otel2influx.OtelTracesToLineProtocol
}

func newTracesExporter(config *Config, params component.ExporterCreateSettings) *tracesExporter {
	logger := newZapInfluxLogger(params.Logger)
	converter := otel2influx.NewOtelTracesToLineProtocol(logger)

	return &tracesExporter{
		logger:    logger,
		cfg:       config,
		converter: converter,
	}
}

func (e *tracesExporter) pushTraces(ctx context.Context, td pdata.Traces) error {
	batch := e.writer.newBatch()

	err := e.converter.WriteTraces(ctx, td, batch)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	return batch.flushAndClose(ctx)
}

// start starts the traces exporter
func (e *tracesExporter) start(_ context.Context, host component.Host) (err error) {

	writer, err := newInfluxHTTPWriter(e.logger, e.cfg, host)
	if err != nil {
		return err
	}
	e.writer = writer

	return nil
}

type metricsExporter struct {
	logger    common.Logger
	cfg       *Config
	writer    *influxHTTPWriter
	converter *otel2influx.OtelMetricsToLineProtocol
}

var metricsSchemata = map[string]common.MetricsSchema{
	"telegraf-prometheus-v1": common.MetricsSchemaTelegrafPrometheusV1,
	"telegraf-prometheus-v2": common.MetricsSchemaTelegrafPrometheusV2,
}

func newMetricsExporter(config *Config, params component.ExporterCreateSettings) (*metricsExporter, error) {
	logger := newZapInfluxLogger(params.Logger)
	schema, found := metricsSchemata[config.MetricsSchema]
	if !found {
		return nil, fmt.Errorf("schema '%s' not recognized", config.MetricsSchema)
	}

	converter, err := otel2influx.NewOtelMetricsToLineProtocol(logger, schema)
	if err != nil {
		return nil, err
	}

	return &metricsExporter{
		logger:    logger,
		cfg:       config,
		converter: converter,
	}, nil
}

func (e *metricsExporter) pushMetrics(ctx context.Context, md pdata.Metrics) error {
	batch := e.writer.newBatch()

	err := e.converter.WriteMetrics(ctx, md, batch)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	return batch.flushAndClose(ctx)
}

// start starts the metrics exporter
func (e *metricsExporter) start(_ context.Context, host component.Host) (err error) {

	writer, err := newInfluxHTTPWriter(e.logger, e.cfg, host)
	if err != nil {
		return err
	}
	e.writer = writer

	return nil
}

type logsExporter struct {
	logger    common.Logger
	cfg       *Config
	writer    *influxHTTPWriter
	converter *otel2influx.OtelLogsToLineProtocol
}

func newLogsExporter(config *Config, params component.ExporterCreateSettings) *logsExporter {
	logger := newZapInfluxLogger(params.Logger)
	converter := otel2influx.NewOtelLogsToLineProtocol(logger)

	return &logsExporter{
		logger:    logger,
		converter: converter,
		cfg:       config,
	}
}

func (e *logsExporter) pushLogs(ctx context.Context, ld pdata.Logs) error {
	batch := e.writer.newBatch()

	err := e.converter.WriteLogs(ctx, ld, batch)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	return batch.flushAndClose(ctx)
}

// start starts the logs exporter
func (e *logsExporter) start(_ context.Context, host component.Host) (err error) {
	writer, err := newInfluxHTTPWriter(e.logger, e.cfg, host)
	if err != nil {
		return err
	}
	e.writer = writer

	return nil
}

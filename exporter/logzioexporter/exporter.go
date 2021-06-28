// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logzioexporter

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/jaegertracing/jaeger/model"
	"github.com/logzio/jaeger-logzio/store"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/translator/trace/jaeger"
)

const (
	loggerName = "logzio-exporter"
)

// logzioExporter implements an OpenTelemetry trace exporter that exports all spans to Logz.io
type logzioExporter struct {
	accountToken                 string
	writer                       *store.LogzioSpanWriter
	logger                       hclog.Logger
	WriteSpanFunc                func(ctx context.Context, span *model.Span) error
	InternalTracesToJaegerTraces func(td pdata.Traces) ([]*model.Batch, error)
}

func newLogzioExporter(config *Config, params component.ExporterCreateSettings) (*logzioExporter, error) {
	logger := Hclog2ZapLogger{
		Zap:  params.Logger,
		name: loggerName,
	}

	if config == nil {
		return nil, errors.New("exporter config can't be null")
	}
	writerConfig := store.LogzioConfig{
		Region:            config.Region,
		AccountToken:      config.TracesToken,
		CustomListenerURL: config.CustomEndpoint,
	}

	spanWriter, err := store.NewLogzioSpanWriter(writerConfig, logger)
	if err != nil {
		return nil, err
	}

	return &logzioExporter{
		writer:                       spanWriter,
		accountToken:                 config.TracesToken,
		logger:                       logger,
		InternalTracesToJaegerTraces: jaeger.InternalTracesToJaegerProto,
		WriteSpanFunc:                spanWriter.WriteSpan,
	}, nil
}

func newLogzioTracesExporter(config *Config, params component.ExporterCreateSettings) (component.TracesExporter, error) {
	exporter, err := newLogzioExporter(config, params)
	if err != nil {
		return nil, err
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		config,
		params.Logger,
		exporter.pushTraceData,
		exporterhelper.WithShutdown(exporter.Shutdown))
}

func newLogzioMetricsExporter(config *Config, params component.ExporterCreateSettings) (component.MetricsExporter, error) {
	exporter, _ := newLogzioExporter(config, params)
	return exporterhelper.NewMetricsExporter(
		config,
		params.Logger,
		exporter.pushMetricsData,
		exporterhelper.WithShutdown(exporter.Shutdown))
}

func (exporter *logzioExporter) pushTraceData(ctx context.Context, traces pdata.Traces) error {
	batches, err := exporter.InternalTracesToJaegerTraces(traces)
	if err != nil {
		return err
	}
	for _, batch := range batches {
		for _, span := range batch.Spans {
			span.Process = batch.Process
			if err := exporter.WriteSpanFunc(ctx, span); err != nil {
				exporter.logger.Debug(fmt.Sprintf("dropped bad span: %s", span.String()))
			}
		}
	}
	return nil
}

func (exporter *logzioExporter) pushMetricsData(ctx context.Context, md pdata.Metrics) error {
	return nil
}

func (exporter *logzioExporter) Shutdown(ctx context.Context) error {
	exporter.logger.Info("Closing logzio exporter..")
	exporter.writer.Close()
	return nil
}

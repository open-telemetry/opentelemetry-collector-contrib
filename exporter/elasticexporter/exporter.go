// Copyright 2020, OpenTelemetry Authors
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

// Package elasticexporter contains an opentelemetry-collector exporter
// for Elastic APM.
package elasticexporter

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// newElasticTraceExporter creates a new exporter for traces.
//
// Traces need to be processed by APM Server. Traces can will not be indexed
// into Elasticsearch as is.
// The "apm_server_url" must be configured in order to create the trace exporter.
func newTraceExporter(
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.TracesExporter, error) {
	exporter, err := newAPMServerExporter(cfg.(*Config), params.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elastic APM trace exporter: %v", err)
	}
	return exporterhelper.NewTraceExporter(cfg, params.Logger, func(ctx context.Context, traces pdata.Traces) (int, error) {
		var dropped int
		var errs []error
		resourceSpansSlice := traces.ResourceSpans()
		for i := 0; i < resourceSpansSlice.Len(); i++ {
			resourceSpans := resourceSpansSlice.At(i)
			n, err := exporter.ExportResourceSpans(ctx, resourceSpans)
			if err != nil {
				errs = append(errs, err)
			}
			dropped += n
		}
		return dropped, componenterror.CombineErrors(errs)
	})
}

// newElasticMetricsExporter creates a new exporter for metrics.
//
// Metrics currently need to be processed by APM Server and can not be directly
// indexed into Elasticsearch as is.
// The "apm_server_url" must be configured in order to create the metrics exporter.
func newMetricsExporter(
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.MetricsExporter, error) {
	exporter, err := newAPMServerExporter(cfg.(*Config), params.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elastic APM metrics exporter: %v", err)
	}
	return exporterhelper.NewMetricsExporter(cfg, params.Logger, func(ctx context.Context, input pdata.Metrics) (int, error) {
		var dropped int
		var errs []error
		resourceMetricsSlice := input.ResourceMetrics()
		for i := 0; i < resourceMetricsSlice.Len(); i++ {
			resourceMetrics := resourceMetricsSlice.At(i)
			n, err := exporter.ExportResourceMetrics(ctx, resourceMetrics)
			if err != nil {
				errs = append(errs, err)
			}
			dropped += n
		}
		return dropped, componenterror.CombineErrors(errs)
	})
}

// newElasticLogsExporter creates a new exporter for logs.
//
// Logs are directly indexed into Elasticsearch and can currently not be
// processed by APM Server.
// The "elasticsearch_url" must be configured in order to create the logs exporter.
func newLogsExporter(
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.LogsExporter, error) {
	exporter, err := newElasticsearchExporter(cfg.(*Config), params.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch logs exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		cfg,
		params.Logger,
		exporter.pushLogsData,
		exporterhelper.WithShutdown(exporter.Shutdown),
	)
}

// Copyright 2019, OpenTelemetry Authors
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

// Package stackdriverexporter contains the wrapper for OpenTelemetry-Stackdriver
// exporter to be used in OpenTelemetry-Service.
package stackdriverexporter

import (
	"context"
	"fmt"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"google.golang.org/api/option"

	"github.com/open-telemetry/opentelemetry-service/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-service/exporter"
	"github.com/open-telemetry/opentelemetry-service/exporter/exporterhelper"
	"github.com/open-telemetry/opentelemetry-service/oterr"
	spandatatranslator "github.com/open-telemetry/opentelemetry-service/translator/trace/spandata"
)

// stackdriverExporter is a wrapper struct of Stackdriver exporter
type stackdriverExporter struct {
	exporter *stackdriver.Exporter
}

func (*stackdriverExporter) Name() string {
	return "stackdriver"
}

func (se *stackdriverExporter) Shutdown() error {
	se.exporter.Flush()
	se.exporter.StopMetricsExporter()
	return nil
}

func newStackdriverTraceExporter(cfg *Config) (exporter.TraceExporter, error) {
	options := getStackdriverOptions(cfg.ProjectID, cfg.Prefix)
	if cfg.Endpoint != "" {
		options.TraceClientOptions = []option.ClientOption{option.WithEndpoint(cfg.Endpoint)}
	}
	if cfg.NumOfWorkers > 0 {
		options.NumberOfWorkers = cfg.NumOfWorkers
	}
	sde, serr := stackdriver.NewExporter(options)
	if serr != nil {
		return nil, fmt.Errorf("cannot configure Stackdriver Trace exporter: %v", serr)
	}
	tExp := &stackdriverExporter{exporter: sde}

	return exporterhelper.NewTraceExporter(
		cfg,
		tExp.pushTraceData,
		exporterhelper.WithTracing(true),
		exporterhelper.WithMetrics(true),
		exporterhelper.WithShutdown(tExp.Shutdown))
}

func newStackdriverMetricsExporter(cfg *Config) (exporter.MetricsExporter, error) {
	options := getStackdriverOptions(cfg.ProjectID, cfg.Prefix)
	if cfg.Endpoint != "" {
		options.MonitoringClientOptions = []option.ClientOption{option.WithEndpoint(cfg.Endpoint)}
	}
	if cfg.NumOfWorkers > 0 {
		options.NumberOfWorkers = cfg.NumOfWorkers
	}
	sde, serr := stackdriver.NewExporter(options)
	if serr != nil {
		return nil, fmt.Errorf("cannot configure Stackdriver metric exporter: %v", serr)
	}
	mExp := &stackdriverExporter{exporter: sde}

	return exporterhelper.NewMetricsExporter(
		cfg,
		mExp.pushMetricsData,
		exporterhelper.WithTracing(true),
		exporterhelper.WithMetrics(true),
		exporterhelper.WithShutdown(mExp.Shutdown))
}

func getStackdriverOptions(ProjectID, MetricPrefix string) stackdriver.Options {
	// TODO:  For each ProjectID, create a different exporter
	// or at least a unique Stackdriver client per ProjectID.

	return stackdriver.Options{
		// If the project ID is an empty string, it will be set by default based on
		// the project this is running on in GCP.
		ProjectID: ProjectID,

		MetricPrefix: MetricPrefix,

		// Set DefaultMonitoringLabels to an empty map to avoid getting the "opencensus_task" label
		DefaultMonitoringLabels: &stackdriver.Labels{},
	}
}

// pushMetricsData is a wrapper method on StackdriverExporter.PushMetricsProto
func (se *stackdriverExporter) pushMetricsData(ctx context.Context, md consumerdata.MetricsData) (int, error) {
	return se.exporter.PushMetricsProto(ctx, md.Node, md.Resource, md.Metrics)
}

// TODO(songya): add an interface PushSpanProto to Stackdriver exporter and remove this method
// pushTraceData is a wrapper method on StackdriverExporter.PushSpans
func (se *stackdriverExporter) pushTraceData(ctx context.Context, td consumerdata.TraceData) (int, error) {
	var errs []error
	goodSpans := 0
	for _, span := range td.Spans {
		sd, err := spandatatranslator.ProtoSpanToOCSpanData(span)
		if err == nil {
			se.exporter.ExportSpan(sd)
			goodSpans++
		} else {
			errs = append(errs, err)
		}
	}

	return len(td.Spans) - goodSpans, oterr.CombineErrors(errs)
}

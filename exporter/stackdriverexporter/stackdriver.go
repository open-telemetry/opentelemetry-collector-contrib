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
// exporter to be used in opentelemetry-collector.
package stackdriverexporter

import (
	"context"
	"fmt"

	"contrib.go.opencensus.io/exporter/stackdriver"
	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

// stackdriverExporter is a wrapper struct of Stackdriver exporter
type stackdriverExporter struct {
	mexporter *stackdriver.Exporter
	texporter *cloudtrace.Exporter
}

func (*stackdriverExporter) Name() string {
	return "stackdriver"
}

func (se *stackdriverExporter) Shutdown(context.Context) error {
	se.mexporter.Flush()
	se.mexporter.StopMetricsExporter()
	return nil
}

func newStackdriverTraceExporter(cfg *Config) (component.TraceExporter, error) {
	topts := []cloudtrace.Option{
		cloudtrace.WithProjectID(cfg.ProjectID),
	}
	if cfg.Endpoint != "" {
		var copts []option.ClientOption
		if cfg.UseInsecure {
			conn, err := grpc.Dial(cfg.Endpoint, grpc.WithInsecure())
			if err != nil {
				return nil, fmt.Errorf("cannot configure grpc conn: %v", err)
			}
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(cfg.Endpoint))
		}
		topts = append(topts, cloudtrace.WithTraceClientOptions(copts))
	}
	if cfg.NumOfWorkers > 0 {
		topts = append(topts, cloudtrace.WithMaxNumberOfWorkers(cfg.NumOfWorkers))
	}
	exp, err := cloudtrace.NewExporter(topts...)
	if err != nil {
		return nil, fmt.Errorf("error creating Stackdriver Trace exporter: %v", err)
	}
	tExp := &stackdriverExporter{texporter: exp}

	return exporterhelper.NewTraceExporter(
		cfg,
		tExp.pushTraces,
		exporterhelper.WithShutdown(tExp.Shutdown))
}

func newStackdriverMetricsExporter(cfg *Config) (component.MetricsExporter, error) {
	// TODO:  For each ProjectID, create a different exporter
	// or at least a unique Stackdriver client per ProjectID.
	options := stackdriver.Options{
		// If the project ID is an empty string, it will be set by default based on
		// the project this is running on in GCP.
		ProjectID: cfg.ProjectID,

		MetricPrefix: cfg.Prefix,

		// Set DefaultMonitoringLabels to an empty map to avoid getting the "opencensus_task" label
		DefaultMonitoringLabels: &stackdriver.Labels{},
	}
	if cfg.Endpoint != "" {
		if cfg.UseInsecure {
			conn, err := grpc.Dial(cfg.Endpoint, grpc.WithInsecure())
			if err != nil {
				return nil, fmt.Errorf("cannot configure grpc conn: %v", err)
			}
			options.TraceClientOptions = []option.ClientOption{option.WithGRPCConn(conn)}
			options.MonitoringClientOptions = []option.ClientOption{option.WithGRPCConn(conn)}
		} else {
			options.TraceClientOptions = []option.ClientOption{option.WithEndpoint(cfg.Endpoint)}
			options.MonitoringClientOptions = []option.ClientOption{option.WithEndpoint(cfg.Endpoint)}
		}
	}
	if cfg.NumOfWorkers > 0 {
		options.NumberOfWorkers = cfg.NumOfWorkers
	}
	if cfg.SkipCreateMetricDescriptor {
		options.SkipCMD = true
	}
	if len(cfg.ResourceMappings) > 0 {
		rm := resourceMapper{
			mappings: cfg.ResourceMappings,
		}
		options.MapResource = rm.mapResource
	}
	sde, serr := stackdriver.NewExporter(options)
	if serr != nil {
		return nil, fmt.Errorf("cannot configure Stackdriver metric exporter: %v", serr)
	}
	mExp := &stackdriverExporter{mexporter: sde}

	return exporterhelper.NewMetricsExporter(
		cfg,
		mExp.pushMetrics,
		exporterhelper.WithShutdown(mExp.Shutdown))
}

// pushMetricsData is a wrapper method on StackdriverExporter.PushMetricsProto
func (se *stackdriverExporter) pushMetricsData(ctx context.Context, md consumerdata.MetricsData) (int, error) {
	return se.mexporter.PushMetricsProto(ctx, md.Node, md.Resource, md.Metrics)
}

// pushMetrics calls pushMetricsData on each element of the given metrics
func (se *stackdriverExporter) pushMetrics(ctx context.Context, m pdata.Metrics) (int, error) {
	mds := pdatautil.MetricsToMetricsData(m)
	dropped := 0
	for _, md := range mds {
		d, err := se.pushMetricsData(ctx, md)
		dropped += d
		if err != nil {
			return dropped, err
		}
	}
	return dropped, nil
}

// pushTraces calls texporter.ExportSpan for each span in the given traces
func (se *stackdriverExporter) pushTraces(ctx context.Context, td pdata.Traces) (int, error) {
	var errs []error
	resourceSpans := td.ResourceSpans()
	numSpans := td.SpanCount()
	goodSpans := 0
	spans := make([]*export.SpanData, 0, numSpans)

	for i := 0; i < resourceSpans.Len(); i++ {
		sd, err := pdataResourceSpansToOTSpanData(resourceSpans.At(i))
		if err == nil {
			spans = append(spans, sd...)
		} else {
			errs = append(errs, err)
		}
	}

	for _, span := range spans {
		se.texporter.ExportSpan(ctx, span)
		goodSpans++
	}

	return numSpans - goodSpans, componenterror.CombineErrors(errs)
}

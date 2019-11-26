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
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
	"github.com/open-telemetry/opentelemetry-collector/oterr"
	spandatatranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace/spandata"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
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
	sde, serr := newStackdriverExporter(cfg)
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
	sde, serr := newStackdriverExporter(cfg)
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

func newStackdriverExporter(cfg *Config) (*stackdriver.Exporter, error) {
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
		dOpts := []option.ClientOption{}
		if cfg.UseInsecure {
			dOpts = append(dOpts, option.WithGRPCDialOption(grpc.WithInsecure()))
		}
		dOpts = append(dOpts, option.WithEndpoint(cfg.Endpoint))
		options.TraceClientOptions = dOpts
		options.MonitoringClientOptions = dOpts
	}
	if cfg.NumOfWorkers > 0 {
		options.NumberOfWorkers = cfg.NumOfWorkers
	}
	if cfg.SkipCreateMetricDescriptor {
		options.SkipCMD = true
	}
	return stackdriver.NewExporter(options)
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

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
	"strings"
	"time"

	"contrib.go.opencensus.io/exporter/stackdriver"
	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/internaldata"
	traceexport "go.opentelemetry.io/otel/sdk/export/trace"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

const name = "stackdriver"

// traceExporter is a wrapper struct of OT cloud trace exporter
type traceExporter struct {
	texporter *cloudtrace.Exporter
}

// metricsExporter is a wrapper struct of OC stackdriver exporter
type metricsExporter struct {
	mexporter *stackdriver.Exporter
}

func (*traceExporter) Name() string {
	return name
}

func (*metricsExporter) Name() string {
	return name
}

func (te *traceExporter) Shutdown(ctx context.Context) error {
	return te.texporter.Shutdown(ctx)
}

func (me *metricsExporter) Shutdown(context.Context) error {
	me.mexporter.Flush()
	me.mexporter.StopMetricsExporter()
	return nil
}

func setVersionInUserAgent(cfg *Config, version string) {
	cfg.UserAgent = strings.ReplaceAll(cfg.UserAgent, "{{version}}", version)
}

func generateClientOptions(cfg *Config) ([]option.ClientOption, error) {
	var copts []option.ClientOption
	// option.WithUserAgent is used by the Trace exporter, but not the Metric exporter (see comment below)
	if cfg.UserAgent != "" {
		copts = append(copts, option.WithUserAgent(cfg.UserAgent))
	}
	if cfg.Endpoint != "" {
		if cfg.UseInsecure {
			// option.WithGRPCConn option takes precedent over all other supplied options so the
			// following user agent will be used by both exporters if we reach this branch
			var dialOpts []grpc.DialOption
			if cfg.UserAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(cfg.UserAgent))
			}
			conn, err := grpc.Dial(cfg.Endpoint, append(dialOpts, grpc.WithInsecure())...)
			if err != nil {
				return nil, fmt.Errorf("cannot configure grpc conn: %w", err)
			}
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(cfg.Endpoint))
		}
	}
	if cfg.GetClientOptions != nil {
		copts = append(copts, cfg.GetClientOptions()...)
	}
	return copts, nil
}

func newStackdriverTraceExporter(cfg *Config, params component.ExporterCreateParams) (component.TracesExporter, error) {
	setVersionInUserAgent(cfg, params.ApplicationStartInfo.Version)

	topts := []cloudtrace.Option{
		cloudtrace.WithProjectID(cfg.ProjectID),
		cloudtrace.WithTimeout(cfg.Timeout),
	}

	copts, err := generateClientOptions(cfg)
	if err != nil {
		return nil, err
	}
	topts = append(topts, cloudtrace.WithTraceClientOptions(copts))
	if cfg.NumOfWorkers > 0 {
		topts = append(topts, cloudtrace.WithMaxNumberOfWorkers(cfg.NumOfWorkers))
	}

	topts, err = appendBundleOptions(topts, cfg.TraceConfig)
	if err != nil {
		return nil, err
	}

	exp, err := cloudtrace.NewExporter(topts...)
	if err != nil {
		return nil, fmt.Errorf("error creating Stackdriver Trace exporter: %w", err)
	}

	tExp := &traceExporter{texporter: exp}

	return exporterhelper.NewTraceExporter(
		cfg,
		params.Logger,
		tExp.pushTraces,
		exporterhelper.WithShutdown(tExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}))
}

func appendBundleOptions(topts []cloudtrace.Option, cfg TraceConfig) ([]cloudtrace.Option, error) {
	topts, err := validateAndAppendDurationOption(topts, "BundleDelayThreshold", cfg.BundleDelayThreshold, cloudtrace.WithBundleDelayThreshold(cfg.BundleDelayThreshold))
	if err != nil {
		return nil, err
	}

	topts, err = validateAndAppendIntOption(topts, "BundleCountThreshold", cfg.BundleCountThreshold, cloudtrace.WithBundleCountThreshold(cfg.BundleCountThreshold))
	if err != nil {
		return nil, err
	}

	topts, err = validateAndAppendIntOption(topts, "BundleByteThreshold", cfg.BundleByteThreshold, cloudtrace.WithBundleByteThreshold(cfg.BundleByteThreshold))
	if err != nil {
		return nil, err
	}

	topts, err = validateAndAppendIntOption(topts, "BundleByteLimit", cfg.BundleByteLimit, cloudtrace.WithBundleByteLimit(cfg.BundleByteLimit))
	if err != nil {
		return nil, err
	}

	topts, err = validateAndAppendIntOption(topts, "BufferMaxBytes", cfg.BufferMaxBytes, cloudtrace.WithBufferMaxBytes(cfg.BufferMaxBytes))
	if err != nil {
		return nil, err
	}

	return topts, nil
}

func validateAndAppendIntOption(topts []cloudtrace.Option, name string, val int, opt cloudtrace.Option) ([]cloudtrace.Option, error) {
	if val < 0 {
		return nil, fmt.Errorf("invalid value for: %s", name)
	}

	if val > 0 {
		topts = append(topts, opt)
	}

	return topts, nil
}

func validateAndAppendDurationOption(topts []cloudtrace.Option, name string, val time.Duration, opt cloudtrace.Option) ([]cloudtrace.Option, error) {
	if val < 0 {
		return nil, fmt.Errorf("invalid value for: %s", name)
	}

	if val > 0 {
		topts = append(topts, opt)
	}

	return topts, nil
}

func newStackdriverMetricsExporter(cfg *Config, params component.ExporterCreateParams) (component.MetricsExporter, error) {
	setVersionInUserAgent(cfg, params.ApplicationStartInfo.Version)

	// TODO:  For each ProjectID, create a different exporter
	// or at least a unique Stackdriver client per ProjectID.
	options := stackdriver.Options{
		// If the project ID is an empty string, it will be set by default based on
		// the project this is running on in GCP.
		ProjectID: cfg.ProjectID,

		MetricPrefix: cfg.MetricConfig.Prefix,

		// Set DefaultMonitoringLabels to an empty map to avoid getting the "opencensus_task" label
		DefaultMonitoringLabels: &stackdriver.Labels{},

		Timeout: cfg.Timeout,
	}

	// note options.UserAgent overrides the option.WithUserAgent client option in the Metric exporter
	if cfg.UserAgent != "" {
		options.UserAgent = cfg.UserAgent
	}

	copts, err := generateClientOptions(cfg)
	if err != nil {
		return nil, err
	}
	options.TraceClientOptions = copts
	options.MonitoringClientOptions = copts

	if cfg.NumOfWorkers > 0 {
		options.NumberOfWorkers = cfg.NumOfWorkers
	}
	if cfg.MetricConfig.SkipCreateMetricDescriptor {
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
		return nil, fmt.Errorf("cannot configure Stackdriver metric exporter: %w", serr)
	}
	mExp := &metricsExporter{mexporter: sde}

	return exporterhelper.NewMetricsExporter(
		cfg,
		params.Logger,
		mExp.pushMetrics,
		exporterhelper.WithShutdown(mExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}))
}

// pushMetrics calls StackdriverExporter.PushMetricsProto on each element of the given metrics
func (me *metricsExporter) pushMetrics(ctx context.Context, m pdata.Metrics) (int, error) {
	// PushMetricsProto doesn't bundle subsequent calls, so we need to
	// combine the data here to avoid generating too many RPC calls.
	mds := internaldata.MetricsToOC(m)
	count := 0
	for _, md := range mds {
		count += len(md.Metrics)
	}
	metrics := make([]*metricspb.Metric, 0, count)
	for _, md := range mds {
		if md.Resource == nil {
			metrics = append(metrics, md.Metrics...)
			continue
		}
		for _, metric := range md.Metrics {
			if metric.Resource == nil {
				metric.Resource = md.Resource
			}
			metrics = append(metrics, metric)
		}
	}
	points := numPoints(metrics)
	// The two nil args here are: node (which is ignored) and resource
	// (which we just moved to individual metrics).
	dropped, err := me.mexporter.PushMetricsProto(ctx, nil, nil, metrics)
	recordPointCount(ctx, points-dropped, dropped, err)
	return dropped, err
}

// pushTraces calls texporter.ExportSpan for each span in the given traces
func (te *traceExporter) pushTraces(ctx context.Context, td pdata.Traces) (int, error) {
	var errs []error
	resourceSpans := td.ResourceSpans()
	numSpans := td.SpanCount()
	spans := make([]*traceexport.SpanSnapshot, 0, numSpans)

	for i := 0; i < resourceSpans.Len(); i++ {
		sd := pdataResourceSpansToOTSpanData(resourceSpans.At(i))
		spans = append(spans, sd...)
	}

	err := te.texporter.ExportSpans(ctx, spans)
	if err != nil {
		errs = append(errs, err)
	}
	return numSpans - len(spans), componenterror.CombineErrors(errs)
}

func numPoints(metrics []*metricspb.Metric) int {
	numPoints := 0
	for _, metric := range metrics {
		tss := metric.GetTimeseries()
		for _, ts := range tss {
			numPoints += len(ts.GetPoints())
		}
	}
	return numPoints
}

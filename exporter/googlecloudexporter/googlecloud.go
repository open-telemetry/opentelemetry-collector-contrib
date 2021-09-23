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

// Package googlecloudexporter contains the wrapper for OpenTelemetry-GoogleCloud
// exporter to be used in opentelemetry-collector.
package googlecloudexporter

import (
	"context"
	"fmt"
	"strings"

	"contrib.go.opencensus.io/exporter/stackdriver"
	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

// traceExporter is a wrapper struct of OT cloud trace exporter
type traceExporter struct {
	texporter *cloudtrace.Exporter
}

// metricsExporter is a wrapper struct of OC stackdriver exporter
type metricsExporter struct {
	mexporter *stackdriver.Exporter
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

func newGoogleCloudTracesExporter(cfg *Config, set component.ExporterCreateSettings) (component.TracesExporter, error) {
	setVersionInUserAgent(cfg, set.BuildInfo.Version)

	topts := []cloudtrace.Option{
		cloudtrace.WithProjectID(cfg.ProjectID),
		cloudtrace.WithTimeout(cfg.Timeout),
	}

	copts, err := generateClientOptions(cfg)
	if err != nil {
		return nil, err
	}
	topts = append(topts, cloudtrace.WithTraceClientOptions(copts))

	exp, err := cloudtrace.New(topts...)
	if err != nil {
		return nil, fmt.Errorf("error creating GoogleCloud Trace exporter: %w", err)
	}

	tExp := &traceExporter{texporter: exp}

	return exporterhelper.NewTracesExporter(
		cfg,
		set,
		tExp.pushTraces,
		exporterhelper.WithShutdown(tExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings))
}

func newGoogleCloudMetricsExporter(cfg *Config, set component.ExporterCreateSettings) (component.MetricsExporter, error) {
	setVersionInUserAgent(cfg, set.BuildInfo.Version)

	// TODO:  For each ProjectID, create a different exporter
	// or at least a unique Google Cloud client per ProjectID.
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
		return nil, fmt.Errorf("cannot configure Google Cloud metric exporter: %w", serr)
	}
	mExp := &metricsExporter{mexporter: sde}

	return exporterhelper.NewMetricsExporter(
		cfg,
		set,
		mExp.pushMetrics,
		exporterhelper.WithShutdown(mExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings))
}

// pushMetrics calls StackdriverExporter.PushMetricsProto on each element of the given metrics
func (me *metricsExporter) pushMetrics(ctx context.Context, m pdata.Metrics) error {
	rms := m.ResourceMetrics()
	mds := make([]*agentmetricspb.ExportMetricsServiceRequest, 0, rms.Len())
	for i := 0; i < rms.Len(); i++ {
		emsr := &agentmetricspb.ExportMetricsServiceRequest{}
		emsr.Node, emsr.Resource, emsr.Metrics = internaldata.ResourceMetricsToOC(rms.At(i))
		mds = append(mds, emsr)
	}
	// PushMetricsProto doesn't bundle subsequent calls, so we need to
	// combine the data here to avoid generating too many RPC calls.
	mds = exportAdditionalLabels(mds)

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
	return err
}

func exportAdditionalLabels(mds []*agentmetricspb.ExportMetricsServiceRequest) []*agentmetricspb.ExportMetricsServiceRequest {
	for _, md := range mds {
		if md.Resource == nil ||
			md.Resource.Labels == nil ||
			md.Node == nil ||
			md.Node.Identifier == nil ||
			len(md.Node.Identifier.HostName) == 0 {
			continue
		}
		// MetricsToOC removes `host.name` label and writes it to node indentifier, here we reintroduce it.
		md.Resource.Labels[conventions.AttributeHostName] = md.Node.Identifier.HostName
	}
	return mds
}

// pushTraces calls texporter.ExportSpan for each span in the given traces
func (te *traceExporter) pushTraces(ctx context.Context, td pdata.Traces) error {
	var errs []error
	resourceSpans := td.ResourceSpans()
	spans := make([]sdktrace.ReadOnlySpan, 0, td.SpanCount())
	for i := 0; i < resourceSpans.Len(); i++ {
		sd := pdataResourceSpansToOTSpanData(resourceSpans.At(i))
		spans = append(spans, sd...)
	}

	err := te.texporter.ExportSpans(ctx, spans)
	if err != nil {
		errs = append(errs, err)
	}
	return consumererror.Combine(errs)
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

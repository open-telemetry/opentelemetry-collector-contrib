// Copyright The OpenTelemetry Authors
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

package otel2influx // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/otel2influx"

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/groupcache/lru"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/common"
)

const (
	jdgMeasurementDependencyLinks = "jaeger-dependencylinks"
	jdgMeasurementSpansQueueDepth = "jaeger-dependencylinks-spans-queuedepth"
	jdgMeasurementSpansDropped    = "jaeger-dependencylinks-spans-dropped"
)

var jdgFieldKeys = map[string]string{
	jdgMeasurementDependencyLinks: common.AttributeCallCount,
	jdgMeasurementSpansQueueDepth: common.AttributeSpansQueueDepth,
	jdgMeasurementSpansDropped:    common.AttributeSpansDropped,
}

type jdgSpan struct {
	serviceName string
	childIDs    []pcommon.SpanID
}

type jdgSpanReport struct {
	traceID      pcommon.TraceID
	spanID       pcommon.SpanID
	parentSpanID pcommon.SpanID
	serviceName  string
}

type JaegerDependencyGraph struct {
	logger common.Logger

	traceGraphByID *lru.Cache // trace ID -> trace graph
	ch             chan *jdgSpanReport

	backgroundCtx       context.Context
	backgroundCtxCancel func()
	backgroundErrs      chan error

	meterReader            metric.Reader
	meterProvider          *metric.MeterProvider
	counterDependencyLinks instrument.Int64Counter
	gaugeSpansQueueDepth   instrument.Int64ObservableGauge
	gaugeSpansDropped      instrument.Int64Counter

	w InfluxWriter
}

func NewJaegerDependencyGraph(logger common.Logger, cacheMaxTrace, queueLength int, w InfluxWriter) (*JaegerDependencyGraph, error) {
	meterReader := metric.NewManualReader(
		metric.WithTemporalitySelector(
			func(kind metric.InstrumentKind) metricdata.Temporality {
				return metricdata.DeltaTemporality
			}))

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(meterReader),
		metric.WithResource(resource.Empty()))

	meter := meterProvider.Meter(jdgMeasurementDependencyLinks)
	dependencyLinks, err := meter.Int64Counter(jdgMeasurementDependencyLinks)
	if err != nil {
		return nil, err
	}
	spansQueueDepth, err := meter.Int64ObservableGauge(jdgMeasurementSpansQueueDepth)
	if err != nil {
		return nil, err
	}
	spansDropped, err := meter.Int64Counter(jdgMeasurementSpansDropped)
	if err != nil {
		return nil, err
	}
	backgroundCtx, backgroundCtxCancel := context.WithCancel(context.Background())

	g := &JaegerDependencyGraph{
		logger: logger,

		traceGraphByID:      lru.New(cacheMaxTrace),
		ch:                  make(chan *jdgSpanReport, queueLength),
		backgroundCtx:       backgroundCtx,
		backgroundCtxCancel: backgroundCtxCancel,
		backgroundErrs:      make(chan error),

		meterReader:            meterReader,
		meterProvider:          meterProvider,
		counterDependencyLinks: dependencyLinks,
		gaugeSpansQueueDepth:   spansQueueDepth,
		gaugeSpansDropped:      spansDropped,

		w: w,
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, o api.Observer) error {
		o.ObserveInt64(spansQueueDepth, int64(len(g.ch)))
		return nil
	}, spansQueueDepth)
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *JaegerDependencyGraph) Start(_ context.Context, _ component.Host) error {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ticker.C:
				resourceMetrics := metricdata.ResourceMetrics{}
				err := g.meterReader.Collect(g.backgroundCtx, &resourceMetrics)
				if err != nil {
					g.logger.Debug("dependency graph reader failed to collect", err)
					continue
				}
				err = g.export(g.backgroundCtx, resourceMetrics)
				if err != nil {
					g.logger.Debug("dependency graph failed to export", err)
					continue
				}
			case <-g.backgroundCtx.Done():
				g.backgroundErrs <- g.backgroundCtx.Err()
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case s := <-g.ch:
				g.handleReportedSpan(g.backgroundCtx, s)
			case <-g.backgroundCtx.Done():
				g.backgroundErrs <- g.backgroundCtx.Err()
				return
			}
		}
	}()

	return nil
}

func (g *JaegerDependencyGraph) Shutdown(ctx context.Context) error {
	g.backgroundCtxCancel()
	return multierr.Combine(
		<-g.backgroundErrs,
		<-g.backgroundErrs,
		g.meterReader.Shutdown(ctx),
		g.meterProvider.Shutdown(ctx),
	)
}

func (g *JaegerDependencyGraph) export(ctx context.Context, resourceMetrics metricdata.ResourceMetrics) error {
	resourceTags := make(map[string]string, resourceMetrics.Resource.Len())
	for _, kv := range resourceMetrics.Resource.Attributes() {
		resourceTags[string(kv.Key)] = kv.Value.Emit()
	}
	batch := g.w.NewBatch()
	for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
		for _, metrics := range scopeMetrics.Metrics {
			tags := make(map[string]string, len(resourceTags)+2)
			for k, v := range resourceTags {
				tags[k] = v
			}
			var dataPoints []metricdata.DataPoint[int64]
			switch data := metrics.Data.(type) {
			case metricdata.Sum[int64]:
				dataPoints = data.DataPoints
			case metricdata.Gauge[int64]:
				dataPoints = data.DataPoints
			default:
				g.logger.Debug("unsupported metric type", "type", fmt.Sprintf("%T", data))
				continue
			}
			for _, dp := range dataPoints {
				for _, kv := range dp.Attributes.ToSlice() {
					tags[string(kv.Key)] = kv.Value.Emit()
				}
				fields := map[string]interface{}{
					jdgFieldKeys[metrics.Name]: dp.Value,
				}
				err := batch.WritePoint(ctx, metrics.Name, tags, fields, dp.Time, common.InfluxMetricValueTypeUntyped)
				if err != nil {
					return err
				}
			}
		}
	}
	return batch.FlushBatch(ctx)
}

func (g *JaegerDependencyGraph) handleReportedSpan(ctx context.Context, report *jdgSpanReport) {
	var traceGraph map[pcommon.SpanID]*jdgSpan
	if v, ok := g.traceGraphByID.Get(report.traceID); ok {
		traceGraph = v.(map[pcommon.SpanID]*jdgSpan)
	} else {
		traceGraph = make(map[pcommon.SpanID]*jdgSpan)
		g.traceGraphByID.Add(report.traceID, traceGraph)
	}

	// for each of this span's children...
	this := traceGraph[report.spanID]
	if this == nil {
		this = &jdgSpan{}
		traceGraph[report.spanID] = this
	} else {
		for _, childSpanID := range this.childIDs {
			child := traceGraph[childSpanID]
			if child.serviceName != "" && child.serviceName != report.serviceName {
				g.counterDependencyLinks.Add(ctx, 1,
					attribute.String(common.AttributeParentServiceName, report.serviceName),
					attribute.String(common.AttributeChildServiceName, child.serviceName))
			}
		}
	}
	this.serviceName = report.serviceName

	// for each of this span's parents (0 or 1)...
	if report.parentSpanID.IsEmpty() {
		return
	}
	parent := traceGraph[report.parentSpanID]
	if parent == nil {
		parent = &jdgSpan{}
		traceGraph[report.parentSpanID] = parent
	} else if parent.serviceName != "" && parent.serviceName != report.serviceName {
		g.counterDependencyLinks.Add(ctx, 1,
			attribute.String(common.AttributeParentServiceName, parent.serviceName),
			attribute.String(common.AttributeChildServiceName, report.serviceName))
	}
	parent.childIDs = append(parent.childIDs, report.spanID)
}

func (g *JaegerDependencyGraph) ReportSpan(ctx context.Context, span ptrace.Span, resource pcommon.Resource) {
	traceID := span.TraceID()
	if traceID.IsEmpty() {
		return
	}
	spanID := span.SpanID()
	if spanID.IsEmpty() {
		return
	}
	serviceNameValue, ok := resource.Attributes().Get(semconv.AttributeServiceName)
	if !ok {
		return
	}
	serviceName := serviceNameValue.Str()
	if serviceName == "" {
		return
	}

	select {
	case g.ch <- &jdgSpanReport{
		traceID:      traceID,
		spanID:       spanID,
		parentSpanID: span.ParentSpanID(),
		serviceName:  serviceName,
	}:
	default:
		g.gaugeSpansDropped.Add(ctx, 1)
	}
	// TODO add span.links using span.kind?
}

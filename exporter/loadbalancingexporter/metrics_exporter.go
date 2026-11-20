// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/metric"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

var _ exporter.Metrics = (*metricExporterImp)(nil)

type exporterMetrics map[*wrappedExporter]pmetric.Metrics

type metricExporterImp struct {
	loadBalancer *loadBalancer
	routingKey   routingKey
	routingAttrs []string

	logger     *zap.Logger
	stopped    bool
	shutdownWg sync.WaitGroup
	telemetry  *metadata.TelemetryBuilder
}

func newMetricsExporter(params exporter.Settings, cfg component.Config) (*metricExporterImp, error) {
	telemetry, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	exporterFactory := otlpexporter.NewFactory()
	cfFunc := func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		oParams := buildExporterSettings(exporterFactory.Type(), params, endpoint)

		return exporterFactory.CreateMetrics(ctx, oParams, &oCfg)
	}

	lb, err := newLoadBalancer(params.Logger, cfg, cfFunc, telemetry)
	if err != nil {
		return nil, err
	}

	metricExporter := metricExporterImp{
		loadBalancer: lb,
		routingKey:   svcRouting,
		telemetry:    telemetry,
		logger:       params.Logger,
	}

	switch cfg.(*Config).RoutingKey {
	case svcRoutingStr, "":
		// default case for empty routing key
		metricExporter.routingKey = svcRouting
	case resourceRoutingStr:
		metricExporter.routingKey = resourceRouting
	case metricNameRoutingStr:
		metricExporter.routingKey = metricNameRouting
	case streamIDRoutingStr:
		metricExporter.routingKey = streamIDRouting
	case attrRoutingStr:
		metricExporter.routingKey = attrRouting
		metricExporter.routingAttrs = cfg.(*Config).RoutingAttributes
	default:
		return nil, fmt.Errorf("unsupported routing_key: %q", cfg.(*Config).RoutingKey)
	}
	return &metricExporter, nil
}

func (*metricExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *metricExporterImp) Start(ctx context.Context, host component.Host) error {
	return e.loadBalancer.Start(ctx, host)
}

func (e *metricExporterImp) Shutdown(ctx context.Context) error {
	err := e.loadBalancer.Shutdown(ctx)
	e.stopped = true
	e.shutdownWg.Wait()
	return err
}

func (e *metricExporterImp) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var batches map[string]pmetric.Metrics

	switch e.routingKey {
	case svcRouting:
		batches = splitMetricsByResourceServiceName(md)
	case resourceRouting:
		batches = splitMetricsByResourceID(md)
	case metricNameRouting:
		batches = splitMetricsByMetricName(md)
	case streamIDRouting:
		batches = splitMetricsByStreamID(md)
	case attrRouting:
		batches = splitMetricsByAttributes(md, e.routingAttrs)
	}

	// Now assign each batch to an exporter, and merge as we go. A Merger (as
	// opposed to repeated metrics.Merge calls) caches the identities of the
	// accumulated data, so merging N batches costs O(N) identity computations
	// instead of O(N^2) — with per-stream routing keys N can easily be in the
	// thousands per ConsumeMetrics call.
	mergersByExporter := map[*wrappedExporter]*metrics.Merger{}

	for routingID, mds := range batches {
		exp, _, err := e.loadBalancer.exporterAndEndpoint([]byte(routingID))
		if err != nil {
			return err
		}

		merger, ok := mergersByExporter[exp]
		if !ok {
			exp.consumeWG.Add(1)
			merger = metrics.NewMerger(pmetric.NewMetrics())
			mergersByExporter[exp] = merger
		}

		merger.Merge(mds)
	}

	metricsByExporter := make(exporterMetrics, len(mergersByExporter))
	for exp, merger := range mergersByExporter {
		metricsByExporter[exp] = merger.Metrics()
	}

	return e.exportBatches(ctx, metricsByExporter)
}

// exportToBackend sends md to one backend, records per-backend telemetry, and signals the
// exporter's consume wait group.
func (e *metricExporterImp) exportToBackend(ctx context.Context, exp *wrappedExporter, md pmetric.Metrics) error {
	start := time.Now()
	err := exp.ConsumeMetrics(ctx, md)
	duration := time.Since(start)
	exp.consumeWG.Done()
	e.telemetry.LoadbalancerBackendLatency.Record(ctx, duration.Milliseconds(), metric.WithAttributeSet(exp.endpointAttr))
	if err == nil {
		e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.successAttr))
	} else {
		e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.failureAttr))
		e.logger.Debug("failed to export metrics", zap.Error(err))
	}
	return err
}

// exportBatches sends each pre-routed batch to its backend. When any backend fails with a
// retryable error, it returns a consumererror.Metrics carrying only the retryable failed data and
// causes, so a retry re-sends neither delivered data nor data a backend rejected permanently.
// The permanently rejected data is dropped. When every failure is permanent, it
// returns a permanent error wrapping a consumererror.Metrics with the failed data.
func (e *metricExporterImp) exportBatches(ctx context.Context, batches exporterMetrics) error {
	var retryableErrs, permanentErrs []error
	var retryable, permanent []pmetric.Metrics
	for exp, mds := range batches {
		err := e.exportToBackend(ctx, exp, mds)
		if err == nil {
			continue
		}
		failed := failedMetricsFromError(err, mds)
		if consumererror.IsPermanent(err) {
			permanentErrs = append(permanentErrs, err)
			permanent = append(permanent, failed)
		} else {
			retryableErrs = append(retryableErrs, err)
			retryable = append(retryable, failed)
		}
	}
	if len(retryableErrs) > 0 {
		// Permanent causes must stay out of the returned error: consumererror.IsPermanent
		// searches the whole error tree and would stop the retry of the retryable data.
		return consumererror.NewMetrics(errors.Join(retryableErrs...), copyFailedMetrics(retryable))
	}
	if len(permanentErrs) > 0 {
		return consumererror.NewPermanent(consumererror.NewMetrics(errors.Join(permanentErrs...), copyFailedMetrics(permanent)))
	}
	return nil
}

func splitMetricsByResourceServiceName(md pmetric.Metrics) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		newMD := pmetric.NewMetrics()
		rmClone := newMD.ResourceMetrics().AppendEmpty()
		rm.CopyTo(rmClone)

		// A resource without a service name is routed by the hash of its resource
		// attributes (as with routing_key "resource") instead of being dropped.
		var key string
		if svc, ok := rm.Resource().Attributes().Get(string(conventions.ServiceNameKey)); ok && svc.Str() != "" {
			key = svc.Str()
		} else {
			key = identity.OfResource(rm.Resource()).String()
		}
		appendMetricsByKey(results, key, newMD)
	}

	return results
}

func splitMetricsByResourceID(md pmetric.Metrics) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		newMD := pmetric.NewMetrics()
		rmClone := newMD.ResourceMetrics().AppendEmpty()
		rm.CopyTo(rmClone)

		key := identity.OfResource(rm.Resource()).String()
		appendMetricsByKey(results, key, newMD)
	}

	return results
}

func splitMetricsByMetricName(md pmetric.Metrics) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)

				newMD, mClone := cloneMetricWithoutType(rm, sm, m)
				m.CopyTo(mClone)

				key := m.Name()
				appendMetricsByKey(results, key, newMD)
			}
		}
	}

	return results
}

func splitMetricsByStreamID(md pmetric.Metrics) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		res := rm.Resource()

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scope := sm.Scope()

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				metricID := identity.OfResourceMetric(res, scope, m)

				forEachMetricDataPoint(rm, sm, m, func(dp attrPoint, newMD pmetric.Metrics) {
					key := identity.OfStream(metricID, dp).String()
					appendMetricsByKey(results, key, newMD)
				})
			}
		}
	}

	return results
}

func splitMetricsByAttributes(md pmetric.Metrics, attrs []string) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceAttrs := rm.Resource().Attributes()

		var baseResourceKeyBuilder strings.Builder
		pendingResourceAttrs := make([]string, 0, len(attrs))
		for _, attr := range attrs {
			if val, ok := resourceAttrs.Get(attr); ok {
				baseResourceKeyBuilder.WriteString(buildAttributeRoutingKeyValue(attr, val))
				continue
			}

			pendingResourceAttrs = append(pendingResourceAttrs, attr)
		}
		baseResourceKey := baseResourceKeyBuilder.String()

		if len(pendingResourceAttrs) == 0 {
			// All split attributes are on resource, so no per-scope/datapoint keying.
			newMD := pmetric.NewMetrics()
			rmClone := newMD.ResourceMetrics().AppendEmpty()
			rm.CopyTo(rmClone)
			appendMetricsByKey(results, baseResourceKey, newMD)
			continue
		}

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeAttrs := sm.Scope().Attributes()

			var baseScopeKeyBuilder strings.Builder
			baseScopeKeyBuilder.WriteString(baseResourceKey)
			pendingScopeAttrs := make([]string, 0, len(pendingResourceAttrs))
			for _, attr := range pendingResourceAttrs {
				if val, ok := scopeAttrs.Get(attr); ok {
					baseScopeKeyBuilder.WriteString(buildAttributeRoutingKeyValue(attr, val))
					continue
				}

				pendingScopeAttrs = append(pendingScopeAttrs, attr)
			}
			baseScopeKey := baseScopeKeyBuilder.String()

			if len(pendingScopeAttrs) == 0 {
				// All split attributes are on resource/scope, so no per-datapoint keying.
				newMD := pmetric.NewMetrics()
				rmClone := newMD.ResourceMetrics().AppendEmpty()
				rm.Resource().CopyTo(rmClone.Resource())
				rmClone.SetSchemaUrl(rm.SchemaUrl())

				smClone := rmClone.ScopeMetrics().AppendEmpty()
				sm.CopyTo(smClone)

				appendMetricsByKey(results, baseScopeKey, newMD)
				continue
			}

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)

				forEachMetricDataPoint(rm, sm, m, func(dp attrPoint, newMD pmetric.Metrics) {
					var key strings.Builder
					key.WriteString(baseScopeKey)
					dpAttrs := dp.Attributes()
					for _, attr := range pendingScopeAttrs {
						if val, ok := dpAttrs.Get(attr); ok {
							key.WriteString(buildAttributeRoutingKeyValue(attr, val))
						} else {
							key.WriteString(buildAttributeRoutingKey(attr))
						}
					}

					appendMetricsByKey(results, key.String(), newMD)
				})
			}
		}
	}

	return results
}

func forEachMetricDataPoint(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric, fn func(dp attrPoint, md pmetric.Metrics)) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		gauge := m.Gauge()
		for i := 0; i < gauge.DataPoints().Len(); i++ {
			dp := gauge.DataPoints().At(i)

			newMD, mClone := cloneMetricWithoutType(rm, sm, m)
			gaugeClone := mClone.SetEmptyGauge()

			dpClone := gaugeClone.DataPoints().AppendEmpty()
			dp.CopyTo(dpClone)

			fn(dp, newMD)
		}
	case pmetric.MetricTypeSum:
		sum := m.Sum()
		for i := 0; i < sum.DataPoints().Len(); i++ {
			dp := sum.DataPoints().At(i)

			newMD, mClone := cloneMetricWithoutType(rm, sm, m)
			sumClone := mClone.SetEmptySum()
			sumClone.SetIsMonotonic(sum.IsMonotonic())
			sumClone.SetAggregationTemporality(sum.AggregationTemporality())

			dpClone := sumClone.DataPoints().AppendEmpty()
			dp.CopyTo(dpClone)

			fn(dp, newMD)
		}
	case pmetric.MetricTypeHistogram:
		histogram := m.Histogram()
		for i := 0; i < histogram.DataPoints().Len(); i++ {
			dp := histogram.DataPoints().At(i)

			newMD, mClone := cloneMetricWithoutType(rm, sm, m)
			histogramClone := mClone.SetEmptyHistogram()
			histogramClone.SetAggregationTemporality(histogram.AggregationTemporality())

			dpClone := histogramClone.DataPoints().AppendEmpty()
			dp.CopyTo(dpClone)

			fn(dp, newMD)
		}
	case pmetric.MetricTypeExponentialHistogram:
		expHistogram := m.ExponentialHistogram()
		for i := 0; i < expHistogram.DataPoints().Len(); i++ {
			dp := expHistogram.DataPoints().At(i)

			newMD, mClone := cloneMetricWithoutType(rm, sm, m)
			expHistogramClone := mClone.SetEmptyExponentialHistogram()
			expHistogramClone.SetAggregationTemporality(expHistogram.AggregationTemporality())

			dpClone := expHistogramClone.DataPoints().AppendEmpty()
			dp.CopyTo(dpClone)

			fn(dp, newMD)
		}
	case pmetric.MetricTypeSummary:
		summary := m.Summary()
		for i := 0; i < summary.DataPoints().Len(); i++ {
			dp := summary.DataPoints().At(i)

			newMD, mClone := cloneMetricWithoutType(rm, sm, m)
			sumClone := mClone.SetEmptySummary()

			dpClone := sumClone.DataPoints().AppendEmpty()
			dp.CopyTo(dpClone)

			fn(dp, newMD)
		}
	}
}

type attrPoint interface {
	Attributes() pcommon.Map
}

func appendMetricsByKey(results map[string]pmetric.Metrics, key string, mds pmetric.Metrics) {
	if existing, ok := results[key]; ok {
		metrics.Merge(existing, mds)
	} else {
		results[key] = mds
	}
}

func cloneMetricWithoutType(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) (md pmetric.Metrics, mClone pmetric.Metric) {
	md = pmetric.NewMetrics()

	rmClone := md.ResourceMetrics().AppendEmpty()
	rm.Resource().CopyTo(rmClone.Resource())
	rmClone.SetSchemaUrl(rm.SchemaUrl())

	smClone := rmClone.ScopeMetrics().AppendEmpty()
	sm.Scope().CopyTo(smClone.Scope())
	smClone.SetSchemaUrl(sm.SchemaUrl())

	mClone = smClone.Metrics().AppendEmpty()
	mClone.SetName(m.Name())
	mClone.SetDescription(m.Description())
	mClone.SetUnit(m.Unit())

	return md, mClone
}

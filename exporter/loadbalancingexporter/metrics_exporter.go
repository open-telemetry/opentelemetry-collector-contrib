// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/metric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

var _ exporter.Metrics = (*metricExporterImp)(nil)

type metricExporterImp struct {
	loadBalancer *loadBalancer
	routingKey   routingKey

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
		var errs []error
		batches, errs = splitMetricsByResourceServiceName(md)
		if len(errs) > 0 {
			for _, ee := range errs {
				e.logger.Error("failed to export metric", zap.Error(ee))
			}
			if len(batches) == 0 {
				return consumererror.NewPermanent(errors.Join(errs...))
			}
		}
	case resourceRouting:
		batches = splitMetricsByResourceID(md)
	case metricNameRouting:
		batches = splitMetricsByMetricName(md)
	case streamIDRouting:
		batches = splitMetricsByStreamID(md)
	}

	// Now assign each batch to an exporter, and merge as we go
	metricsByExporter := map[*wrappedExporter]pmetric.Metrics{}
	exporterEndpoints := map[*wrappedExporter]string{}

	for routingID, mds := range batches {
		exp, endpoint, err := e.loadBalancer.exporterAndEndpoint([]byte(routingID))
		if err != nil {
			return err
		}

		expMetrics, ok := metricsByExporter[exp]
		if !ok {
			exp.consumeWG.Add(1)
			expMetrics = pmetric.NewMetrics()
			metricsByExporter[exp] = expMetrics
			exporterEndpoints[exp] = endpoint
		}

		metrics.Merge(expMetrics, mds)
	}

	var errs error
	for exp, mds := range metricsByExporter {
		start := time.Now()
		err := exp.ConsumeMetrics(ctx, mds)
		duration := time.Since(start)

		exp.consumeWG.Done()
		errs = multierr.Append(errs, err)
		e.telemetry.LoadbalancerBackendLatency.Record(ctx, duration.Milliseconds(), metric.WithAttributeSet(exp.endpointAttr))
		if err == nil {
			e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.successAttr))
		} else {
			e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.failureAttr))
			e.logger.Debug("failed to export metrics", zap.Error(err))
		}
	}

	return errs
}

func splitMetricsByResourceServiceName(md pmetric.Metrics) (map[string]pmetric.Metrics, []error) {
	results := map[string]pmetric.Metrics{}
	var errs []error

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		svc, ok := rm.Resource().Attributes().Get(string(conventions.ServiceNameKey))
		if !ok {
			errs = append(errs, fmt.Errorf("unable to get service name from resource metric with attributes: %v", rm.Resource().Attributes().AsRaw()))
			continue
		}

		newMD := pmetric.NewMetrics()
		rmClone := newMD.ResourceMetrics().AppendEmpty()
		rm.CopyTo(rmClone)

		key := svc.Str()
		existing, ok := results[key]
		if ok {
			metrics.Merge(existing, newMD)
		} else {
			results[key] = newMD
		}
	}

	return results, errs
}

func splitMetricsByResourceID(md pmetric.Metrics) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		newMD := pmetric.NewMetrics()
		rmClone := newMD.ResourceMetrics().AppendEmpty()
		rm.CopyTo(rmClone)

		key := identity.OfResource(rm.Resource()).String()
		existing, ok := results[key]
		if ok {
			metrics.Merge(existing, newMD)
		} else {
			results[key] = newMD
		}
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
				existing, ok := results[key]
				if ok {
					metrics.Merge(existing, newMD)
				} else {
					results[key] = newMD
				}
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

				switch m.Type() {
				case pmetric.MetricTypeGauge:
					gauge := m.Gauge()

					for l := 0; l < gauge.DataPoints().Len(); l++ {
						dp := gauge.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						gaugeClone := mClone.SetEmptyGauge()

						dpClone := gaugeClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				case pmetric.MetricTypeSum:
					sum := m.Sum()

					for l := 0; l < sum.DataPoints().Len(); l++ {
						dp := sum.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						sumClone := mClone.SetEmptySum()
						sumClone.SetIsMonotonic(sum.IsMonotonic())
						sumClone.SetAggregationTemporality(sum.AggregationTemporality())

						dpClone := sumClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				case pmetric.MetricTypeHistogram:
					histogram := m.Histogram()

					for l := 0; l < histogram.DataPoints().Len(); l++ {
						dp := histogram.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						histogramClone := mClone.SetEmptyHistogram()
						histogramClone.SetAggregationTemporality(histogram.AggregationTemporality())

						dpClone := histogramClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					expHistogram := m.ExponentialHistogram()

					for l := 0; l < expHistogram.DataPoints().Len(); l++ {
						dp := expHistogram.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						expHistogramClone := mClone.SetEmptyExponentialHistogram()
						expHistogramClone.SetAggregationTemporality(expHistogram.AggregationTemporality())

						dpClone := expHistogramClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				case pmetric.MetricTypeSummary:
					summary := m.Summary()

					for l := 0; l < summary.DataPoints().Len(); l++ {
						dp := summary.DataPoints().At(l)

						newMD, mClone := cloneMetricWithoutType(rm, sm, m)
						sumClone := mClone.SetEmptySummary()

						dpClone := sumClone.DataPoints().AppendEmpty()
						dp.CopyTo(dpClone)

						key := identity.OfStream(metricID, dp).String()
						existing, ok := results[key]
						if ok {
							metrics.Merge(existing, newMD)
						} else {
							results[key] = newMD
						}
					}
				}
			}
		}
	}

	return results
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

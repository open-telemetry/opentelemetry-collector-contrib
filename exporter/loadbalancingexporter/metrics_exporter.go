// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

var _ exporter.Metrics = (*metricExporterImp)(nil)

type metricExporterImp struct {
	loadBalancer loadBalancer
	routingKey   routingKey

	stopped    bool
	shutdownWg sync.WaitGroup
}

func newMetricsExporter(params exporter.CreateSettings, cfg component.Config) (*metricExporterImp, error) {
	exporterFactory := otlpexporter.NewFactory()

	lb, err := newLoadBalancer(params, cfg, func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		return exporterFactory.CreateMetricsExporter(ctx, params, &oCfg)
	})
	if err != nil {
		return nil, err
	}

	metricExporter := metricExporterImp{loadBalancer: lb, routingKey: svcRouting}

	switch cfg.(*Config).RoutingKey {
	case "service", "":
		// default case for empty routing key
		metricExporter.routingKey = svcRouting
	case "resource":
		metricExporter.routingKey = resourceRouting
	case "metric":
		metricExporter.routingKey = metricNameRouting
	default:
		return nil, fmt.Errorf("unsupported routing_key: %q", cfg.(*Config).RoutingKey)
	}
	return &metricExporter, nil

}

func (e *metricExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *metricExporterImp) Start(ctx context.Context, host component.Host) error {
	return e.loadBalancer.Start(ctx, host)
}

func (e *metricExporterImp) Shutdown(context.Context) error {
	e.stopped = true
	e.shutdownWg.Wait()
	return nil
}

func (e *metricExporterImp) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errs error
	batches := batchpersignal.SplitMetrics(md)
	for _, batch := range batches {
		errs = multierr.Append(errs, e.consumeMetric(ctx, batch))
	}

	return errs
}

func (e *metricExporterImp) consumeMetric(ctx context.Context, md pmetric.Metrics) error {
	var exp component.Component
	routingIds, err := routingIdentifiersFromMetrics(md, e.routingKey)
	if err != nil {
		return err
	}
	for rid := range routingIds {
		endpoint := e.loadBalancer.Endpoint([]byte(rid))
		exp, err = e.loadBalancer.Exporter(endpoint)
		if err != nil {
			return err
		}

		te, ok := exp.(exporter.Metrics)
		if !ok {
			return fmt.Errorf("unable to export metrics, unexpected exporter type: expected exporter.Metrics but got %T", exp)
		}

		start := time.Now()
		err = te.ConsumeMetrics(ctx, md)
		duration := time.Since(start)

		if err == nil {
			_ = stats.RecordWithTags(
				ctx,
				[]tag.Mutator{tag.Upsert(endpointTagKey, endpoint), successTrueMutator},
				mBackendLatency.M(duration.Milliseconds()))
		} else {
			_ = stats.RecordWithTags(
				ctx,
				[]tag.Mutator{tag.Upsert(endpointTagKey, endpoint), successFalseMutator},
				mBackendLatency.M(duration.Milliseconds()))
		}
	}

	return err
}

func routingIdentifiersFromMetrics(mds pmetric.Metrics, key routingKey) (map[string]bool, error) {
	ids := make(map[string]bool)

	// no need to test "empty labels"
	// no need to test "empty resources"

	rs := mds.ResourceMetrics()
	if rs.Len() == 0 {
		return nil, errors.New("empty resource metrics")
	}

	ils := rs.At(0).ScopeMetrics()
	if ils.Len() == 0 {
		return nil, errors.New("empty scope metrics")
	}

	metrics := ils.At(0).Metrics()
	if metrics.Len() == 0 {
		return nil, errors.New("empty metrics")
	}

	for i := 0; i < rs.Len(); i++ {
		resource := rs.At(i).Resource()
		switch key {
		default:
		case svcRouting, traceIDRouting:
			svc, ok := resource.Attributes().Get(conventions.AttributeServiceName)
			if !ok {
				return nil, errors.New("unable to get service name")
			}
			ids[svc.Str()] = true
		case metricNameRouting:
			sm := rs.At(i).ScopeMetrics()
			for j := 0; j < sm.Len(); j++ {
				metrics := sm.At(j).Metrics()
				for k := 0; k < metrics.Len(); k++ {
					md := metrics.At(k)
					rKey := metricRoutingKey(md)
					ids[rKey] = true
				}
			}
		case resourceRouting:
			sm := rs.At(i).ScopeMetrics()
			for j := 0; j < sm.Len(); j++ {
				metrics := sm.At(j).Metrics()
				for k := 0; k < metrics.Len(); k++ {
					md := metrics.At(k)
					rKey := resourceRoutingKey(md, resource.Attributes())
					ids[rKey] = true
				}
			}
		}
	}

	return ids, nil

}

// maintain
func sortedMapAttrs(attrs pcommon.Map) []string {
	keys := make([]string, 0)
	for k := range attrs.AsRaw() {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	attrsHash := make([]string, 0)
	for _, k := range keys {
		attrsHash = append(attrsHash, k)
		if v, ok := attrs.Get(k); ok {
			attrsHash = append(attrsHash, v.AsString())
		}
	}
	return attrsHash
}

func resourceRoutingKey(md pmetric.Metric, attrs pcommon.Map) string {
	attrsHash := sortedMapAttrs(attrs)
	attrsHash = append(attrsHash, md.Name())
	routingRef := strings.Join(attrsHash, "")

	return routingRef
}

func metricRoutingKey(md pmetric.Metric) string {
	return md.Name()
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

const (
	pseudoAttrSpanName = "span.name"
	pseudoAttrSpanKind = "span.kind"
)

var _ exporter.Traces = (*traceExporterImp)(nil)

type exporterTraces map[*wrappedExporter]ptrace.Traces

type traceExporterImp struct {
	loadBalancer *loadBalancer
	routingKey   routingKey
	routingAttrs []string

	logger     *zap.Logger
	stopped    bool
	shutdownWg sync.WaitGroup
	telemetry  *metadata.TelemetryBuilder
}

// Create new traces exporter
func newTracesExporter(params exporter.Settings, cfg component.Config) (*traceExporterImp, error) {
	telemetry, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	exporterFactory := otlpexporter.NewFactory()
	cfFunc := func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		oParams := buildExporterSettings(exporterFactory.Type(), params, endpoint)

		return exporterFactory.CreateTraces(ctx, oParams, &oCfg)
	}

	lb, err := newLoadBalancer(params.Logger, cfg, cfFunc, telemetry)
	if err != nil {
		return nil, err
	}

	traceExporter := traceExporterImp{
		loadBalancer: lb,
		routingKey:   traceIDRouting,
		telemetry:    telemetry,
		logger:       params.Logger,
	}

	switch cfg.(*Config).RoutingKey {
	case svcRoutingStr:
		traceExporter.routingKey = svcRouting
	case attrRoutingStr:
		traceExporter.routingKey = attrRouting
		traceExporter.routingAttrs = cfg.(*Config).RoutingAttributes
	case traceIDRoutingStr, "":
	default:
		return nil, fmt.Errorf("unsupported routing_key: %s", cfg.(*Config).RoutingKey)
	}
	return &traceExporter, nil
}

func (*traceExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *traceExporterImp) Start(ctx context.Context, host component.Host) error {
	return e.loadBalancer.Start(ctx, host)
}

func (e *traceExporterImp) Shutdown(ctx context.Context) error {
	err := e.loadBalancer.Shutdown(ctx)
	e.stopped = true
	e.shutdownWg.Wait()
	return err
}

func (e *traceExporterImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	batches := batchpersignal.SplitTraces(td)

	exporterSegregatedTraces := make(exporterTraces)
	endpoints := make(map[*wrappedExporter]string)
	for _, batch := range batches {
		routingID, err := routingIdentifiersFromTraces(batch, e.routingKey, e.routingAttrs)
		if err != nil {
			return err
		}

		for rid := range routingID {
			exp, endpoint, err := e.loadBalancer.exporterAndEndpoint([]byte(rid))
			if err != nil {
				return err
			}

			_, ok := exporterSegregatedTraces[exp]
			if !ok {
				exp.consumeWG.Add(1)
				exporterSegregatedTraces[exp] = ptrace.NewTraces()
			}
			exporterSegregatedTraces[exp] = mergeTraces(exporterSegregatedTraces[exp], batch)

			endpoints[exp] = endpoint
		}
	}

	var errs error

	for exp, td := range exporterSegregatedTraces {
		start := time.Now()
		err := exp.ConsumeTraces(ctx, td)
		exp.consumeWG.Done()
		errs = multierr.Append(errs, err)
		duration := time.Since(start)
		e.telemetry.LoadbalancerBackendLatency.Record(ctx, duration.Milliseconds(), metric.WithAttributeSet(exp.endpointAttr))
		if err == nil {
			e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.successAttr))
		} else {
			e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.failureAttr))
			e.logger.Debug("failed to export traces", zap.Error(err))
		}
	}

	return errs
}

// routingIdentifiersFromTraces reads the traces and determines an identifier that can be used to define a position on the
// ring hash. It takes the routingKey, defining what type of routing should be used, and a series of attributes
// (optionally) used if the routingKey is attrRouting.
//
// only svcRouting and attrRouting are supported. For attrRouting, any attribute, as well the "pseudo" attributes span.name
// and span.kind are supported.
//
// In practice, makes the assumption that ptrace.Traces includes only one trace of each kind, in the "trace tree".
func routingIdentifiersFromTraces(td ptrace.Traces, rType routingKey, attrs []string) (map[string]bool, error) {
	ids := make(map[string]bool)
	rs := td.ResourceSpans()
	if rs.Len() == 0 {
		return nil, errors.New("empty resource spans")
	}

	ils := rs.At(0).ScopeSpans()
	if ils.Len() == 0 {
		return nil, errors.New("empty scope spans")
	}

	spans := ils.At(0).Spans()
	if spans.Len() == 0 {
		return nil, errors.New("empty spans")
	}
	// Determine how the key should be populated.
	switch rType {
	case traceIDRouting:
		// The simple case is the TraceID routing. In this case, we just use the string representation of the Trace ID.
		tid := spans.At(0).TraceID()
		ids[string(tid[:])] = true

		return ids, nil
	case svcRouting:
		// Service Name is still handled as an "attribute router", it's just expressed as a "pseudo attribute"
		attrs = []string{"service.name"}
	case attrRouting:
		// By default, we'll just use the input provided.
		break
	default:
		return nil, fmt.Errorf("unsupported routing_key: %d", rType)
	}

	// Composite the attributes together as a key.
	for i := 0; i < rs.Len(); i++ {
		// rKey will never return an error. See
		// 1. https://pkg.go.dev/bytes#Buffer.Write
		// 2. https://stackoverflow.com/a/70388629
		var rKey strings.Builder

		for _, a := range attrs {
			// resource spans
			rAttr, ok := rs.At(i).Resource().Attributes().Get(a)
			if ok {
				rKey.WriteString(rAttr.Str())
				continue
			}

			// ils or "instrumentation library spans"
			ils := rs.At(0).ScopeSpans()
			iAttr, ok := ils.At(0).Scope().Attributes().Get(a)
			if ok {
				rKey.WriteString(iAttr.Str())
				continue
			}

			// the lowest level span (or what engineers regularly interact with)
			spans := ils.At(0).Spans()

			if a == pseudoAttrSpanKind {
				rKey.WriteString(spans.At(0).Kind().String())

				continue
			}

			if a == pseudoAttrSpanName {
				rKey.WriteString(spans.At(0).Name())

				continue
			}

			sAttr, ok := spans.At(0).Attributes().Get(a)
			if ok {
				rKey.WriteString(sAttr.Str())
				continue
			}
		}

		// No matter what, there will be a key here (even if that key is "").
		ids[rKey.String()] = true
	}

	return ids, nil
}

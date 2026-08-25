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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
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
	case randomnessRoutingStr:
		traceExporter.routingKey = randomnessRouting
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
	// traceID and randomness routing compute their key per span, so they can route spans
	// directly into per-backend traces in a single pass, avoiding the SplitTraces +
	// mergeTraces round-trip.
	switch e.routingKey {
	case traceIDRouting:
		return e.consumeTracesPerSpan(ctx, td, spanTraceIDIdentifier)
	case randomnessRouting:
		return e.consumeTracesPerSpan(ctx, td, func(span ptrace.Span) []byte {
			return e.randomnessIdentifier(ctx, span)
		})
	}

	batches := batchpersignal.SplitTraces(td)

	exporterSegregatedTraces := make(exporterTraces)
	for _, batch := range batches {
		routingID, err := routingIdentifiersFromTraces(batch, e.routingKey, e.routingAttrs)
		if err != nil {
			// Release the consumeWG counts already added for backends collected so far;
			// otherwise Shutdown's consumeWG.Wait() would hang on the skipped Done() calls.
			for exp := range exporterSegregatedTraces {
				exp.consumeWG.Done()
			}
			return err
		}

		for rid := range routingID {
			exp, _, err := e.loadBalancer.exporterAndEndpoint([]byte(rid))
			if err != nil {
				// Release the consumeWG counts already added for backends collected so far;
				// otherwise Shutdown's consumeWG.Wait() would hang on the skipped Done() calls.
				for exp := range exporterSegregatedTraces {
					exp.consumeWG.Done()
				}
				return err
			}

			_, ok := exporterSegregatedTraces[exp]
			if !ok {
				exp.consumeWG.Add(1)
				exporterSegregatedTraces[exp] = ptrace.NewTraces()
			}
			exporterSegregatedTraces[exp] = mergeTraces(exporterSegregatedTraces[exp], batch)
		}
	}

	var errs error
	for exp, td := range exporterSegregatedTraces {
		errs = multierr.Append(errs, e.exportToBackend(ctx, exp, td))
	}
	return errs
}

// spanTraceIDIdentifier is the identifierFor function for traceID routing.
func spanTraceIDIdentifier(span ptrace.Span) []byte {
	tid := span.TraceID()
	return tid[:]
}

// randomnessIdentifier is the identifierFor function for randomness routing: the explicit
// randomness value (the 14-hex-digit tracestate ot=rv) when the span carries one, otherwise
// randomness derived from the trace ID, mirroring the W3C/OTel sampling specification. Traces
// sharing an rv route to the same backend; traces without one keep per-trace locality.
func (e *traceExporterImp) randomnessIdentifier(ctx context.Context, span ptrace.Span) []byte {
	if raw := span.TraceState().AsRaw(); raw != "" {
		w3c, err := sampling.NewW3CTraceState(raw)
		if rnd, ok := w3c.OTelValue().RValueRandomness(); ok {
			// Member- and value-level parse failures (e.g. a malformed th
			// beside a valid rv) keep the members that did parse, so a
			// recovered rv still routes the trace.
			return []byte(rnd.RValue())
		}
		if err != nil && rawTraceStateHasRValue(raw) {
			// Header-level failures (W3C syntax errors such as an uppercase
			// vendor key, or a header over the 1024-byte size limit) discard
			// the whole tracestate including any rv. Count only failures that
			// discarded an rv the header appeared to carry, i.e. traces that
			// actually fell back to trace ID randomness; a failed header that
			// never had an rv would have routed by trace ID regardless.
			e.telemetry.LoadbalancerRandomnessTracestateUnparseable.Add(ctx, 1)
		}
	}
	rnd := sampling.TraceIDToRandomness(span.TraceID())
	return []byte(rnd.RValue())
}

// rawTraceStateHasRValue reports whether a tracestate header appears to carry an explicit
// randomness value in its ot member. It is a best-effort scan for headers that failed W3C
// validation and so can't be parsed properly.
func rawTraceStateHasRValue(raw string) bool {
	for member := range strings.SplitSeq(raw, ",") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(member), "ot="); ok && strings.Contains(value, "rv:") {
			return true
		}
	}
	return false
}

// consumeTracesPerSpan routes each span to the backend for its identifier, accumulating spans
// directly into one ptrace.Traces per backend. It copies each span once and allocates a
// single ptrace.Traces per backend, versus one per trace on the SplitTraces path.
// identifierFor must yield the same value for every span of a trace; the backend is resolved
// once per trace ID and reused for the trace's remaining spans in the batch. That caching also
// bounds the identifier cost: randomness routing pays a tracestate validation and parse once
// per trace per batch, where traceID routing slices the trace ID bytes.
func (e *traceExporterImp) consumeTracesPerSpan(ctx context.Context, td ptrace.Traces, identifierFor func(ptrace.Span) []byte) error {
	// dest holds a backend's in-progress traces and the source resource/scope indices of the
	// ScopeSpans being appended to, so a contiguous run of the same source scope reuses it.
	type dest struct {
		traces ptrace.Traces
		curSS  ptrace.ScopeSpans
		rsIdx  int
		ssIdx  int
		active bool
	}
	dests := make(map[*wrappedExporter]*dest, e.loadBalancer.NumBackends())
	// Resolving a backend hashes the trace ID under a lock; cache it so spans that share a
	// trace ID resolve only once per batch.
	expByTID := make(map[pcommon.TraceID]*wrappedExporter)

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			ss := sss.At(j)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				tid := span.TraceID()

				exp, ok := expByTID[tid]
				if !ok {
					var err error
					exp, _, err = e.loadBalancer.exporterAndEndpoint(identifierFor(span))
					if err != nil {
						// Release the consumeWG counts already added for backends collected so
						// far; otherwise Shutdown's consumeWG.Wait() would hang on the skipped
						// Done() calls.
						for exp := range dests {
							exp.consumeWG.Done()
						}
						return err
					}
					expByTID[tid] = exp
				}

				d, ok := dests[exp]
				if !ok {
					exp.consumeWG.Add(1)
					d = &dest{traces: ptrace.NewTraces()}
					dests[exp] = d
				}

				// Start a new ResourceSpans/ScopeSpans when the source resource/scope changes.
				if !d.active || d.rsIdx != i || d.ssIdx != j {
					destRS := d.traces.ResourceSpans().AppendEmpty()
					rs.Resource().CopyTo(destRS.Resource())
					destRS.SetSchemaUrl(rs.SchemaUrl())
					d.curSS = destRS.ScopeSpans().AppendEmpty()
					ss.Scope().CopyTo(d.curSS.Scope())
					d.curSS.SetSchemaUrl(ss.SchemaUrl())
					d.rsIdx, d.ssIdx, d.active = i, j, true
				}

				span.CopyTo(d.curSS.Spans().AppendEmpty())
			}
		}
	}

	var errs error
	for exp, d := range dests {
		errs = multierr.Append(errs, e.exportToBackend(ctx, exp, d.traces))
	}
	return errs
}

// exportToBackend sends td to one backend, records per-backend telemetry, and signals the
// exporter's consume wait group.
func (e *traceExporterImp) exportToBackend(ctx context.Context, exp *wrappedExporter, td ptrace.Traces) error {
	start := time.Now()
	err := exp.ConsumeTraces(ctx, td)
	exp.consumeWG.Done()
	duration := time.Since(start)
	e.telemetry.LoadbalancerBackendLatency.Record(ctx, duration.Milliseconds(), metric.WithAttributeSet(exp.endpointAttr))
	if err == nil {
		e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.successAttr))
	} else {
		e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(exp.failureAttr))
		e.logger.Debug("failed to export traces", zap.Error(err))
	}
	return err
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
				rKey.WriteString(buildAttributeRoutingKeyValue(a, rAttr))
				continue
			}

			// ils or "instrumentation library spans"
			ils := rs.At(0).ScopeSpans()
			iAttr, ok := ils.At(0).Scope().Attributes().Get(a)
			if ok {
				rKey.WriteString(buildAttributeRoutingKeyValue(a, iAttr))
				continue
			}

			// the lowest level span (or what engineers regularly interact with)
			spans := ils.At(0).Spans()

			if a == pseudoAttrSpanKind {
				rKey.WriteString(buildAttributeRoutingKeyStrValue(a, spans.At(0).Kind().String()))
				continue
			}

			if a == pseudoAttrSpanName {
				rKey.WriteString(buildAttributeRoutingKeyStrValue(a, spans.At(0).Name()))
				continue
			}

			sAttr, ok := spans.At(0).Attributes().Get(a)
			if ok {
				rKey.WriteString(buildAttributeRoutingKeyValue(a, sAttr))
				continue
			}

			rKey.WriteString(buildAttributeRoutingKey(a))
		}

		// No matter what, there will be a key here (even if that key is "").
		ids[rKey.String()] = true
	}

	return ids, nil
}

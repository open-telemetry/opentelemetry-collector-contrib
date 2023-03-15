// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var _ exporter.Traces = (*traceExporterImp)(nil)

type traceExporterImp struct {
	loadBalancer loadBalancer
	resourceKeys []string

	traceConsumer traceConsumer
	stopped       bool
	shutdownWg    sync.WaitGroup
	logger        *zap.Logger
}

type routingEntry struct {
	routingKey routingKey
	keyValue   string
	trace      ptrace.Traces
}

type traceConsumer func(ctx context.Context, td ptrace.Traces) error

// Create new traces exporter
func newTracesExporter(params exporter.CreateSettings, cfg component.Config, logger *zap.Logger) (*traceExporterImp, error) {
	exporterFactory := otlpexporter.NewFactory()

	lb, err := newLoadBalancer(params, cfg, func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		return exporterFactory.CreateTracesExporter(ctx, params, &oCfg)
	})
	if err != nil {
		return nil, err
	}

	traceExporter := traceExporterImp{loadBalancer: lb, logger: logger}

	switch cfg.(*Config).RoutingKey {
	case "service":
		traceExporter.traceConsumer = traceExporter.consumeTracesByResource
		traceExporter.resourceKeys = []string{"service.name"}
	case "resource":
		traceExporter.traceConsumer = traceExporter.consumeTracesByResource
		traceExporter.resourceKeys = cfg.(*Config).ResourceKeys
	case "traceID", "":
		traceExporter.traceConsumer = traceExporter.consumeTracesById
	default:
		return nil, fmt.Errorf("unsupported routing_key: %s", cfg.(*Config).RoutingKey)
	}
	return &traceExporter, nil
}

func buildExporterConfig(cfg *Config, endpoint string) otlpexporter.Config {
	oCfg := cfg.Protocol.OTLP
	oCfg.Endpoint = endpoint
	return oCfg
}

func (e *traceExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *traceExporterImp) Start(ctx context.Context, host component.Host) error {
	return e.loadBalancer.Start(ctx, host)
}

func (e *traceExporterImp) Shutdown(context.Context) error {
	e.stopped = true
	e.shutdownWg.Wait()
	return nil
}

func (e *traceExporterImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return e.traceConsumer(ctx, td)
}

func (e *traceExporterImp) consumeTracesById(ctx context.Context, td ptrace.Traces) error {
	var errs error
	batches := batchpersignal.SplitTraces(td)

	for _, t := range batches {
		if tid, err := routeByTraceId(t); err == nil {
			errs = multierr.Append(errs, e.consumeTrace(ctx, t, tid))
		} else {
			return err
		}
	}
	return errs
}

func (e *traceExporterImp) consumeTracesByResource(ctx context.Context, td ptrace.Traces) error {
	var errs error
	routeBatches, err := splitTracesByResourceAttr(td, e.resourceKeys)
	if err != nil {
		return err
	}
	for _, batch := range routeBatches {
		switch batch.routingKey {
		case resourceAttrRouting:
			errs = multierr.Append(errs, e.consumeTrace(ctx, batch.trace, batch.keyValue))
		case traceIDRouting:
			errs = multierr.Append(errs, e.consumeTracesById(ctx, batch.trace))
		}
	}
	return errs

}

func (e *traceExporterImp) consumeTrace(ctx context.Context, td ptrace.Traces, rid string) error {
	// Routes a single trace via a given routing ID
	endpoint := e.loadBalancer.Endpoint([]byte(rid))
	exp, err := e.loadBalancer.Exporter(endpoint)
	if err != nil {
		return err
	}

	te, ok := exp.(exporter.Traces)
	if !ok {
		return fmt.Errorf("unable to export traces, unexpected exporter type: expected exporter.Traces but got %T", exp)
	}

	start := time.Now()
	err = te.ConsumeTraces(ctx, td)
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
	return err
}

func getResourceAttrValue(rs ptrace.ResourceSpans, resourceKeys []string) (string, bool) {
	for _, attrKey := range resourceKeys {
		if attributeValue, ok := rs.Resource().Attributes().Get(attrKey); ok {
			return attributeValue.Str(), true
		}
	}
	return "", false
}

func splitTracesByResourceAttr(batches ptrace.Traces, resourceKeys []string) ([]routingEntry, error) {
	// This function batches all the ResourceSpans with the same routing resource attribute value into a single ptrace.Trace
	// This returns a list of routing entries which consists of the routing key, routing key value and the trace
	// There should be a 1:1 mapping between key value <-> trace
	// This is because we group all Resource Spans with the same key value under a single trace
	result := []routingEntry{}
	rss := batches.ResourceSpans()

	// This is a mapping between the resource attribute values found and the constructed trace
	routeMap := make(map[string]ptrace.Traces)

	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		if keyValue, ok := getResourceAttrValue(rs, resourceKeys); ok {
			// Check if this keyValue has previously been seen
			// if not it constructs an empty ptrace.Trace
			if _, ok := routeMap[keyValue]; !ok {
				routeMap[keyValue] = ptrace.NewTraces()
			}
			rs.CopyTo(routeMap[keyValue].ResourceSpans().AppendEmpty())
		} else {
			// If none of the resource attributes have been found
			// We fallback to routing the given Resource Span by Trace ID
			t := ptrace.NewTraces()
			rs.CopyTo(t.ResourceSpans().AppendEmpty())
			// We can't route this whole Resource Span by a single trace ID
			// because it's possible for the spans under the RS to have different trace IDs
			result = append(result, routingEntry{
				routingKey: traceIDRouting,
				trace:      t,
			})
		}
	}

	// We convert the attr value:trace mapping into a list of routingEntries
	for key, trace := range routeMap {
		result = append(result, routingEntry{
			routingKey: resourceAttrRouting,
			keyValue:   key,
			trace:      trace,
		})
	}

	return result, nil
}

func routeByTraceId(td ptrace.Traces) (string, error) {
	// This function assumes that you are receiving a single trace i.e. single traceId
	// returns the traceId as the routing key
	rs := td.ResourceSpans()
	if rs.Len() == 0 {
		return "", errors.New("empty resource spans")
	}
	if rs.Len() > 1 {
		return "", errors.New("routeByTraceId must receive a ptrace.Traces with a single ResourceSpan")
	}
	ils := rs.At(0).ScopeSpans()
	if ils.Len() == 0 {
		return "", errors.New("empty scope spans")
	}
	spans := ils.At(0).Spans()
	if spans.Len() == 0 {
		return "", errors.New("empty spans")
	}
	tid := spans.At(0).TraceID()
	return string(tid[:]), nil
}

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
)

var _ exporter.Traces = (*traceExporterImp)(nil)

type traceExporterImp struct {
	loadBalancer    loadBalancer
	routingKey      routingKey
	resourceAttrKey string

	stopped    bool
	shutdownWg sync.WaitGroup
}

// Create new traces exporter
func newTracesExporter(params exporter.CreateSettings, cfg component.Config) (*traceExporterImp, error) {
	exporterFactory := otlpexporter.NewFactory()

	lb, err := newLoadBalancer(params, cfg, func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		return exporterFactory.CreateTracesExporter(ctx, params, &oCfg)
	})
	if err != nil {
		return nil, err
	}

	traceExporter := traceExporterImp{loadBalancer: lb, routingKey: traceIDRouting}

	switch cfg.(*Config).RoutingKey {
	case "service":
		traceExporter.routingKey = svcRouting
	case "resourceAttr":
		traceExporter.routingKey = resourceAttrRouting
		traceExporter.resourceAttrKey = cfg.(*Config).ResourceAttrKey
	case "traceID", "":
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

func SplitTracesByResourceAttr(batch ptrace.Traces, attrKey string) []ptrace.Traces {
	var result []ptrace.Traces
	var strKey string

	for i := 0; i < batch.ResourceSpans().Len(); i++ {
		rs := batch.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			// the batches for this ILS
			batches := map[string]ptrace.ResourceSpans{}

			key, ok := rs.Resource().Attributes().Get(attrKey)

			ils := rs.ScopeSpans().At(j)
			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)

				if !ok {
					strKey = span.TraceID().String()
				} else {
					strKey = key.Str()
				}

				// for the first traceID in the ILS, initialize the map entry
				// and add the singleTraceBatch to the result list
				if _, ok := batches[strKey]; !ok {
					trace := ptrace.NewTraces()
					newRS := trace.ResourceSpans().AppendEmpty()
					// currently, the ResourceSpans implementation has only a Resource and an ILS. We'll copy the Resource
					// and set our own ILS
					rs.Resource().CopyTo(newRS.Resource())
					newRS.SetSchemaUrl(rs.SchemaUrl())
					newILS := newRS.ScopeSpans().AppendEmpty()
					// currently, the ILS implementation has only an InstrumentationLibrary and spans. We'll copy the library
					// and set our own spans
					ils.Scope().CopyTo(newILS.Scope())
					newILS.SetSchemaUrl(ils.SchemaUrl())
					batches[strKey] = newRS

					result = append(result, trace)
				}

				// there is only one instrumentation library per batch
				tgt := batches[strKey].ScopeSpans().At(0).Spans().AppendEmpty()
				span.CopyTo(tgt)
			}
		}
	}

	return result
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
	var errs error
	var batches []ptrace.Traces
	if e.routingKey == resourceAttrRouting {
		batches = SplitTracesByResourceAttr(td, e.resourceAttrKey)
	} else {
		batches = batchpersignal.SplitTraces(td)
	}
	for _, batch := range batches {
		errs = multierr.Append(errs, e.consumeTrace(ctx, batch))
	}

	return errs
}

func (e *traceExporterImp) consumeTrace(ctx context.Context, td ptrace.Traces) error {
	var exp component.Component
	var attrKey string
	if e.routingKey == svcRouting || e.routingKey == traceIDRouting {
		attrKey = ""
	} else {
		attrKey = e.resourceAttrKey
	}
	routingIds, err := routingIdentifiersFromTraces(td, e.routingKey, attrKey)
	if err != nil {
		return err
	}
	var rid string
	for key := range routingIds {
		rid = key
		break
	}
	// for rid := range routingIds {
	endpoint := e.loadBalancer.Endpoint([]byte(rid))
	exp, err = e.loadBalancer.Exporter(endpoint)
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
	// }
	return err
}

func routingIdentifiersFromTraces(td ptrace.Traces, routing routingKey, routeKey string) (map[string][]int, error) {
	ids := make(map[string][]int)
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

	if routing == svcRouting || routing == resourceAttrRouting {
		var attrKey string
		if routing == svcRouting {
			attrKey = "service.name"
		} else {
			attrKey = routeKey
		}
		for i := 0; i < rs.Len(); i++ {
			attr, ok := rs.At(i).Resource().Attributes().Get(attrKey)
			if !ok {
				return nil, errors.New("unable to get attribute")
			}
			_, exists := ids[attr.Str()]
			if exists {
				ids[attr.Str()] = []int{i}
			} else {
				ids[attr.Str()] = append(ids[attr.Str()], i)
			}
		}
		return ids, nil
	}
	tid := spans.At(0).TraceID()
	ids[string(tid[:])] = []int{}
	for i := 0; i < rs.Len(); i++ {
		ids[string(tid[:])] = append(ids[string(tid[:])], i)
	}
	return ids, nil
}

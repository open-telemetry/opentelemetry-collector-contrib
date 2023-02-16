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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"
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

type baseTracesExporter struct {
	component.Component
	consumer.Traces
	routingKey routingKey
}

type traceExporterImp struct {
	loadBalancer    loadBalancer
	routingKey      routingKey
	resourceAttrKey string

	stopped    bool
	shutdownWg sync.WaitGroup
}

type LoadBalancerGenerator func(params exporter.CreateSettings, cfg component.Config, factory componentFactory) (*loadBalancerImp, error)

// Create new traces exporter
func newTracesExporter(params exporter.CreateSettings, cfg component.Config, lbf LoadBalancerGenerator) (exporter.Traces, error) {
	exporterFactory := otlpexporter.NewFactory()

	lb, err := lbf(params, cfg, func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		return exporterFactory.CreateTracesExporter(ctx, params, &oCfg)
	})
	if err != nil {
		return nil, err
	}

	traceExporter := traceExporterImp{loadBalancer: lb, routingKey: traceIDRouting}

	switch cfg.(*Config).RoutingKey {
	case "service":
		traceExporter.routingKey = resourceAttrRouting
		traceExporter.resourceAttrKey = "service.name"
		return &baseTracesExporter{
			Component:  &traceExporter,
			Traces:     batchperresourceattr.NewBatchPerResourceTraces(traceExporter.resourceAttrKey, &traceExporter),
			routingKey: svcRouting,
		}, nil
	case "resourceAttr":
		traceExporter.routingKey = resourceAttrRouting
		traceExporter.resourceAttrKey = cfg.(*Config).ResourceAttrKey
		return &baseTracesExporter{
			Component:  &traceExporter,
			Traces:     batchperresourceattr.NewBatchPerResourceTraces(traceExporter.resourceAttrKey, &traceExporter),
			routingKey: resourceAttrRouting,
		}, nil
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
	batches := batchpersignal.SplitTraces(td)
	for _, batch := range batches {
		errs = multierr.Append(errs, e.consumeTrace(ctx, batch))
	}

	return errs
}

func (e *traceExporterImp) consumeTrace(ctx context.Context, td ptrace.Traces) error {
	var exp component.Component
	var attrKey string
	if e.routingKey == svcRouting || e.routingKey == resourceAttrRouting {
		attrKey = e.resourceAttrKey
	} else {
		attrKey = ""
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
	return err
}

// This function should receive a Trace with a single unique value for the routingKey
func routingIdentifiersFromTraces(td ptrace.Traces, routing routingKey, routeKey string) (map[string][]int, error) {
	keys := make(map[string][]int)
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
		for i := 0; i < rs.Len(); i++ {
			attr, ok := rs.At(i).Resource().Attributes().Get(routeKey)
			if !ok {
				return nil, fmt.Errorf("unable to get routing attribute: %s, %d", routeKey, routing)
			}
			_, exists := keys[attr.Str()]
			if exists {
				keys[attr.Str()] = []int{i}
			} else {
				keys[attr.Str()] = append(keys[attr.Str()], i)
			}
		}
		if len(keys) != 1 {
			return nil, errors.New("batch of traces include multiple values of the routing attribute")
		}
		return keys, nil
	}
	tid := spans.At(0).TraceID().String()
	keys[tid] = []int{}
	for i := 0; i < rs.Len(); i++ {
		keys[tid] = append(keys[tid], i)
	}
	return keys, nil
}

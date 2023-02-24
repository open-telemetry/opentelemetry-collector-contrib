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
	loadBalancer     loadBalancer
	routingKey       routingKey
	resourceAttrKeys []string

	stopped    bool
	shutdownWg sync.WaitGroup
	logger     *zap.Logger
}

type routingFunction func(ptrace.Traces) (map[string][]int, error)

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

	traceExporter := traceExporterImp{loadBalancer: lb, routingKey: traceIDRouting, logger: logger}

	switch cfg.(*Config).RoutingKey {
	case "service":
		traceExporter.routingKey = svcRouting
	case "resourceAttr":
		traceExporter.routingKey = resourceAttrRouting
		traceExporter.resourceAttrKeys = cfg.(*Config).ResourceAttrKeys
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

func SplitTracesByResourceAttr(batches ptrace.Traces, attrKeys []string) (map[string][]ptrace.Traces, error) {
	// This code is based on the ConsumeTraces function of the batchperresourceattr
	// modified to support multiple routing keys + fallback on the traceId when routing attr does not exist
	result := make(map[string][]ptrace.Traces)

	rss := batches.ResourceSpans()
	lenRss := rss.Len()

	indicesByAttr := make(map[string]map[string][]int)
	var fallbackIndices []int
	var attrFound bool
	for i := 0; i < lenRss; i++ {
		rs := rss.At(i)
		attrFound = false
		for _, attrKey := range attrKeys {
			var keyValue string
			if _, ok := indicesByAttr[attrKey]; !ok {
				indicesByAttr[attrKey] = make(map[string][]int)
			}
			if attributeValue, ok := rs.Resource().Attributes().Get(attrKey); ok {
				keyValue = attributeValue.Str()
				indicesByAttr[attrKey][keyValue] = append(indicesByAttr[attrKey][keyValue], i)
				attrFound = true
				break
			}
		}
		if !attrFound {
			// These will be processed further to be split per traceID
			fallbackIndices = append(fallbackIndices, i)
		}
	}

	for j := 0; j < len(fallbackIndices); j++ {
		t := ptrace.NewTraces()
		rs := rss.At(fallbackIndices[j])
		rs.CopyTo(t.ResourceSpans().AppendEmpty())
		nt := batchpersignal.SplitTraces(t)
		result["traceId"] = append(result["traceId"], nt...)
	}

	for routeKey, routeKeyAttrs := range indicesByAttr {
		for _, indices := range routeKeyAttrs {
			tracesForAttr := ptrace.NewTraces()
			for _, i := range indices {
				rs := rss.At(i)
				rs.CopyTo((tracesForAttr.ResourceSpans().AppendEmpty()))
			}
			result[routeKey] = append(result[routeKey], tracesForAttr)
		}
	}

	return result, nil
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
	var err error
	var errs error
	var batches = make(map[string][]ptrace.Traces)
	if e.routingKey == resourceAttrRouting {
		batches, err = SplitTracesByResourceAttr(td, e.resourceAttrKeys)
		if err != nil {
			return err
		}
	} else {
		batches["traceId"] = batchpersignal.SplitTraces(td)
	}
	rfs := make(map[string]routingFunction)
	for key := range batches {
		if key == "traceId" {
			rfs[key] = func(x ptrace.Traces) (map[string][]int, error) {
				return routeByTraceId(x)
			}
		} else {
			rfs[key] = func(x ptrace.Traces) (map[string][]int, error) {
				return routeByResourceAttr(x, key)
			}
		}
	}
	for key, tb := range batches {
		for _, t := range tb {
			errs = multierr.Append(errs, e.consumeTrace(ctx, t, rfs[key]))
		}
	}

	return errs
}

func (e *traceExporterImp) consumeTrace(ctx context.Context, td ptrace.Traces, rf routingFunction) error {
	var exp component.Component
	routingIds, err := rf(td)
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

func validateNotEmpty(td ptrace.Traces) error {
	rs := td.ResourceSpans()
	if rs.Len() == 0 {
		return errors.New("empty resource spans")
	}
	ils := rs.At(0).ScopeSpans()
	if ils.Len() == 0 {
		return errors.New("empty scope spans")
	}
	spans := ils.At(0).Spans()
	if spans.Len() == 0 {
		return errors.New("empty spans")
	}
	return nil
}

func routeByTraceId(td ptrace.Traces) (map[string][]int, error) {
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
	tid := spans.At(0).TraceID()
	ids[string(tid[:])] = []int{}
	for i := 0; i < rs.Len(); i++ {
		ids[string(tid[:])] = append(ids[string(tid[:])], i)
	}
	return ids, nil
}

func routeByResourceAttr(td ptrace.Traces, routeKey string) (map[string][]int, error) {
	ids := make(map[string][]int)
	err := validateNotEmpty(td)
	if err != nil {
		return nil, err
	}

	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		attr, ok := rs.At(i).Resource().Attributes().Get(routeKey)
		if !ok {
			// If resource attribute is not found, falls back to  Trace ID routing
			return routeByTraceId(td)
		}
		ids[attr.Str()] = append(ids[attr.Str()], i)
	}
	if len(ids) > 1 {
		return nil, errors.New("received traces were not split by resource attr")
	}
	return ids, nil
}

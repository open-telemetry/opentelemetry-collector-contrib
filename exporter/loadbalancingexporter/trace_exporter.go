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

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

var _ exporter.Traces = (*traceExporterImp)(nil)

type traceExporterImp struct {
	loadBalancer loadBalancer
	routingKey   routingKey

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

// If we are sending to only one endpoint, returns that endpoint.
// Otherwise returns empty string.
func (e *traceExporterImp) HasMultipleEndpoints(ctx context.Context, td ptrace.Traces, batches []ptrace.Traces) (string, error) {
	var errs error

	// First check if we're sending to multiple exporters.
	endpoint := ""
	hasMultipleEndpoints := false

	for batch := range batches {
		routingIDs, err := routingIdentifiersFromTraces(batches[batch], e.routingKey)
		errs = multierr.Append(errs, err)
		if len(routingIDs) > 2 {
			hasMultipleEndpoints = true
			break
		} else {
			for rid := range routingIDs {
				if endpoint != "" && endpoint != rid {
					hasMultipleEndpoints = true
					break
				} else {
					endpoint = rid
				}
			}
		}
	}

	if hasMultipleEndpoints {
		return "", errs
	}
	return endpoint, errs
}

func (e *traceExporterImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errs error
	batches := batchpersignal.SplitTraces(td)
	end, err := e.HasMultipleEndpoints(ctx, td, batches)
	errs = multierr.Append(errs, err)

	if end != "" {
		// We don't need to batch; we only send to one backend.
		endpoint := e.loadBalancer.Endpoint([]byte(end))
		errs = multierr.Append(errs, e.consumeTrace(ctx, td, endpoint))
		return errs
	}

	// Map the trace data to their respective endpoints.
	endpointToTraceData := make(map[string]ptrace.Traces)
	for batch := range batches {
		routingIDs, err := routingIdentifiersFromTraces(batches[batch], e.routingKey)
		errs = multierr.Append(errs, err)
		for rid := range routingIDs {
			endpoint := e.loadBalancer.Endpoint([]byte(rid))
			if _, ok := endpointToTraceData[endpoint]; ok {
				batches[batch].ResourceSpans().MoveAndAppendTo(endpointToTraceData[endpoint].ResourceSpans())
			} else {
				newTrace := ptrace.NewTraces()
				batches[batch].ResourceSpans().MoveAndAppendTo(newTrace.ResourceSpans())
				endpointToTraceData[endpoint] = newTrace
			}
		}
	}

	// Send the trace data off by endpoint.
	for endpoint, traces := range endpointToTraceData {
		errs = multierr.Append(errs, e.consumeTrace(ctx, traces, endpoint))
	}
	return errs
}

func (e *traceExporterImp) consumeTrace(ctx context.Context, td ptrace.Traces, endpoint string) error {
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

func routingIdentifiersFromTraces(td ptrace.Traces, key routingKey) (map[string]bool, error) {
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

	if key == svcRouting {
		for i := 0; i < rs.Len(); i++ {
			svc, ok := rs.At(i).Resource().Attributes().Get("service.name")
			if !ok {
				return nil, errors.New("unable to get service name")
			}
			ids[svc.Str()] = true
		}
		return ids, nil
	}
	tid := spans.At(0).TraceID()
	ids[string(tid[:])] = true
	return ids, nil
}

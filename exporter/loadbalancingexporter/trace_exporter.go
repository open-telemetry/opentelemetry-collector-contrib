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

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

var _ component.TracesExporter = (*traceExporterImp)(nil)

var (
	errNoTracesInBatch = errors.New("no traces were found in the batch")
)

type traceExporterImp struct {
	loadBalancer loadBalancer

	stopped    bool
	shutdownWg sync.WaitGroup
}

// Create new traces exporter
func newTracesExporter(params component.ExporterCreateSettings, cfg config.Exporter) (*traceExporterImp, error) {
	exporterFactory := otlpexporter.NewFactory()

	lb, err := newLoadBalancer(params, cfg, func(ctx context.Context, endpoint string) (component.Exporter, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		return exporterFactory.CreateTracesExporter(ctx, params, &oCfg)
	})
	if err != nil {
		return nil, err
	}

	return &traceExporterImp{
		loadBalancer: lb,
	}, nil
}

func buildExporterConfig(cfg *Config, endpoint string) otlpexporter.Config {
	oCfg := cfg.Protocol.OTLP
	oCfg.ExporterSettings = config.NewExporterSettings(config.NewID("otlp"))
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

func (e *traceExporterImp) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	var errors []error
	batches := batchpersignal.SplitTraces(td)
	for _, batch := range batches {
		if err := e.consumeTrace(ctx, batch); err != nil {
			errors = append(errors, err)
		}
	}

	return consumererror.Combine(errors)
}

func (e *traceExporterImp) consumeTrace(ctx context.Context, td pdata.Traces) error {
	traceID := traceIDFromTraces(td)
	if traceID == pdata.InvalidTraceID() {
		return errNoTracesInBatch
	}

	endpoint := e.loadBalancer.Endpoint(traceID)
	exp, err := e.loadBalancer.Exporter(endpoint)
	if err != nil {
		return err
	}

	te, ok := exp.(component.TracesExporter)
	if !ok {
		expectType := (*component.TracesExporter)(nil)
		return fmt.Errorf("expected %T but got %T", expectType, exp)
	}

	start := time.Now()
	err = te.ConsumeTraces(ctx, td)
	duration := time.Since(start)
	ctx, _ = tag.New(ctx, tag.Upsert(tag.MustNewKey("endpoint"), endpoint))

	if err == nil {
		sCtx, _ := tag.New(ctx, tag.Upsert(tag.MustNewKey("success"), "true"))
		stats.Record(sCtx, mBackendLatency.M(duration.Milliseconds()))
	} else {
		fCtx, _ := tag.New(ctx, tag.Upsert(tag.MustNewKey("success"), "false"))
		stats.Record(fCtx, mBackendLatency.M(duration.Milliseconds()))
	}

	return err
}

func traceIDFromTraces(td pdata.Traces) pdata.TraceID {
	rs := td.ResourceSpans()
	if rs.Len() == 0 {
		return pdata.InvalidTraceID()
	}

	ils := rs.At(0).InstrumentationLibrarySpans()
	if ils.Len() == 0 {
		return pdata.InvalidTraceID()
	}

	spans := ils.At(0).Spans()
	if spans.Len() == 0 {
		return pdata.InvalidTraceID()
	}

	return spans.At(0).TraceID()
}

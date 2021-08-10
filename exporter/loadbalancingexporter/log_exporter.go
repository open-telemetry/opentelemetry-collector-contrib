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
	"fmt"
	"math/rand"
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

var _ component.LogsExporter = (*logExporterImp)(nil)

type logExporterImp struct {
	loadBalancer loadBalancer

	stopped    bool
	shutdownWg sync.WaitGroup
}

// Create new logs exporter
func newLogsExporter(params component.ExporterCreateSettings, cfg config.Exporter) (*logExporterImp, error) {
	exporterFactory := otlpexporter.NewFactory()

	lb, err := newLoadBalancer(params, cfg, func(ctx context.Context, endpoint string) (component.Exporter, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		return exporterFactory.CreateLogsExporter(ctx, params, &oCfg)
	})
	if err != nil {
		return nil, err
	}

	return &logExporterImp{
		loadBalancer: lb,
	}, nil
}

func (e *logExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *logExporterImp) Start(ctx context.Context, host component.Host) error {
	return e.loadBalancer.Start(ctx, host)
}

func (e *logExporterImp) Shutdown(context.Context) error {
	e.stopped = true
	e.shutdownWg.Wait()
	return nil
}

func (e *logExporterImp) ConsumeLogs(ctx context.Context, ld pdata.Logs) error {
	var errors []error
	batches := batchpersignal.SplitLogs(ld)
	for _, batch := range batches {
		if err := e.consumeLog(ctx, batch); err != nil {
			errors = append(errors, err)
		}
	}

	return consumererror.Combine(errors)
}

func (e *logExporterImp) consumeLog(ctx context.Context, ld pdata.Logs) error {
	traceID := traceIDFromLogs(ld)
	balancingKey := traceID
	if traceID == pdata.InvalidTraceID() {
		// every log may not contain a traceID
		// generate a random traceID as balancingKey
		// so the log can be routed to a random backend
		balancingKey = random()
	}

	endpoint := e.loadBalancer.Endpoint(balancingKey)
	exp, err := e.loadBalancer.Exporter(endpoint)
	if err != nil {
		return err
	}

	le, ok := exp.(component.LogsExporter)
	if !ok {
		expectType := (*component.LogsExporter)(nil)
		return fmt.Errorf("unable to export logs, unexpected exporter type: expected %T but got %T", expectType, exp)
	}

	start := time.Now()
	err = le.ConsumeLogs(ctx, ld)
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

func traceIDFromLogs(ld pdata.Logs) pdata.TraceID {
	rl := ld.ResourceLogs()
	if rl.Len() == 0 {
		return pdata.InvalidTraceID()
	}

	ill := rl.At(0).InstrumentationLibraryLogs()
	if ill.Len() == 0 {
		return pdata.InvalidTraceID()
	}

	logs := ill.At(0).Logs()
	if logs.Len() == 0 {
		return pdata.InvalidTraceID()
	}

	return logs.At(0).TraceID()
}

func random() pdata.TraceID {
	v1 := uint8(rand.Intn(256))
	v2 := uint8(rand.Intn(256))
	v3 := uint8(rand.Intn(256))
	v4 := uint8(rand.Intn(256))
	return pdata.NewTraceID([16]byte{v1, v2, v3, v4})
}

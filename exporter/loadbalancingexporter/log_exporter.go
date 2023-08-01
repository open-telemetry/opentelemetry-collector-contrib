// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

var _ exporter.Logs = (*logExporterImp)(nil)

type logExporterImp struct {
	loadBalancer loadBalancer

	started    bool
	shutdownWg sync.WaitGroup
}

// Create new logs exporter
func newLogsExporter(params exporter.CreateSettings, cfg component.Config) (*logExporterImp, error) {
	exporterFactory := otlpexporter.NewFactory()

	lb, err := newLoadBalancer(params, cfg, func(ctx context.Context, endpoint string) (component.Component, error) {
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
	e.started = true
	return e.loadBalancer.Start(ctx, host)
}

func (e *logExporterImp) Shutdown(context.Context) error {
	if !e.started {
		return nil
	}
	e.started = false
	e.shutdownWg.Wait()
	return nil
}

func (e *logExporterImp) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var errs error
	batches := batchpersignal.SplitLogs(ld)
	for _, batch := range batches {
		errs = multierr.Append(errs, e.consumeLog(ctx, batch))
	}

	return errs
}

func (e *logExporterImp) consumeLog(ctx context.Context, ld plog.Logs) error {
	traceID := traceIDFromLogs(ld)
	balancingKey := traceID
	if traceID == pcommon.NewTraceIDEmpty() {
		// every log may not contain a traceID
		// generate a random traceID as balancingKey
		// so the log can be routed to a random backend
		balancingKey = random()
	}

	endpoint := e.loadBalancer.Endpoint(balancingKey[:])
	exp, err := e.loadBalancer.Exporter(endpoint)
	if err != nil {
		return err
	}

	le, ok := exp.(exporter.Logs)
	if !ok {
		return fmt.Errorf("unable to export logs, unexpected exporter type: expected exporter.Logs but got %T", exp)
	}

	start := time.Now()
	err = le.ConsumeLogs(ctx, ld)
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

func traceIDFromLogs(ld plog.Logs) pcommon.TraceID {
	rl := ld.ResourceLogs()
	if rl.Len() == 0 {
		return pcommon.NewTraceIDEmpty()
	}

	sl := rl.At(0).ScopeLogs()
	if sl.Len() == 0 {
		return pcommon.NewTraceIDEmpty()
	}

	logs := sl.At(0).LogRecords()
	if logs.Len() == 0 {
		return pcommon.NewTraceIDEmpty()
	}

	return logs.At(0).TraceID()
}

func random() pcommon.TraceID {
	v1 := uint8(rand.Intn(256))
	v2 := uint8(rand.Intn(256))
	v3 := uint8(rand.Intn(256))
	v4 := uint8(rand.Intn(256))
	return [16]byte{v1, v2, v3, v4}
}

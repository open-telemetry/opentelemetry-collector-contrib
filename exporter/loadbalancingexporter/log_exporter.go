// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

var _ exporter.Logs = (*logExporterImp)(nil)

type logExporterImp struct {
	loadBalancer *loadBalancer

	started    bool
	shutdownWg sync.WaitGroup
	telemetry  *metadata.TelemetryBuilder
}

// Create new logs exporter
func newLogsExporter(params exporter.Settings, cfg component.Config) (*logExporterImp, error) {
	telemetry, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	exporterFactory := otlpexporter.NewFactory()
	cfFunc := func(ctx context.Context, endpoint string) (component.Component, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		oParams := buildExporterSettings(params, endpoint)

		return exporterFactory.CreateLogs(ctx, oParams, &oCfg)
	}

	lb, err := newLoadBalancer(params.Logger, cfg, cfFunc, telemetry)
	if err != nil {
		return nil, err
	}

	return &logExporterImp{
		loadBalancer: lb,
		telemetry:    telemetry,
	}, nil
}

func (e *logExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *logExporterImp) Start(ctx context.Context, host component.Host) error {
	e.started = true
	return e.loadBalancer.Start(ctx, host)
}

func (e *logExporterImp) Shutdown(ctx context.Context) error {
	if !e.started {
		return nil
	}
	err := e.loadBalancer.Shutdown(ctx)
	e.started = false
	e.shutdownWg.Wait()
	return err
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

	le, _, err := e.loadBalancer.exporterAndEndpoint(balancingKey[:])
	if err != nil {
		return err
	}

	le.consumeWG.Add(1)
	defer le.consumeWG.Done()

	start := time.Now()
	err = le.ConsumeLogs(ctx, ld)
	duration := time.Since(start)
	e.telemetry.LoadbalancerBackendLatency.Record(ctx, duration.Milliseconds(), metric.WithAttributeSet(le.endpointAttr))
	if err == nil {
		e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(le.successAttr))
	} else {
		e.telemetry.LoadbalancerBackendOutcome.Add(ctx, 1, metric.WithAttributeSet(le.failureAttr))
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

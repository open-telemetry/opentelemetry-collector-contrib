// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
)

var errExporterIsStopping = errors.New("exporter is stopping")

// wrappedExporter is an exporter that waits for the data processing to complete before shutting down.
// consumeWG has to be incremented explicitly by the consumer of the wrapped exporter.
type wrappedExporter struct {
	component.Component
	consumeWG sync.WaitGroup
	consumeMu sync.Mutex
	stopping  atomic.Bool
	endpoint  string

	// we store the attributes here for both cases, to avoid new allocations on the hot path
	endpointAttr      attribute.Set
	successAttr       attribute.Set
	failureAttr       attribute.Set
	logRequestAttr    attribute.Set
	metricRequestAttr attribute.Set
}

func newWrappedExporter(exp component.Component, identifier string) *wrappedExporter {
	endpoint := endpointWithPort(identifier)
	ea := attribute.String("endpoint", endpoint)
	return &wrappedExporter{
		Component:         exp,
		endpoint:          endpoint,
		endpointAttr:      attribute.NewSet(ea),
		successAttr:       attribute.NewSet(ea, attribute.Bool("success", true)),
		failureAttr:       attribute.NewSet(ea, attribute.Bool("success", false)),
		logRequestAttr:    backendRequestAttributeSet(backendRequestSignalLogs, endpoint),
		metricRequestAttr: backendRequestAttributeSet(backendRequestSignalMetrics, endpoint),
	}
}

func (we *wrappedExporter) Shutdown(ctx context.Context) error {
	we.consumeMu.Lock()
	we.stopping.Store(true)
	we.consumeMu.Unlock()
	we.consumeWG.Wait()
	return we.Component.Shutdown(ctx)
}

func (we *wrappedExporter) markStopping() {
	we.consumeMu.Lock()
	we.stopping.Store(true)
	we.consumeMu.Unlock()
}

func (we *wrappedExporter) isStopping() bool {
	return we.stopping.Load()
}

func (we *wrappedExporter) tryStartConsume() bool {
	return we.startConsume(false)
}

func (we *wrappedExporter) forceStartConsume() {
	we.startConsume(true)
}

func (we *wrappedExporter) startConsume(allowStopping bool) bool {
	we.consumeMu.Lock()
	defer we.consumeMu.Unlock()
	if !allowStopping && we.stopping.Load() {
		return false
	}
	we.consumeWG.Add(1)
	return true
}

func (we *wrappedExporter) doneConsume() {
	we.consumeWG.Done()
}

func (we *wrappedExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	te, ok := we.Component.(exporter.Traces)
	if !ok {
		return fmt.Errorf("unable to export traces, unexpected exporter type: expected exporter.Traces but got %T", we.Component)
	}
	return te.ConsumeTraces(ctx, td)
}

func (we *wrappedExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	me, ok := we.Component.(exporter.Metrics)
	if !ok {
		return fmt.Errorf("unable to export metrics, unexpected exporter type: expected exporter.Metrics but got %T", we.Component)
	}
	return me.ConsumeMetrics(ctx, md)
}

func (we *wrappedExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	le, ok := we.Component.(exporter.Logs)
	if !ok {
		return fmt.Errorf("unable to export logs, unexpected exporter type: expected exporter.Logs but got %T", we.Component)
	}
	return le.ConsumeLogs(ctx, ld)
}

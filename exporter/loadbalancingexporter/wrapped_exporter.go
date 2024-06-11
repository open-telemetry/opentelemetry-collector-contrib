// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// wrappedExporter is an exporter that waits for the data processing to complete before shutting down.
// consumeWG has to be incremented explicitly by the consumer of the wrapped exporter.
type wrappedExporter struct {
	component.Component
	consumeWG sync.WaitGroup
}

func newWrappedExporter(exp component.Component) *wrappedExporter {
	return &wrappedExporter{Component: exp}
}

func (we *wrappedExporter) Shutdown(ctx context.Context) error {
	we.consumeWG.Wait()
	return we.Component.Shutdown(ctx)
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

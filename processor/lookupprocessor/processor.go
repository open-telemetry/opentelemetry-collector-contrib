// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup"
)

type lookupProcessor struct {
	source lookup.Source
	logger *zap.Logger
}

func newLookupProcessor(source lookup.Source, logger *zap.Logger) *lookupProcessor {
	return &lookupProcessor{source: source, logger: logger}
}

func (p *lookupProcessor) Start(ctx context.Context, host component.Host) error {
	return p.source.Start(ctx, host)
}

func (p *lookupProcessor) Shutdown(ctx context.Context) error {
	return p.source.Shutdown(ctx)
}

func (*lookupProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	_ = ctx
	return td, nil
}

func (*lookupProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	_ = ctx
	return md, nil
}

func (*lookupProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	_ = ctx
	return ld, nil
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
)

type resourceProcessor struct {
	logger   *zap.Logger
	attrProc *attraction.AttrProc
}

func (rp *resourceProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rp.attrProc.Process(ctx, rp.logger, rss.At(i).Resource().Attributes())
	}
	return td, nil
}

func (rp *resourceProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rp.attrProc.Process(ctx, rp.logger, rms.At(i).Resource().Attributes())
	}
	return md, nil
}

func (rp *resourceProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rp.attrProc.Process(ctx, rp.logger, rls.At(i).Resource().Attributes())
	}
	return ld, nil
}

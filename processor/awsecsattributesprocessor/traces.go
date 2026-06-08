// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type tracesProcessor struct {
	*ecsCore
	next consumer.Traces
}

func newTracesProcessor(logger *zap.Logger, cfg *Config, next consumer.Traces, endpoints endpointsFn) *tracesProcessor {
	return &tracesProcessor{ecsCore: newCore(logger, cfg, endpoints), next: next}
}

func (p *tracesProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := range rss.Len() {
		p.enrichResource(ctx, rss.At(i).Resource())
	}
	return p.next.ConsumeTraces(ctx, td)
}

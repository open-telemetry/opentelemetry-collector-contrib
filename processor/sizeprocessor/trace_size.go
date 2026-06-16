// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sizeprocessor // import "github.com/multiplayer-app/opentelemetry-collector-contrib/processor/sizeprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type spanSizeProcessor struct {
	logger *zap.Logger
}

// newTracesProcessor returns a processor that modifies attributes of a span.
// To construct the attributes processors, the use of the factory methods are required
// in order to validate the inputs.
func newSpanSizeProcessor(logger *zap.Logger) *spanSizeProcessor {
	return &spanSizeProcessor{
		logger: logger,
	}
}

func calculateSpanSize(span ptrace.Span) (int, error) {
	size := sizeOf(span)

	return size, nil
}

func (a *spanSizeProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			// scope := ils.Scope()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				size, err := calculateSpanSize(span)
				if err != nil {
					continue
				}

				span.Attributes().PutInt("span.size", int64(size))
			}
		}
	}
	return td, nil
}

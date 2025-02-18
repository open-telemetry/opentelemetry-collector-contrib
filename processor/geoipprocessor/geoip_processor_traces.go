// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

func (g *geoIPProcessor) processTraces(ctx context.Context, ts ptrace.Traces) (ptrace.Traces, error) {
	rt := ts.ResourceSpans()
	for i := range rt.Len() {
		switch g.cfg.Context {
		case resource:
			err := g.processAttributes(ctx, rt.At(i).Resource().Attributes())
			if err != nil {
				return ts, err
			}
		case record:
			for j := range rt.At(i).ScopeSpans().Len() {
				for k := range rt.At(i).ScopeSpans().At(j).Spans().Len() {
					err := g.processAttributes(ctx, rt.At(i).ScopeSpans().At(j).Spans().At(k).Attributes())
					if err != nil {
						return ts, err
					}
				}
			}
		default:
			return ts, errUnspecifiedSource
		}
	}
	return ts, nil
}

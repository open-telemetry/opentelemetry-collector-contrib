package geoipprocessor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

func (g *geoIPProcessor) processTraces(ctx context.Context, ts ptrace.Traces) (ptrace.Traces, error) {
	rt := ts.ResourceSpans()
	for i := 0; i < rt.Len(); i++ {
		switch g.sourceConfig.From {
		case ResourceSource:
			err := g.processAttributes(ctx, rt.At(i).Resource().Attributes())
			if err != nil {
				return ts, err
			}
		case AttributeSource:
			for j := 0; j < rt.At(i).ScopeSpans().Len(); j++ {
				for k := 0; k < rt.At(i).ScopeSpans().At(j).Spans().Len(); k++ {
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

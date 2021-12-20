package spanprocessor

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterspan"
	"go.opentelemetry.io/collector/model/pdata"
)

type tcsSpanProcessor struct {
	p *spanProcessor
}

func newTCSSpanProcessor(p *spanProcessor) *tcsSpanProcessor {
	return &tcsSpanProcessor{p: p}
}

func (sp *tcsSpanProcessor) processTraces(
	_ context.Context, td pdata.Traces) (pdata.Traces, error) {

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.InstrumentationLibrarySpans()
		resource := rs.Resource()

		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			library := ils.InstrumentationLibrary()

			for k := 0; k < spans.Len(); k++ {
				s := spans.At(k)
				if filterspan.SkipSpanWithoutAttrs(
					tcsAttrs, sp.p.include, sp.p.exclude, s, resource, library) {

					continue
				}

				sp.p.processFromAttributes(s)
				sp.p.processToAttributes(s)
			}
		}
	}

	return td, nil
}

var tcsAttrs = []string{"tenant", "service"}

package schemaprocessor

import (
	"context"

	"go.opentelemetry.io/collector/model/pdata"
)

type schemaProcessor struct {
	factory *factory
}

func newSchemaProcessor(factory *factory, _ *Config) *schemaProcessor {
	return &schemaProcessor{factory: factory}
}

func (a *schemaProcessor) processTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resource := rs.Resource()
		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			library := ils.InstrumentationLibrary()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				_ = resource
				_ = library
				_ = span
			}
		}
	}
	return td, nil
}

func (a *schemaProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	rss := md.ResourceMetrics()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resource := rs.Resource()
		ilms := rs.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()
			library := ils.InstrumentationLibrary()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)

				_ = resource
				_ = library
				_ = metric
			}
		}
	}
	return md, nil
}

func (a *schemaProcessor) processLogs(_ context.Context, ld pdata.Logs) (pdata.Logs, error) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rs := rls.At(i)
		ills := rs.InstrumentationLibraryLogs()
		resource := rs.Resource()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			logs := ils.LogRecords()
			library := ils.InstrumentationLibrary()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)

				_ = resource
				_ = library
				_ = lr
			}
		}
	}
	return ld, nil
}

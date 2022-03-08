package schemaprocessor

import (
	"context"

	"go.opentelemetry.io/collector/model/pdata"
)

type schemaProcessor struct {
	factory *factory
}

func newSchemaProcessor(ctx context.Context, factory *factory, cfg *Config) *schemaProcessor {
	// Start prefetching schemas specified in the config.
	for _, transform := range cfg.Transform {
		go factory.getSchema(ctx, transform.To)
	}
	return &schemaProcessor{factory: factory}
}

func (a *schemaProcessor) processTraces(ctx context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resource := rs.Resource()

		resourceSchema, err := a.factory.getSchema(ctx, rs.SchemaUrl())
		if err != nil {
			// TODO: what do we do if we can't fetch the schema?
		}
		// TODO: use resourceSchema to transform the Resource.
		// See as an inspiration for example:
		// https://github.com/tigrannajaryan/telemetry-schema/blob/main/schema/converter/converter.go
		_ = resourceSchema

		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			library := ils.InstrumentationLibrary()

			libSchema, err := a.factory.getSchema(ctx, ils.SchemaUrl())
			if err != nil {
				// TODO: what do we do if we can't fetch the schema?
			}
			_ = libSchema

			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				// TODO: use resourceSchema and libSchema to transform the span.

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

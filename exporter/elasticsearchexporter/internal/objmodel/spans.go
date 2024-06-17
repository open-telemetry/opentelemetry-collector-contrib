package objmodel // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"

import "go.opentelemetry.io/collector/pdata/ptrace"

// pSpans processes a ptrace.SpanEventSlice into key value pairs and adds
// them to the Elasticsearch document.
type pSpans struct {
	ptrace.SpanEventSlice
}

// NewSpansProcessorValue is a utility function to create a processor value from
// span processor. If the span is empty then it returns a NilValue.
func NewSpansProcessorValue(s ptrace.SpanEventSlice) Value {
	if s.Len() == 0 {
		return NilValue
	}
	return ProcessorValue(newSpansProcessor(s))
}

// newSpansProcessor creates a new processor for processing ptrace.SpanEventSlice.
func newSpansProcessor(s ptrace.SpanEventSlice) pSpans {
	return pSpans{SpanEventSlice: s}
}

// Len gives the number of entries that will be added to the Elasticsearch document.
func (s pSpans) Len() int {
	var count int
	for i := 0; i < s.SpanEventSlice.Len(); i++ {
		count += 1 /*timestamp*/ + s.At(i).Attributes().Len()
	}
	return count
}

// Process iterates over the spans and adds them to the provided document against a
// given key.
func (s pSpans) Process(doc *Document, key string) {
	for i := 0; i < s.SpanEventSlice.Len(); i++ {
		span := s.At(i)
		fKey := flattenKey(key, span.Name())

		doc.fields = append(doc.fields, NewKV(fKey+".time", TimestampValue(span.Timestamp())))
		// TODO (lahsivjar): Defer this to map processor and deprecate AddAttribute*
		doc.AddAttributes(fKey, span.Attributes())
	}
}

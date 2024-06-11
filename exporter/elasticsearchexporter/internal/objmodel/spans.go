package objmodel

import "go.opentelemetry.io/collector/pdata/ptrace"

// Spans processes a ptrace.SpanEventSlice into key value pairs and adds
// them to the Elasticsearch document.
type Spans struct {
	ptrace.SpanEventSlice
}

// NewSpansProcessor creates a new processor for processing ptrace.SpanEventSlice.
func NewSpansProcessor(s ptrace.SpanEventSlice) Spans {
	return Spans{SpanEventSlice: s}
}

// Len gives the number of entries that will be added to the Elasticsearch document.
func (s Spans) Len() int {
	var count int
	for i := 0; i < s.SpanEventSlice.Len(); i++ {
		count += 1 /*timestamp*/ + s.At(i).Attributes().Len()
	}
	return count
}

// Process iterates over the spans and adds them to the provided document against a
// given key.
func (s Spans) Process(doc *Document, key string) {
	for i := 0; i < s.SpanEventSlice.Len(); i++ {
		span := s.At(i)
		fKey := flattenKey(key, span.Name())

		doc.Add(fKey+".time", TimestampValue(span.Timestamp()))
		// TODO (lahsivjar): Defer this to map processor and deprecate AddAttribute*
		doc.AddAttributes(fKey, span.Attributes())
	}
}

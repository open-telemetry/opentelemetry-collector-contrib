package azureblobexporter

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

type tracesJSONLMarshaler struct{}

func (m *tracesJSONLMarshaler) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	marshaler := &ptrace.JSONMarshaler{}
	var buf bytes.Buffer

	rss := td.ResourceSpans()

	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resource := rs.Resource()

		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			ss := sss.At(j)
			scope := ss.Scope()

			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				tempTd := ptrace.NewTraces()
				tempRs := tempTd.ResourceSpans().AppendEmpty()
				resource.CopyTo(tempRs.Resource())
				tempSs := tempRs.ScopeSpans().AppendEmpty()
				scope.CopyTo(tempSs.Scope())
				span.CopyTo(tempSs.Spans().AppendEmpty())

				jsonBytes, err := marshaler.MarshalTraces(tempTd)
				if err != nil {
					return nil, err
				}

				buf.Write(jsonBytes)
				buf.WriteByte('\n')
			}
		}
	}

	return buf.Bytes(), nil
}

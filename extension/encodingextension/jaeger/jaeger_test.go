package jaeger

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestGetTraceCodec(t *testing.T) {
	tcodec := jaegerProtobufTrace{}
	require.NotNil(t, t)
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("foo")
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	span.SetTraceID(traceID)
	b, err := tcodec.MarshalTraces(td)
	require.NoError(t, err)
	td2, err := tcodec.UnmarshalTraces(b)
	require.NoError(t, err)
	require.Equal(t, 1, td2.SpanCount())
}

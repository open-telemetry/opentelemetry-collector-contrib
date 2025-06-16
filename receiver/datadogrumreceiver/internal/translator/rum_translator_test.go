package translator

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/stretchr/testify/assert"
)

func TestParseW3CTraceContext(t *testing.T) {
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	traceID, spanID, err := parseW3CTraceContext(traceparent)
	assert.NoError(t, err)
	assert.Equal(t, traceID, pcommon.TraceID([16]byte{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36}))
	assert.Equal(t, spanID, pcommon.SpanID([8]byte{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7}))
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

func TestWriterPoolBasic(t *testing.T) {
	size := 1024
	wp := newWriterPool(size)
	span := constructWriterPoolSpan()
	w := wp.borrow()
	assert.NotNil(t, w)
	assert.NotNil(t, w.buffer)
	assert.NotNil(t, w.encoder)
	assert.Equal(t, size, w.buffer.Cap())
	assert.Equal(t, 0, w.buffer.Len())
	resource := pcommon.NewResource()
	segment, _ := MakeSegment(span, resource, nil, false, nil, false)
	require.NoError(t, w.Encode(*segment))
	jsonStr := w.String()
	assert.Equal(t, len(jsonStr), w.buffer.Len())
	wp.release(w)
}

func BenchmarkWithoutPool(b *testing.B) {
	logger := zap.NewNop()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		span := constructWriterPoolSpan()
		b.StartTimer()
		buffer := bytes.NewBuffer(make([]byte, 0, 2048))
		encoder := json.NewEncoder(buffer)
		segment, _ := MakeSegment(span, pcommon.NewResource(), nil, false, nil, false)
		err := encoder.Encode(*segment)
		assert.NoError(b, err)
		logger.Info(buffer.String())
	}
}

func BenchmarkWithPool(b *testing.B) {
	logger := zap.NewNop()
	wp := newWriterPool(2048)
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		span := constructWriterPoolSpan()
		b.StartTimer()
		w := wp.borrow()
		segment, _ := MakeSegment(span, pcommon.NewResource(), nil, false, nil, false)
		err := w.Encode(*segment)
		assert.Nil(b, err)
		logger.Info(w.String())
	}
}

func constructWriterPoolSpan() ptrace.Span {
	attributes := make(map[string]any)
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventions.AttributeHTTPStatusCode] = 200
	return constructHTTPServerSpan(attributes)
}

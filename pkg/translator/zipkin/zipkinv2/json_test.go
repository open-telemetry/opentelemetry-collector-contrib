// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinv2

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestJSONUnmarshaler_UnmarshalTraces(t *testing.T) {
	data, err := os.ReadFile("testdata/zipkin_v2_single.json")
	require.NoError(t, err)
	decoder := NewJSONTracesUnmarshaler(false)
	td, err := decoder.UnmarshalTraces(data)
	assert.NoError(t, err)
	assert.Equal(t, 1, td.SpanCount())
}

func TestJSONUnmarshaler_DecodeTracesError(t *testing.T) {
	decoder := NewJSONTracesUnmarshaler(false)
	_, err := decoder.UnmarshalTraces([]byte("{"))
	assert.Error(t, err)
}

func TestJSONEncoder_EncodeTraces(t *testing.T) {
	marshaler := NewJSONTracesMarshaler()
	buf, err := marshaler.MarshalTraces(generateTraceSingleSpanErrorStatus())
	assert.NoError(t, err)
	assert.Greater(t, len(buf), 1)
}

func TestJSONEncoder_EncodeTracesError(t *testing.T) {
	invalidTD := ptrace.NewTraces()
	// Add one span with empty trace ID.
	invalidTD.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	marshaler := NewJSONTracesMarshaler()
	buf, err := marshaler.MarshalTraces(invalidTD)
	assert.Error(t, err)
	assert.Nil(t, buf)
}

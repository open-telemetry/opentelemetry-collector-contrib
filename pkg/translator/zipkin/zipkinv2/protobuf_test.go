// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestProtobufMarshalUnmarshal(t *testing.T) {
	pb, err := NewProtobufTracesMarshaler().MarshalTraces(generateTraceSingleSpanErrorStatus())
	require.NoError(t, err)

	pbUnmarshaler := protobufUnmarshaler{debugWasSet: false}
	td, err := pbUnmarshaler.UnmarshalTraces(pb)
	assert.NoError(t, err)
	assert.Equal(t, generateTraceSingleSpanErrorStatus(), td)
}

func TestProtobuf_UnmarshalTracesError(t *testing.T) {
	decoder := protobufUnmarshaler{debugWasSet: false}
	_, err := decoder.UnmarshalTraces([]byte("{"))
	assert.Error(t, err)
}

func TestProtobuf_MarshalTracesError(t *testing.T) {
	invalidTD := ptrace.NewTraces()
	// Add one span with empty trace ID.
	invalidTD.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	marshaler := NewProtobufTracesMarshaler()
	buf, err := marshaler.MarshalTraces(invalidTD)
	assert.Nil(t, buf)
	assert.Error(t, err)
}

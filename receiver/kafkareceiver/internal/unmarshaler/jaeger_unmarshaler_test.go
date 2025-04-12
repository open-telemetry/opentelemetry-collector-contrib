// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler

import (
	"bytes"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

func TestUnmarshalJaeger(t *testing.T) {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("foo")
	span.SetStartTimestamp(pcommon.Timestamp(10))
	span.SetEndTimestamp(pcommon.Timestamp(20))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	batches := jaeger.ProtoFromTraces(td)

	protoBytes, err := batches[0].Spans[0].Marshal()
	require.NoError(t, err)

	jsonMarshaler := &jsonpb.Marshaler{}
	jsonBytes := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(jsonBytes, batches[0].Spans[0]))

	tests := []struct {
		unmarshaler ptrace.Unmarshaler
		encoding    string
		bytes       []byte
	}{
		{
			unmarshaler: JaegerProtoSpanUnmarshaler{},
			bytes:       protoBytes,
		},
		{
			unmarshaler: JaegerJSONSpanUnmarshaler{},
			bytes:       jsonBytes.Bytes(),
		},
	}
	for _, test := range tests {
		t.Run(test.encoding, func(t *testing.T) {
			got, err := test.unmarshaler.UnmarshalTraces(test.bytes)
			require.NoError(t, err)
			assert.Equal(t, td, got)
		})
	}
}

func TestUnmarshalJaegerProto_error(t *testing.T) {
	p := JaegerProtoSpanUnmarshaler{}
	_, err := p.UnmarshalTraces([]byte("+$%"))
	assert.Error(t, err)
}

func TestUnmarshalJaegerJSON_error(t *testing.T) {
	p := JaegerJSONSpanUnmarshaler{}
	_, err := p.UnmarshalTraces([]byte("+$%"))
	assert.Error(t, err)
}

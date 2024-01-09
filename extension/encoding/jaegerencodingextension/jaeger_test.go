// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension

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
	batches, err := jaeger.ProtoFromTraces(td)
	require.NoError(t, err)

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
			unmarshaler: jaegerProtobufTrace{},
			encoding:    "jaeger_proto",
			bytes:       protoBytes,
		},
		{
			unmarshaler: jaegerJSONTrace{},
			encoding:    "jaeger_json",
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

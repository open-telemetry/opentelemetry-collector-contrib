// Copyright 2020 The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulsarexporter

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

func buildTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("foo")
	span.SetStartTimestamp(pcommon.Timestamp(10))
	span.SetEndTimestamp(pcommon.Timestamp(20))
	span.SetTraceID(pcommon.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))

	return td
}

func TestJaegerJsonBatchMarshaler(t *testing.T) {
	ptraces := buildTraces()
	batches, err := jaeger.ProtoFromTraces(ptraces)
	require.NoError(t, err)

	jsonMarshaler := &jsonpb.Marshaler{}
	buffer := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(buffer, batches[0]))
	jsonBytes := buffer.Bytes()

	jaegerJsonMarshaler := jaegerMarshaler{
		newJaegerJSONMarshaler(),
	}
	jaegerJsonMessages, err := jaegerJsonMarshaler.Marshal(ptraces, "")
	require.NoError(t, err)
	assert.Equal(t, jaegerJsonMessages[0].Payload, jsonBytes)
}

func TestJaegerProtoBatchMarshaler(t *testing.T) {
	ptraces := buildTraces()
	batches, err := jaeger.ProtoFromTraces(ptraces)
	require.NoError(t, err)

	jaegerProtoBytes, err := batches[0].Marshal()
	require.NoError(t, err)
	require.NotNil(t, jaegerProtoBytes)

	jaegerProtoMarshaler := jaegerMarshaler{
		jaegerProtoBatchMarshaler{},
	}
	jaegerProtoMessage, err := jaegerProtoMarshaler.Marshal(ptraces, "")
	require.NoError(t, err)
	assert.Equal(t, jaegerProtoBytes, jaegerProtoMessage[0].Payload)
}

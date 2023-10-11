// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"testing"
)

func buildTraces(num int) ptrace.Traces {
	td := ptrace.NewTraces()
	for n := 0; n < num; n++ {
		span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("foo")
		span.SetStartTimestamp(pcommon.Timestamp(10))
		span.SetEndTimestamp(pcommon.Timestamp(20))
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	}
	return td
}

func TestJaegerProto(t *testing.T) {
	config := &Config{Protocol: JaegerProtocolProtobuf}
	ex, err := createExtension(context.Background(), extension.CreateSettings{}, config)
	require.NoError(t, err)

	j := ex.(*jaegerExtension)
	err = j.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.NotNil(t, j.marshaler)
	require.NotNil(t, j.unmarshaler)

	trace := buildTraces(5)
	require.NotNil(t, trace)

	bts, err := j.MarshalTraces(trace)
	require.NoError(t, err)

	trace1, err := j.UnmarshalTraces(bts)
	require.NoError(t, err)
	require.Equal(t, trace.SpanCount(), trace1.SpanCount())
}

func TestJaegerProtoCompatible(t *testing.T) {
	config := &Config{Protocol: JaegerProtocolProtobuf}
	ex, err := createExtension(context.Background(), extension.CreateSettings{}, config)
	require.NoError(t, err)

	j := ex.(*jaegerExtension)
	err = j.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.NotNil(t, j.marshaler)
	require.NotNil(t, j.unmarshaler)

	trace := buildTraces(1)

	batch, err := jaeger.ProtoFromTraces(trace)
	require.NoError(t, err)

	bts, err := batch[0].Marshal()
	require.NoError(t, err)

	trace1, err := j.UnmarshalTraces(bts)
	require.NoError(t, err)
	require.Equal(t, trace.SpanCount(), trace1.SpanCount())
}

func Test0(t *testing.T) {
	arr := make([]int, 0, 3)
	arr = append(arr, 0)
	arr = append(arr, 1)
	arr = append(arr, 2)
	arr = append(arr, 3)

}

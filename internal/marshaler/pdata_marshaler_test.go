// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestPdataLogsMarshaler(t *testing.T) {
	input := testdata.GenerateLogs(2)
	compare := func(expected, actual plog.Logs) error { return plogtest.CompareLogs(expected, actual) }
	t.Run("protobuf", func(t *testing.T) {
		testPdataMarshaler(t, input, compare,
			NewPdataLogsMarshaler(&plog.ProtoMarshaler{}).MarshalLogs,
			(&plog.ProtoUnmarshaler{}).UnmarshalLogs,
		)
	})
	t.Run("json", func(t *testing.T) {
		testPdataMarshaler(t, input, compare,
			NewPdataLogsMarshaler(&plog.JSONMarshaler{}).MarshalLogs,
			(&plog.JSONUnmarshaler{}).UnmarshalLogs,
		)
	})
}

func TestPdataMetricsMarshaler(t *testing.T) {
	input := testdata.GenerateMetrics(2)
	compare := func(expected, actual pmetric.Metrics) error { return pmetrictest.CompareMetrics(expected, actual) }
	t.Run("protobuf", func(t *testing.T) {
		testPdataMarshaler(t, input, compare,
			NewPdataMetricsMarshaler(&pmetric.ProtoMarshaler{}).MarshalMetrics,
			(&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics,
		)
	})
	t.Run("json", func(t *testing.T) {
		testPdataMarshaler(t, input, compare,
			NewPdataMetricsMarshaler(&pmetric.JSONMarshaler{}).MarshalMetrics,
			(&pmetric.JSONUnmarshaler{}).UnmarshalMetrics,
		)
	})
}

func TestPdataTracesMarshaler(t *testing.T) {
	input := testdata.GenerateTraces(2)
	compare := func(expected, actual ptrace.Traces) error { return ptracetest.CompareTraces(expected, actual) }
	t.Run("protobuf", func(t *testing.T) {
		testPdataMarshaler(t, input, compare,
			NewPdataTracesMarshaler(&ptrace.ProtoMarshaler{}).MarshalTraces,
			(&ptrace.ProtoUnmarshaler{}).UnmarshalTraces,
		)
	})
	t.Run("json", func(t *testing.T) {
		testPdataMarshaler(t, input, compare,
			NewPdataTracesMarshaler(&ptrace.JSONMarshaler{}).MarshalTraces,
			(&ptrace.JSONUnmarshaler{}).UnmarshalTraces,
		)
	})
}

func testPdataMarshaler[Data any](
	t *testing.T,
	input Data,
	compare func(Data, Data) error,
	marshal func(Data) ([]Message, error),
	unmarshal func([]byte) (Data, error),
) {
	t.Helper()

	messages, err := marshal(input)
	require.NoError(t, err)
	require.Len(t, messages, 1) // 1 message per batch
	assert.Nil(t, messages[0].Key)

	output, err := unmarshal(messages[0].Value)
	require.NoError(t, err)
	assert.NoError(t, compare(input, output))
}

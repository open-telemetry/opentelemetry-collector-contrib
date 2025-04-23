// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/column/orderedmap"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap/zaptest"
)

func Test_attributesToMap(t *testing.T) {
	attributes := pcommon.NewMap()
	attributes.PutStr("key", "value")
	attributes.PutBool("bool", true)
	attributes.PutInt("int", 0)
	attributes.PutDouble("double", 0.0)
	result := AttributesToMap(attributes)
	require.Equal(
		t,
		orderedmap.FromMap(map[string]string{
			"key":    "value",
			"bool":   "true",
			"int":    "0",
			"double": "0",
		}),
		result,
	)
}

func Test_convertExemplars(t *testing.T) {
	SetLogger(zaptest.NewLogger(t))
	t.Run("empty exemplar", func(t *testing.T) {
		exemplars := pmetric.NewExemplarSlice()
		var (
			expectAttrs    clickhouse.ArraySet
			expectTimes    clickhouse.ArraySet
			expectValues   clickhouse.ArraySet
			expectTraceIDs clickhouse.ArraySet
			expectSpanIDs  clickhouse.ArraySet
		)
		attrs, times, values, traceIDs, spanIDs := convertExemplars(exemplars)
		require.Equal(t, expectAttrs, attrs)
		require.Equal(t, expectTimes, times)
		require.Equal(t, expectValues, values)
		require.Equal(t, expectTraceIDs, traceIDs)
		require.Equal(t, expectSpanIDs, spanIDs)
	})
	t.Run("one exemplar with only FilteredAttributes", func(t *testing.T) {
		exemplars := pmetric.NewExemplarSlice()
		exemplar := exemplars.AppendEmpty()
		exemplar.FilteredAttributes().PutStr("key1", "value1")
		exemplar.FilteredAttributes().PutStr("key2", "value2")

		attrs, times, values, traceIDs, spanIDs := convertExemplars(exemplars)
		require.Equal(t, clickhouse.ArraySet{orderedmap.FromMap(map[string]string{"key1": "value1", "key2": "value2"})}, attrs)
		require.Equal(t, clickhouse.ArraySet{time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)}, times)
		require.Equal(t, clickhouse.ArraySet{0.0}, values)
		require.Equal(t, clickhouse.ArraySet{"00000000000000000000000000000000"}, traceIDs)
		require.Equal(t, clickhouse.ArraySet{"0000000000000000"}, spanIDs)
	})
	t.Run("one exemplar with only TimeUnixNano", func(t *testing.T) {
		exemplars := pmetric.NewExemplarSlice()
		exemplar := exemplars.AppendEmpty()
		exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1672218930, 0)))

		attrs, times, values, traceIDs, spanIDs := convertExemplars(exemplars)
		require.Equal(t, clickhouse.ArraySet{orderedmap.FromMap(map[string]string{})}, attrs)
		require.Equal(t, clickhouse.ArraySet{time.Unix(1672218930, 0).UTC()}, times)
		require.Equal(t, clickhouse.ArraySet{0.0}, values)
		require.Equal(t, clickhouse.ArraySet{"00000000000000000000000000000000"}, traceIDs)
		require.Equal(t, clickhouse.ArraySet{"0000000000000000"}, spanIDs)
	})
	t.Run("one exemplar with only DoubleValue ", func(t *testing.T) {
		exemplars := pmetric.NewExemplarSlice()
		exemplar := exemplars.AppendEmpty()
		exemplar.SetDoubleValue(15.0)

		attrs, times, values, traceIDs, spanIDs := convertExemplars(exemplars)
		require.Equal(t, clickhouse.ArraySet{orderedmap.FromMap(map[string]string{})}, attrs)
		require.Equal(t, clickhouse.ArraySet{time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)}, times)
		require.Equal(t, clickhouse.ArraySet{15.0}, values)
		require.Equal(t, clickhouse.ArraySet{"00000000000000000000000000000000"}, traceIDs)
		require.Equal(t, clickhouse.ArraySet{"0000000000000000"}, spanIDs)
	})
	t.Run("one exemplar with only IntValue ", func(t *testing.T) {
		exemplars := pmetric.NewExemplarSlice()
		exemplar := exemplars.AppendEmpty()
		exemplar.SetIntValue(20)

		attrs, times, values, traceIDs, spanIDs := convertExemplars(exemplars)
		require.Equal(t, clickhouse.ArraySet{orderedmap.FromMap(map[string]string{})}, attrs)
		require.Equal(t, clickhouse.ArraySet{time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)}, times)
		require.Equal(t, clickhouse.ArraySet{20.0}, values)
		require.Equal(t, clickhouse.ArraySet{"00000000000000000000000000000000"}, traceIDs)
		require.Equal(t, clickhouse.ArraySet{"0000000000000000"}, spanIDs)
	})
	t.Run("one exemplar with only SpanId", func(t *testing.T) {
		exemplars := pmetric.NewExemplarSlice()
		exemplar := exemplars.AppendEmpty()
		exemplar.SetSpanID([8]byte{1, 2, 3, 4})

		attrs, times, values, traceIDs, spanIDs := convertExemplars(exemplars)
		require.Equal(t, clickhouse.ArraySet{orderedmap.FromMap(map[string]string{})}, attrs)
		require.Equal(t, clickhouse.ArraySet{time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)}, times)
		require.Equal(t, clickhouse.ArraySet{0.0}, values)
		require.Equal(t, clickhouse.ArraySet{"00000000000000000000000000000000"}, traceIDs)
		require.Equal(t, clickhouse.ArraySet{"0102030400000000"}, spanIDs)
	})
	t.Run("one exemplar with only TraceID", func(t *testing.T) {
		exemplars := pmetric.NewExemplarSlice()
		exemplar := exemplars.AppendEmpty()
		exemplar.SetTraceID([16]byte{1, 2, 3, 4})

		attrs, times, values, traceIDs, spanIDs := convertExemplars(exemplars)
		require.Equal(t, clickhouse.ArraySet{orderedmap.FromMap(map[string]string{})}, attrs)
		require.Equal(t, clickhouse.ArraySet{time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)}, times)
		require.Equal(t, clickhouse.ArraySet{0.0}, values)
		require.Equal(t, clickhouse.ArraySet{"01020304000000000000000000000000"}, traceIDs)
		require.Equal(t, clickhouse.ArraySet{"0000000000000000"}, spanIDs)
	})
	t.Run("two exemplars", func(t *testing.T) {
		exemplars := pmetric.NewExemplarSlice()
		exemplar := exemplars.AppendEmpty()
		exemplar.FilteredAttributes().PutStr("key1", "value1")
		exemplar.FilteredAttributes().PutStr("key2", "value2")
		exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1672218930, 0)))
		exemplar.SetDoubleValue(15.0)
		exemplar.SetIntValue(20)
		exemplar.SetSpanID([8]byte{1, 2, 3, 4})
		exemplar.SetTraceID([16]byte{1, 2, 3, 4})

		exemplar = exemplars.AppendEmpty()
		exemplar.FilteredAttributes().PutStr("key3", "value3")
		exemplar.FilteredAttributes().PutStr("key4", "value4")
		exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1672219930, 0)))
		exemplar.SetIntValue(21)
		exemplar.SetDoubleValue(16.0)
		exemplar.SetSpanID([8]byte{1, 2, 3, 5})
		exemplar.SetTraceID([16]byte{1, 2, 3, 5})

		attrs, times, values, traceIDs, spanIDs := convertExemplars(exemplars)
		require.Equal(t, clickhouse.ArraySet{orderedmap.FromMap(map[string]string{"key1": "value1", "key2": "value2"}), orderedmap.FromMap(map[string]string{"key3": "value3", "key4": "value4"})}, attrs)
		require.Equal(t, clickhouse.ArraySet{time.Unix(1672218930, 0).UTC(), time.Unix(1672219930, 0).UTC()}, times)
		require.Equal(t, clickhouse.ArraySet{20.0, 16.0}, values)
		require.Equal(t, clickhouse.ArraySet{"01020304000000000000000000000000", "01020305000000000000000000000000"}, traceIDs)
		require.Equal(t, clickhouse.ArraySet{"0102030400000000", "0102030500000000"}, spanIDs)
	})
}

func Test_convertValueAtQuantile(t *testing.T) {
	t.Run("empty valueAtQuantileSlice", func(t *testing.T) {
		var (
			expectQuantiles clickhouse.ArraySet
			expectValues    clickhouse.ArraySet
		)
		valueAtQuantileSlice := pmetric.NewSummaryDataPointValueAtQuantileSlice()
		quantiles, values := convertValueAtQuantile(valueAtQuantileSlice)
		require.Equal(t, expectQuantiles, quantiles)
		require.Equal(t, expectValues, values)
	})

	t.Run("one valueAtQuantile with only set Value", func(t *testing.T) {
		valueAtQuantileSlice := pmetric.NewSummaryDataPointValueAtQuantileSlice()
		valueAtQuantile := valueAtQuantileSlice.AppendEmpty()
		valueAtQuantile.SetValue(1.0)

		quantiles, values := convertValueAtQuantile(valueAtQuantileSlice)
		require.Equal(t, clickhouse.ArraySet{0.0}, quantiles)
		require.Equal(t, clickhouse.ArraySet{1.0}, values)
	})

	t.Run("one valueAtQuantile with only set Quantile", func(t *testing.T) {
		valueAtQuantileSlice := pmetric.NewSummaryDataPointValueAtQuantileSlice()
		valueAtQuantile := valueAtQuantileSlice.AppendEmpty()
		valueAtQuantile.SetQuantile(1.0)

		quantiles, values := convertValueAtQuantile(valueAtQuantileSlice)
		require.Equal(t, clickhouse.ArraySet{1.0}, quantiles)
		require.Equal(t, clickhouse.ArraySet{0.0}, values)
	})

	t.Run("two valueAtQuantiles", func(t *testing.T) {
		valueAtQuantileSlice := pmetric.NewSummaryDataPointValueAtQuantileSlice()
		valueAtQuantile := valueAtQuantileSlice.AppendEmpty()
		valueAtQuantile.SetQuantile(1.0)
		valueAtQuantile.SetValue(1.0)

		valueAtQuantile = valueAtQuantileSlice.AppendEmpty()
		valueAtQuantile.SetQuantile(2.0)
		valueAtQuantile.SetValue(2.0)

		quantiles, values := convertValueAtQuantile(valueAtQuantileSlice)
		require.Equal(t, clickhouse.ArraySet{1.0, 2.0}, quantiles)
		require.Equal(t, clickhouse.ArraySet{1.0, 2.0}, values)
	})
}

func Test_getValue(t *testing.T) {
	SetLogger(zaptest.NewLogger(t))
	t.Run("set int64 value with NumberDataPointValueType", func(t *testing.T) {
		require.Equal(t, 10.0, getValue(int64(10), 0, pmetric.NumberDataPointValueTypeInt))
	})
	t.Run("set float64 value with NumberDataPointValueType", func(t *testing.T) {
		require.Equal(t, 20.0, getValue(0, 20.0, pmetric.NumberDataPointValueTypeDouble))
	})
	t.Run("set int64 value with ExemplarValueType", func(t *testing.T) {
		require.Equal(t, 10.0, getValue(int64(10), 0, pmetric.ExemplarValueTypeInt))
	})
	t.Run("set float64 value with ExemplarValueType", func(t *testing.T) {
		require.Equal(t, 20.0, getValue(0, 20.0, pmetric.ExemplarValueTypeDouble))
	})
	t.Run("set a unsupport dataType", func(t *testing.T) {
		require.Equal(t, 0.0, getValue(int64(10), 0, pmetric.MetricTypeHistogram))
	})
}

func Test_newPlaceholder(t *testing.T) {
	expectStr := "(?,?,?,?,?),"
	require.Equal(t, newPlaceholder(5), &expectStr)
}

func Test_GetServiceName(t *testing.T) {
	t.Run("should return empty string on unset service.name", func(t *testing.T) {
		require.Empty(t, GetServiceName(pcommon.NewMap()))
	})
	t.Run("should return correct string from service.name", func(t *testing.T) {
		resAttr := pcommon.NewMap()
		resAttr.PutStr(conventions.AttributeServiceName, "test-service")
		require.Equal(t, "test-service", GetServiceName(resAttr))
	})
	t.Run("should return empty string on empty service.name", func(t *testing.T) {
		resAttr := pcommon.NewMap()
		resAttr.PutEmpty(conventions.AttributeServiceName)
		require.Empty(t, GetServiceName(resAttr))
	})
	t.Run("should return string from non-string service.name", func(t *testing.T) {
		resAttr := pcommon.NewMap()
		resAttr.PutBool(conventions.AttributeServiceName, true)
		require.Equal(t, "true", GetServiceName(resAttr))
	})
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
)

func TestStorageKeyEncoding(t *testing.T) {
	makeMetric := func() metrics.Metric {
		res := pcommon.NewResource()
		res.Attributes().PutStr("service.name", "test")
		scope := pcommon.NewInstrumentationScope()
		scope.SetName("sdk")
		scope.SetVersion("1")
		scope.Attributes().PutStr("scope", "test")
		m := pmetric.NewMetric()
		m.SetName("requests")
		m.SetUnit("ms")
		m.SetEmptySum().SetIsMonotonic(true)
		m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		return metrics.From(res, scope, m)
	}
	attrs := pcommon.NewMap()
	attrs.PutStr("route", "/")
	fingerprint := streamFingerprint(makeMetric(), attrs)
	// Pin the encoding so stored keys cannot change unnoticed between releases.
	require.Equal(t, "v1/77f56d575d36ec770e7fec57e8458717a7683aef3d9645d3d36700621134f3ea", streamHashKey(streamID{fingerprint: fingerprint}))
	for _, tt := range []struct {
		name   string
		change func(metrics.Metric)
	}{
		{"resource attributes", func(m metrics.Metric) { m.Resource().Attributes().PutStr("service.name", "other") }},
		{"scope attributes", func(m metrics.Metric) { m.Scope().Attributes().PutStr("scope", "other") }},
		{"scope name", func(m metrics.Metric) { m.Scope().SetName("other") }},
		{"scope version", func(m metrics.Metric) { m.Scope().SetVersion("2") }},
		{"metric name", func(m metrics.Metric) { m.SetName("other") }},
		{"metric unit", func(m metrics.Metric) { m.SetUnit("s") }},
		{"metric type", func(m metrics.Metric) {
			m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		}},
		{"temporality", func(m metrics.Metric) { m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative) }},
		{"monotonic", func(m metrics.Metric) { m.Sum().SetIsMonotonic(false) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := makeMetric()
			tt.change(m)
			require.NotEqual(t, fingerprint, streamFingerprint(m, attrs))
		})
	}
	attrs.PutStr("route", "/other")
	require.NotEqual(t, fingerprint, streamFingerprint(makeMetric(), attrs))
}

func TestStreamHashKey(t *testing.T) {
	makeStream := func(name, unit, scopeName, scopeVersion string) streamID {
		scope := pcommon.NewInstrumentationScope()
		scope.SetName(scopeName)
		scope.SetVersion(scopeVersion)
		metric := pmetric.NewMetric()
		metric.SetName(name)
		metric.SetUnit(unit)
		metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := metric.Sum().DataPoints().AppendEmpty()
		res := pcommon.NewResource()
		return streamID{
			Stream:      identity.OfStream(identity.OfResourceMetric(res, scope, metric), dp),
			fingerprint: streamFingerprint(metrics.From(res, scope, metric), dp.Attributes()),
		}
	}
	for _, tt := range []struct {
		name string
		a, b streamID
	}{
		{"metric boundary", makeStream("foo", "ms", "", ""), makeStream("foom", "s", "", "")},
		{"embedded zero", makeStream("a\x00", "b", "", ""), makeStream("a", "\x00b", "", "")},
		{"scope boundary", makeStream("foo", "s", "ab", "c"), makeStream("foo", "s", "a", "bc")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.a.Hash().Sum64(), tt.b.Hash().Sum64())
			require.NotEqual(t, streamHashKey(tt.a), streamHashKey(tt.b))
		})
	}
	first := makeStream("foo", "s", "sdk", "1")
	second := makeStream("foo", "s", "sdk", "1")
	require.Equal(t, streamHashKey(first), streamHashKey(second))
}

func TestDecodeRejectsWrongTypeAndMalformedData(t *testing.T) {
	number, err := marshalNumberDP(guard(pmetric.NewNumberDataPoint()))
	require.NoError(t, err)
	histogram, err := marshalHistogramDP(guard(pmetric.NewHistogramDataPoint()))
	require.NoError(t, err)
	expo, err := marshalExpoDP(guard(pmetric.NewExponentialHistogramDataPoint()))
	require.NoError(t, err)
	for _, tt := range []struct {
		name   string
		valid  []byte
		decode func([]byte) error
	}{
		{"sum", number, func(b []byte) error { _, err := unmarshalNumberDP(b); return err }},
		{"histogram", histogram, func(b []byte) error { _, err := unmarshalHistogramDP(b); return err }},
		{"exponential histogram", expo, func(b []byte) error { _, err := unmarshalExpoDP(b); return err }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.decode(tt.valid))
			for _, data := range [][]byte{nil, []byte("corrupt"), number, histogram, expo} {
				if bytes.Equal(data, tt.valid) {
					continue
				}
				require.Error(t, tt.decode(data))
			}
		})
	}
}

func TestAttributeEncoding(t *testing.T) {
	for _, tt := range []struct {
		name    string
		value   any
		encoded string
	}{
		{"empty", nil, "6e"},
		{"string", "a\x00b", "7303610062"},
		{"false", false, "6200"},
		{"true", true, "6201"},
		{"integer", int64(-1), "69ffffffffffffffff"},
		{"double", float64(1), "663ff0000000000000"},
		{"negative zero", math.Copysign(0, -1), "668000000000000000"},
		{"nan", math.Float64frombits(0x7ff8000000000001), "667ff8000000000001"},
		{"bytes", []byte{'a', 0, 'b'}, "7803610062"},
		{"map", map[string]any{"b": true, "a": nil}, "6d0201616e01626201"},
		{"slice", []any{nil, "x", []any{true}}, "61036e73017861016201"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			value := pcommon.NewValueEmpty()
			require.NoError(t, value.FromRaw(tt.value))
			require.Equal(t, tt.encoded, hex.EncodeToString(appendAttributeValue(nil, value)))
		})
	}
}

func TestAttributeMapOrderAndBoundaries(t *testing.T) {
	first, second := pcommon.NewMap(), pcommon.NewMap()
	first.PutStr("z", "last")
	first.PutEmptyMap("a").PutInt("x", 1)
	firstValue, _ := first.Get("a")
	firstValue.Map().PutInt("y", 2)
	second.PutEmptyMap("a").PutInt("y", 2)
	secondValue, _ := second.Get("a")
	secondValue.Map().PutInt("x", 1)
	second.PutStr("z", "last")
	require.Equal(t, appendAttributeMap(nil, first), appendAttributeMap(nil, second))
	for _, tt := range []struct {
		name string
		a, b any
	}{
		{"numeric types", int64(1), float64(1)},
		{"bytes and string", []byte("x"), "x"},
		{"slice boundaries", []any{"ab", "c"}, []any{"a", "bc"}},
		{"map boundaries", map[string]any{"ab": "c"}, map[string]any{"a": "bc"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, b := pcommon.NewValueEmpty(), pcommon.NewValueEmpty()
			require.NoError(t, a.FromRaw(tt.a))
			require.NoError(t, b.FromRaw(tt.b))
			require.NotEqual(t, appendAttributeValue(nil, a), appendAttributeValue(nil, b))
		})
	}
}

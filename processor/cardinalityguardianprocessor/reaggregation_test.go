// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// hashAttributes tests.

func TestHashAttributes_EmptyMap(t *testing.T) {
	attrs := pcommon.NewMap()
	assert.Equal(t, uint64(0), hashAttributes(attrs))
}

func TestHashAttributes_OrderIndependent(t *testing.T) {
	// Two maps with the same key-value pairs in different insertion order
	// must produce the same hash.
	a := pcommon.NewMap()
	a.PutStr("method", "GET")
	a.PutStr("status", "200")

	b := pcommon.NewMap()
	b.PutStr("status", "200")
	b.PutStr("method", "GET")

	assert.Equal(t, hashAttributes(a), hashAttributes(b))
}

func TestHashAttributes_DifferentValues(t *testing.T) {
	a := pcommon.NewMap()
	a.PutStr("method", "GET")

	b := pcommon.NewMap()
	b.PutStr("method", "POST")

	assert.NotEqual(t, hashAttributes(a), hashAttributes(b))
}

func TestHashAttributes_DifferentKeys(t *testing.T) {
	a := pcommon.NewMap()
	a.PutStr("method", "GET")

	b := pcommon.NewMap()
	b.PutStr("verb", "GET")

	assert.NotEqual(t, hashAttributes(a), hashAttributes(b))
}

// TestHashAttributes_CrossPairValueSwap verifies that swapping values across
// two keys of the same map (e.g. {method=GET, status=POST} vs
// {method=POST, status=GET}) produces a different hash. A naive XOR of
// per-pair hashes would cancel under this swap; pairHashMix must break that
// symmetry.
func TestHashAttributes_CrossPairValueSwap(t *testing.T) {
	a := pcommon.NewMap()
	a.PutStr("method", "GET")
	a.PutStr("status", "POST")

	b := pcommon.NewMap()
	b.PutStr("method", "POST")
	b.PutStr("status", "GET")

	assert.NotEqual(t, hashAttributes(a), hashAttributes(b),
		"cross-pair value swap must change the hash")
}

func TestHashAttributes_NonStringValueTypes(t *testing.T) {
	// Map-, slice-, and int-typed attribute values with different contents
	// must each produce different pair hashes.
	a := pcommon.NewMap()
	mapValA := a.PutEmptyMap("payload")
	mapValA.PutStr("inner", "alpha")

	b := pcommon.NewMap()
	mapValB := b.PutEmptyMap("payload")
	mapValB.PutStr("inner", "beta")

	assert.NotEqual(t, hashAttributes(a), hashAttributes(b))

	// Slice values too.
	sa := pcommon.NewMap()
	sliceA := sa.PutEmptySlice("payload")
	sliceA.AppendEmpty().SetStr("a")

	sb := pcommon.NewMap()
	sliceB := sb.PutEmptySlice("payload")
	sliceB.AppendEmpty().SetStr("b")

	assert.NotEqual(t, hashAttributes(sa), hashAttributes(sb))

	// Int values too.
	ia := pcommon.NewMap()
	ia.PutInt("n", 42)
	ib := pcommon.NewMap()
	ib.PutInt("n", 43)

	assert.NotEqual(t, hashAttributes(ia), hashAttributes(ib))
}

func TestHashAttributes_SubsetDiffers(t *testing.T) {
	// {a=1, b=2} should differ from {a=1}.
	a := pcommon.NewMap()
	a.PutStr("a", "1")
	a.PutStr("b", "2")

	b := pcommon.NewMap()
	b.PutStr("a", "1")

	assert.NotEqual(t, hashAttributes(a), hashAttributes(b))
}

// Gauge reaggregation tests.

func TestReaggregateGauge_NoCollision(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("host", "web-1")
	dp1.SetDoubleValue(72.5)
	dp1.SetTimestamp(100)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("host", "web-2")
	dp2.SetDoubleValue(68.3)
	dp2.SetTimestamp(200)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeGauge, false)

	// No collisions — both data points should remain.
	assert.Equal(t, 2, dps.Len())
}

func TestReaggregateGauge_TakesLatestTimestamp(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()

	// Two data points with identical attributes (simulating post-strip collision).
	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("method", "GET")
	dp1.SetDoubleValue(72.5)
	dp1.SetTimestamp(100)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("method", "GET")
	dp2.SetDoubleValue(68.3)
	dp2.SetTimestamp(200)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeGauge, false)

	require.Equal(t, 1, dps.Len())
	// Gauge: latest timestamp wins → dp2's value.
	assert.InDelta(t, 68.3, dps.At(0).DoubleValue(), 0.001)
	assert.Equal(t, pcommon.Timestamp(200), dps.At(0).Timestamp())
}

func TestReaggregateGauge_ThreeWayCollision(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("method", "GET")
	dp1.SetDoubleValue(10.0)
	dp1.SetTimestamp(100)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("method", "GET")
	dp2.SetDoubleValue(20.0)
	dp2.SetTimestamp(300) // latest

	dp3 := dps.AppendEmpty()
	dp3.Attributes().PutStr("method", "GET")
	dp3.SetDoubleValue(30.0)
	dp3.SetTimestamp(200)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeGauge, false)

	require.Equal(t, 1, dps.Len())
	assert.InDelta(t, 20.0, dps.At(0).DoubleValue(), 0.001)
	assert.Equal(t, pcommon.Timestamp(300), dps.At(0).Timestamp())
}

func TestReaggregateGauge_IntValues(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("method", "GET")
	dp1.SetIntValue(10)
	dp1.SetTimestamp(100)
	dp1.SetStartTimestamp(50)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("method", "GET")
	dp2.SetIntValue(20)
	dp2.SetTimestamp(200)      // latest wins
	dp2.SetStartTimestamp(150) // > 0, so it should be copied

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeGauge, false)

	require.Equal(t, 1, dps.Len())
	assert.Equal(t, int64(20), dps.At(0).IntValue())
	assert.Equal(t, pcommon.Timestamp(200), dps.At(0).Timestamp())
	assert.Equal(t, pcommon.Timestamp(150), dps.At(0).StartTimestamp())
}

// Delta Sum reaggregation tests.

func TestReaggregateDeltaSum_NoCollision(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("endpoint", "/users")
	dp1.SetIntValue(5)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("endpoint", "/health")
	dp2.SetIntValue(3)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)

	assert.Equal(t, 2, dps.Len())
}

func TestReaggregateDeltaSum_IntValues(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("method", "GET")
	dp1.SetIntValue(5)
	dp1.SetStartTimestamp(10)
	dp1.SetTimestamp(100)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("method", "GET")
	dp2.SetIntValue(3)
	dp2.SetStartTimestamp(20)
	dp2.SetTimestamp(200)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)

	require.Equal(t, 1, dps.Len())
	assert.Equal(t, int64(8), dps.At(0).IntValue())
	// Start timestamp: earlier of the two.
	assert.Equal(t, pcommon.Timestamp(10), dps.At(0).StartTimestamp())
	// End timestamp: later of the two.
	assert.Equal(t, pcommon.Timestamp(200), dps.At(0).Timestamp())
}

func TestReaggregateDeltaSum_DoubleValues(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("method", "POST")
	dp1.SetDoubleValue(1.5)
	dp1.SetStartTimestamp(10)
	dp1.SetTimestamp(100)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("method", "POST")
	dp2.SetDoubleValue(2.5)
	dp2.SetStartTimestamp(5)
	dp2.SetTimestamp(150)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)

	require.Equal(t, 1, dps.Len())
	assert.InDelta(t, 4.0, dps.At(0).DoubleValue(), 0.001)
	assert.Equal(t, pcommon.Timestamp(5), dps.At(0).StartTimestamp())
	assert.Equal(t, pcommon.Timestamp(150), dps.At(0).Timestamp())
}

func TestReaggregateDeltaSum_MixedTypes(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("method", "GET")
	dp1.SetIntValue(5)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("method", "GET")
	dp2.SetDoubleValue(2.5)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)

	require.Equal(t, 1, dps.Len())
	// Mixed types promote to double: 5.0 + 2.5 = 7.5
	assert.InDelta(t, 7.5, dps.At(0).DoubleValue(), 0.001)
}

func TestReaggregateDeltaSum_MultipleGroups(t *testing.T) {
	// Two separate collision groups: {method=GET} and {method=POST}.
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("method", "GET")
	dp1.SetIntValue(10)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("method", "POST")
	dp2.SetIntValue(20)

	dp3 := dps.AppendEmpty()
	dp3.Attributes().PutStr("method", "GET")
	dp3.SetIntValue(30)

	dp4 := dps.AppendEmpty()
	dp4.Attributes().PutStr("method", "POST")
	dp4.SetIntValue(40)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)

	require.Equal(t, 2, dps.Len())

	// Collect merged values by method.
	values := map[string]int64{}
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		method, _ := dp.Attributes().Get("method")
		values[method.Str()] = dp.IntValue()
	}

	assert.Equal(t, int64(40), values["GET"])  // 10 + 30
	assert.Equal(t, int64(60), values["POST"]) // 20 + 40
}

// Edge cases.

func TestReaggregate_MergesExemplars(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("method", "GET")
	dp1.SetIntValue(10)
	ex1 := dp1.Exemplars().AppendEmpty()
	ex1.SetDoubleValue(1.0)

	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("method", "GET")
	dp2.SetIntValue(20)
	ex2 := dp2.Exemplars().AppendEmpty()
	ex2.SetDoubleValue(2.0)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)

	require.Equal(t, 1, dps.Len())
	assert.Equal(t, int64(30), dps.At(0).IntValue())

	// Both exemplars should be preserved in the merged data point.
	require.Equal(t, 2, dps.At(0).Exemplars().Len())
	assert.Equal(t, 1.0, dps.At(0).Exemplars().At(0).DoubleValue())
	assert.Equal(t, 2.0, dps.At(0).Exemplars().At(1).DoubleValue())
}

func TestReaggregate_SingleDataPoint(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()
	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("method", "GET")
	dp1.SetIntValue(42)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)

	require.Equal(t, 1, dps.Len())
	assert.Equal(t, int64(42), dps.At(0).IntValue())
}

func TestReaggregate_EmptySlice(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()
	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)
	assert.Equal(t, 0, dps.Len())
}

func TestReaggregate_EmptyAttributes(t *testing.T) {
	// Two data points with no attributes — they collide on empty identity.
	dps := pmetric.NewNumberDataPointSlice()

	dp1 := dps.AppendEmpty()
	dp1.SetIntValue(10)
	dp1.SetTimestamp(100)

	dp2 := dps.AppendEmpty()
	dp2.SetIntValue(20)
	dp2.SetTimestamp(200)

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)

	require.Equal(t, 1, dps.Len())
	assert.Equal(t, int64(30), dps.At(0).IntValue())
}

// Single-Writer Invariant.

func TestReaggregate_SingleWriterInvariant(t *testing.T) {
	// After reaggregation, no two data points should share the same
	// attribute identity. This is the core invariant that prevents
	// the Single-Writer violation.
	dps := pmetric.NewNumberDataPointSlice()

	// Create 5 data points, 3 of which collide.
	for i := range 3 {
		dp := dps.AppendEmpty()
		dp.Attributes().PutStr("method", "GET")
		dp.SetIntValue(int64(i + 1))
	}
	for i := range 2 {
		dp := dps.AppendEmpty()
		dp.Attributes().PutStr("method", "POST")
		dp.SetIntValue(int64(i + 10))
	}

	reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)

	// Verify: all remaining data points have unique identities.
	seen := make(map[uint64]bool)
	for i := 0; i < dps.Len(); i++ {
		h := hashAttributes(dps.At(i).Attributes())
		assert.False(t, seen[h], "duplicate identity after reaggregation at index %d", i)
		seen[h] = true
	}
}

// addNumberValue and copyNumberValue tests.

func TestAddNumberValue_IntPlusInt(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()
	src := dps.AppendEmpty()
	src.SetIntValue(3)
	dst := dps.AppendEmpty()
	dst.SetIntValue(5)

	addNumberValue(src, dst)
	assert.Equal(t, int64(8), dst.IntValue())
}

func TestAddNumberValue_DoublePlusDouble(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()
	src := dps.AppendEmpty()
	src.SetDoubleValue(1.5)
	dst := dps.AppendEmpty()
	dst.SetDoubleValue(2.5)

	addNumberValue(src, dst)
	assert.InDelta(t, 4.0, dst.DoubleValue(), 0.001)
}

func TestAddNumberValue_IntPlusDouble(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()
	src := dps.AppendEmpty()
	src.SetIntValue(3)
	dst := dps.AppendEmpty()
	dst.SetDoubleValue(2.5)

	addNumberValue(src, dst)
	assert.InDelta(t, 5.5, dst.DoubleValue(), 0.001)
}

func TestAddNumberValue_EmptyType(t *testing.T) {
	dps := pmetric.NewNumberDataPointSlice()
	src := dps.AppendEmpty() // Empty value type
	dst := dps.AppendEmpty() // Empty value type

	addNumberValue(src, dst)
	// Promotes to double because types are not explicitly Int or Double, but evaluates to 0.0 + 0.0
	assert.InDelta(t, 0.0, dst.DoubleValue(), 0.001)
}

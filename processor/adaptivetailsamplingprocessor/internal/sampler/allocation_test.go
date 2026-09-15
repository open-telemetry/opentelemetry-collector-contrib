// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file pin the ported math to dynsampler-go v0.6.4 using
// the library's own test vectors (emathroughput_test.go), so parity is
// verifiable without driving the library's private, timer-based update path.

// Golden vector: TestEMAThroughputSampleUpdateMapsSparseCounts. A steady key
// at 40 events/interval among churning single-count keys settles at rate 4
// for goal 10 events/sec over 1s intervals with weight 0.2.
func TestEMAState_GoldenSparseCounts(t *testing.T) {
	s := newEMAState(0.2, 0)
	rng := rand.New(rand.NewPCG(1, 2))
	var table map[string]int
	for i := 0; i <= 100; i++ {
		input := map[string]float64{"largest_count": 40}
		for range 5 {
			input[fmt.Sprintf("sporadic-%d-%d", i, rng.Int())] = 1
		}
		s.observe(input)
		table = s.rates(10, time.Second)
	}
	require.NotNil(t, table)
	assert.Equal(t, 4, table["largest_count"])
}

// Golden vector: TestEMAThroughputAgesOutSmallValues. A key that stops
// appearing decays below the age-out threshold and is dropped; the average of
// a steady key converges on its true count.
func TestEMAState_GoldenAgesOutSmallValues(t *testing.T) {
	s := newEMAState(0.2, 0)
	for range 100 {
		s.observe(map[string]float64{"foo": 500})
	}
	require.Len(t, s.movingAverage, 1)
	assert.Equal(t, float64(500), math.Round(s.movingAverage["foo"]))

	for range 100 {
		s.observe(map[string]float64{"asdf": 1})
	}
	_, found := s.movingAverage["foo"]
	assert.False(t, found, "unseen key must age out")
	_, found = s.movingAverage["asdf"]
	assert.True(t, found)
}

// An empty interval must not decay the averages (the library deliberately
// skips updates when there was no traffic).
func TestEMAState_EmptyIntervalDoesNotDecay(t *testing.T) {
	s := newEMAState(0.5, 0)
	s.observe(map[string]float64{"foo": 100})
	before := s.movingAverage["foo"]
	s.observe(map[string]float64{})
	assert.Equal(t, before, s.movingAverage["foo"])
}

// Allocation properties that hold for any input: every rate is >= 1, rates
// are monotone in counts (a busier key is never sampled more gently than a
// quieter one), and the expected kept volume is close to the goal when the
// goal is achievable.
func TestEMAState_RateProperties(t *testing.T) {
	s := newEMAState(0.5, 0)
	counts := map[string]float64{
		"huge": 100000, "big": 10000, "mid": 1000, "small": 100, "tiny": 1,
	}
	// Feed the same distribution repeatedly so averages converge to it.
	for range 50 {
		in := make(map[string]float64, len(counts))
		maps.Copy(in, counts)
		s.observe(in)
	}
	const goalPerSec = 100.0
	table := s.rates(goalPerSec, time.Second)
	require.NotNil(t, table)

	assert.GreaterOrEqual(t, table["tiny"], 1)
	assert.GreaterOrEqual(t, table["huge"], table["big"])
	assert.GreaterOrEqual(t, table["big"], table["mid"])
	assert.GreaterOrEqual(t, table["mid"], table["small"])

	var kept float64
	for k, v := range counts {
		kept += v / float64(table[k])
	}
	assert.InDelta(t, goalPerSec, kept, goalPerSec*0.35,
		"expected kept volume should be near the goal (log allocation is approximate)")
}

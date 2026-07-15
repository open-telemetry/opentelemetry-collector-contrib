// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
)

func hashRingDistributionStats(numEndpoints, numIDs int, seed1, seed2 uint64) (float64, int, int) {
	endpoints := make([]string, numEndpoints)
	for i := range endpoints {
		endpoints[i] = fmt.Sprintf("endpoint-%d", i)
	}
	ring := newHashRing(endpoints)
	counts := make(map[string]int, numEndpoints)
	rng := rand.New(rand.NewPCG(seed1, seed2))
	var id [16]byte
	for range numIDs {
		for i := range id {
			id[i] = byte(rng.IntN(256))
		}
		counts[ring.endpointFor(id[:])]++
	}

	mean := float64(numIDs) / float64(numEndpoints)
	var variance float64
	minCount, maxCount := numIDs, 0
	for _, endpoint := range endpoints {
		count := counts[endpoint]
		if count < minCount {
			minCount = count
		}
		if count > maxCount {
			maxCount = count
		}
		diff := float64(count) - mean
		variance += diff * diff
	}

	return math.Sqrt(variance/float64(numEndpoints)) / mean, minCount, maxCount
}

func TestNewHashRing(t *testing.T) {
	// prepare
	endpoints := []string{"endpoint-1", "endpoint-2"}

	// test
	ring := newHashRing(endpoints)

	// verify
	assert.Len(t, ring.items, 2*defaultWeight)
}

func TestEndpointFor(t *testing.T) {
	// prepare
	endpoints := []string{"endpoint-1", "endpoint-2"}
	ring := newHashRing(endpoints)

	for _, tt := range []struct {
		id       []byte
		expected string
	}{
		// check that we are indeed alternating endpoints for different inputs
		{[]byte{1, 2, 0, 0}, "endpoint-2"},
		{[]byte{128, 128, 0, 0}, "endpoint-1"},
		{[]byte("ad-service-7"), "endpoint-2"},
		{[]byte("get-recommendations-1"), "endpoint-2"},
	} {
		t.Run(fmt.Sprintf("Endpoint for id %s", string(tt.id)), func(t *testing.T) {
			// test
			endpoint := ring.endpointFor(tt.id)

			// verify
			assert.Equal(t, tt.expected, endpoint)
		})
	}
}

func TestPositionsFor(t *testing.T) {
	// prepare
	endpoint := "host1"

	// test
	positions := positionsFor(endpoint, 10)

	// verify
	assert.Len(t, positions, 10)
}

func TestHashRingDistributionAcrossManyEndpoints(t *testing.T) {
	for _, tt := range []struct {
		name         string
		numEndpoints int
		numIDs       int
		seed1        uint64
		seed2        uint64
		maxCV        float64
		maxRatio     float64
	}{
		{name: "10 endpoints seed A", numEndpoints: 10, numIDs: 20_000, seed1: 10, seed2: 41200, maxCV: 0.10, maxRatio: 1.35},
		{name: "10 endpoints seed B", numEndpoints: 10, numIDs: 20_000, seed1: 11, seed2: 99, maxCV: 0.10, maxRatio: 1.35},
		{name: "100 endpoints seed A", numEndpoints: 100, numIDs: 50_000, seed1: 100, seed2: 41200, maxCV: 0.10, maxRatio: 1.70},
		{name: "100 endpoints seed B", numEndpoints: 100, numIDs: 50_000, seed1: 101, seed2: 99, maxCV: 0.10, maxRatio: 1.70},
		{name: "300 endpoints seed A", numEndpoints: 300, numIDs: 100_000, seed1: 300, seed2: 41200, maxCV: 0.12, maxRatio: 1.95},
		{name: "300 endpoints seed B", numEndpoints: 300, numIDs: 100_000, seed1: 301, seed2: 99, maxCV: 0.12, maxRatio: 1.95},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cv, minCount, maxCount := hashRingDistributionStats(tt.numEndpoints, tt.numIDs, tt.seed1, tt.seed2)
			t.Logf("cv=%0.5f min=%d max=%d max/min=%0.2f", cv, minCount, maxCount, float64(maxCount)/float64(minCount))
			assert.LessOrEqual(t, cv, tt.maxCV, "coefficient of variation; min=%d max=%d", minCount, maxCount)
			assert.LessOrEqual(t, float64(maxCount)/float64(minCount), tt.maxRatio)
		})
	}
}

func TestHashRingIsInvariantToEndpointOrder(t *testing.T) {
	const numEndpoints = 300

	endpoints := make([]string, numEndpoints)
	reversed := make([]string, numEndpoints)
	for i := range endpoints {
		endpoints[i] = fmt.Sprintf("endpoint-%d", i)
		reversed[numEndpoints-1-i] = endpoints[i]
	}

	assert.True(t, newHashRing(endpoints).equal(newHashRing(reversed)))
}

func TestHashRingRetainsFullWeightAcrossManyEndpoints(t *testing.T) {
	const numEndpoints = 300

	endpoints := make([]string, numEndpoints)
	counts := make(map[string]int, numEndpoints)
	for i := range endpoints {
		endpoints[i] = fmt.Sprintf("endpoint-%d", i)
	}

	items := positionsForEndpoints(endpoints, defaultWeight)
	assert.Len(t, items, numEndpoints*defaultWeight)
	for _, item := range items {
		counts[item.endpoint]++
	}
	for _, endpoint := range endpoints {
		assert.Equal(t, defaultWeight, counts[endpoint], endpoint)
	}
}

func TestPositionsForEndpointsDropsWhenProbeLimitIsExhausted(t *testing.T) {
	items := positionsForEndpointsWithOptions([]string{"endpoint-1", "endpoint-2", "endpoint-3"}, 2, 1, 1)

	assert.Len(t, items, 1)
	assert.Equal(t, position(0), items[0].pos)
}

func TestBinarySearch(t *testing.T) {
	// prepare
	items := []ringItem{
		{pos: 14},
		{pos: 25},
		{pos: 33},
		{pos: 47},
		{pos: 56},
		{pos: 121},
		{pos: 134},
		{pos: 158},
		{pos: 240},
		{pos: 270},
		{pos: 350},
	}
	ringSize := len(items)
	left, right := items[:ringSize/2], items[ringSize/2:]

	for _, tt := range []struct {
		requested position
		expected  position
	}{
		{position(85), position(121)},
		{position(14), position(14)},
		{position(351), position(14)},
		{position(270), position(270)},
		{position(271), position(350)},
	} {
		t.Run(fmt.Sprintf("Angle %d Requested", uint32(tt.requested)), func(t *testing.T) {
			// test
			found := bsearch(tt.requested, left, right)

			// verify
			assert.Equal(t, tt.expected, found.pos)
		})
	}
}

func TestPositionsForEndpoints(t *testing.T) {
	for _, tt := range []struct {
		name      string
		endpoints []string
		expected  []ringItem
	}{
		{
			"Single Endpoint",
			[]string{"endpoint-1"},
			[]ringItem{
				// this was first calculated by running the algorithm and taking its output
				{pos: 0x4be4, endpoint: "endpoint-1"},
				{pos: 0x666a, endpoint: "endpoint-1"},
				{pos: 0xad6f, endpoint: "endpoint-1"},
				{pos: 0x1a4c1, endpoint: "endpoint-1"},
				{pos: 0x1bae2, endpoint: "endpoint-1"},
			},
		},
		{
			"Duplicate Endpoint",
			[]string{"endpoint-1", "endpoint-1"},
			[]ringItem{
				// We expect to not have duplicate items.
				// When a clash occurs, the next free positions should be taken. In this case, there will always be
				// exactly one clash because of duplicate endpoints. So, the pos will always be i and i+1.
				{pos: 0x4be4, endpoint: "endpoint-1"},
				{pos: 0x4be5, endpoint: "endpoint-1"},
				{pos: 0x666a, endpoint: "endpoint-1"},
				{pos: 0x666b, endpoint: "endpoint-1"},
				{pos: 0xad6f, endpoint: "endpoint-1"},
				{pos: 0xad70, endpoint: "endpoint-1"},
				{pos: 0x1a4c1, endpoint: "endpoint-1"},
				{pos: 0x1a4c2, endpoint: "endpoint-1"},
				{pos: 0x1bae2, endpoint: "endpoint-1"},
				{pos: 0x1bae3, endpoint: "endpoint-1"},
			},
		},
		{
			"Multiple Endpoints",
			[]string{"endpoint-A", "endpoint-B"},
			[]ringItem{
				// we expect to have 5 positions for each endpoint
				{pos: 0x61a4, endpoint: "endpoint-B"},
				{pos: 0xca91, endpoint: "endpoint-A"},
				{pos: 0x10b72, endpoint: "endpoint-A"},
				{pos: 0x11372, endpoint: "endpoint-B"},
				{pos: 0x116b1, endpoint: "endpoint-B"},
				{pos: 0x11801, endpoint: "endpoint-A"},
				{pos: 0x18a36, endpoint: "endpoint-B"},
				{pos: 0x1af01, endpoint: "endpoint-B"},
				{pos: 0x1af16, endpoint: "endpoint-A"},
				{pos: 0x1e723, endpoint: "endpoint-A"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// test
			items := positionsForEndpoints(tt.endpoints, 5)

			// verify
			assert.Equal(t, tt.expected, items)
		})
	}
}

func TestEqual(t *testing.T) {
	original := &hashRing{
		[]ringItem{
			{pos: position(123), endpoint: "endpoint-1"},
		},
	}

	for _, tt := range []struct {
		name      string
		candidate *hashRing
		outcome   bool
	}{
		{
			"empty",
			&hashRing{[]ringItem{}},
			false,
		},
		{
			"null",
			nil,
			false,
		},
		{
			"equal",
			&hashRing{
				[]ringItem{
					{pos: position(123), endpoint: "endpoint-1"},
				},
			},
			true,
		},
		{
			"different length",
			&hashRing{
				[]ringItem{
					{pos: position(123), endpoint: "endpoint-1"},
					{pos: position(124), endpoint: "endpoint-2"},
				},
			},
			false,
		},
		{
			"different position",
			&hashRing{
				[]ringItem{
					{pos: position(124), endpoint: "endpoint-1"},
				},
			},
			false,
		},
		{
			"different endpoint",
			&hashRing{
				[]ringItem{
					{pos: position(123), endpoint: "endpoint-2"},
				},
			},
			false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.outcome, original.equal(tt.candidate))
		})
	}
}

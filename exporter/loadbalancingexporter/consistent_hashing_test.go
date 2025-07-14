// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		{[]byte("get-recommendations-1"), "endpoint-1"},
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
				{pos: 0x21ca, endpoint: "endpoint-1"},
				{pos: 0x29d3, endpoint: "endpoint-1"},
				{pos: 0x3984, endpoint: "endpoint-1"},
				{pos: 0x5eaf, endpoint: "endpoint-1"},
				{pos: 0x8bc1, endpoint: "endpoint-1"},
			},
		},
		{
			"Duplicate Endpoint",
			[]string{"endpoint-1", "endpoint-1"},
			[]ringItem{
				// We expect to not have duplicate items.
				// When a clash occurs, the next free positions should be taken. In this case, there will always be
				// exactly one clash because of duplicate endpoints. So, the pos will always be i and i+1.
				{pos: 0x21ca, endpoint: "endpoint-1"},
				{pos: 0x21cb, endpoint: "endpoint-1"},
				{pos: 0x29d3, endpoint: "endpoint-1"},
				{pos: 0x29d4, endpoint: "endpoint-1"},
				{pos: 0x3984, endpoint: "endpoint-1"},
				{pos: 0x3985, endpoint: "endpoint-1"},
				{pos: 0x5eaf, endpoint: "endpoint-1"},
				{pos: 0x5eb0, endpoint: "endpoint-1"},
				{pos: 0x8bc1, endpoint: "endpoint-1"},
				{pos: 0x8bc2, endpoint: "endpoint-1"},
			},
		},
		{
			"Multiple Endpoints",
			[]string{"endpoint-A", "endpoint-B"},
			[]ringItem{
				// we expect to have 5 positions for each endpoint
				{pos: 0xdde, endpoint: "endpoint-B"},
				{pos: 0x162e, endpoint: "endpoint-A"},
				{pos: 0x21f5, endpoint: "endpoint-B"},
				{pos: 0x34e5, endpoint: "endpoint-A"},
				{pos: 0x61fb, endpoint: "endpoint-B"},
				{pos: 0x6910, endpoint: "endpoint-B"},
				{pos: 0x76a0, endpoint: "endpoint-A"},
				{pos: 0x7e2b, endpoint: "endpoint-A"},
				{pos: 0x7f7c, endpoint: "endpoint-A"},
				{pos: 0x85ac, endpoint: "endpoint-B"},
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

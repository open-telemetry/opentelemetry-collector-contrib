// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		{[]byte{1, 2, 0, 0}, "endpoint-1"},
		{[]byte{128, 128, 0, 0}, "endpoint-2"},
		{[]byte("ad-service-7"), "endpoint-1"},
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
				{pos: 1401, endpoint: "endpoint-1"},
				{pos: 4175, endpoint: "endpoint-1"},
				{pos: 14133, endpoint: "endpoint-1"},
				{pos: 17836, endpoint: "endpoint-1"},
				{pos: 21667, endpoint: "endpoint-1"},
			},
		},
		{
			"Duplicate Endpoint",
			[]string{"endpoint-1", "endpoint-1"},
			[]ringItem{
				// we expect to not have duplicate items
				{pos: 1401, endpoint: "endpoint-1"},
				{pos: 4175, endpoint: "endpoint-1"},
				{pos: 14133, endpoint: "endpoint-1"},
				{pos: 17836, endpoint: "endpoint-1"},
				{pos: 21667, endpoint: "endpoint-1"},
			},
		},
		{
			"Multiple Endpoints",
			[]string{"endpoint-1", "endpoint-2"},
			[]ringItem{
				// we expect to have 5 positions for each endpoint
				{pos: 1401, endpoint: "endpoint-1"},
				{pos: 4175, endpoint: "endpoint-1"},
				{pos: 10240, endpoint: "endpoint-2"},
				{pos: 14133, endpoint: "endpoint-1"},
				{pos: 15002, endpoint: "endpoint-2"},
				{pos: 17836, endpoint: "endpoint-1"},
				{pos: 21263, endpoint: "endpoint-2"},
				{pos: 21667, endpoint: "endpoint-1"},
				{pos: 26806, endpoint: "endpoint-2"},
				{pos: 27020, endpoint: "endpoint-2"},
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

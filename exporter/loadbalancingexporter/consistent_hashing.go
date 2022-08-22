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

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"hash/crc32"
	"sort"
)

const maxPositions uint32 = 36000 // 360 degrees with two decimal places
const defaultWeight int = 100     // the number of points in the ring for each entry. For better results, it should be higher than 100.

// position represents a specific angle in the ring.
// Each entry in the ring is positioned at an angle in a hypothetical circle, meaning that it ranges from 0 to 360.
type position uint32

// ringItem connects a specific angle in the ring with a specific endpoint.
type ringItem struct {
	pos      position
	endpoint string
}

// hashRing is a consistent hash ring following Karger et al.
type hashRing struct {
	// ringItems holds all the positions, used for the lookup the position for the closest next ring item
	items []ringItem
}

// newHashRing builds a new immutable consistent hash ring based on the given endpoints.
func newHashRing(endpoints []string) *hashRing {
	items := positionsForEndpoints(endpoints, defaultWeight)
	return &hashRing{
		items: items,
	}
}

// endpointFor calculates which backend is responsible for the given traceID
func (h *hashRing) endpointFor(identifier []byte) string {
	if h == nil {
		// perhaps the ring itself couldn't get initialized yet?
		return ""
	}
	hasher := crc32.NewIEEE()
	hasher.Write(identifier)
	hash := hasher.Sum32()
	pos := hash % maxPositions

	return h.findEndpoint(position(pos))
}

// findEndpoint returns the "next" endpoint starting from the given position, or an empty string in case no endpoints are available
func (h *hashRing) findEndpoint(pos position) string {
	ringSize := len(h.items)
	if ringSize == 0 {
		return ""
	}
	left, right := h.items[:ringSize/2], h.items[ringSize/2:]
	found := bsearch(pos, left, right)
	return found.endpoint
}

// bsearch is a binary search-like algorithm, returning the closest "next" item instead of an exact match
func bsearch(pos position, left []ringItem, right []ringItem) ringItem {
	// if it's the last item of the left side, return it
	if left[len(left)-1].pos == pos {
		return left[len(left)-1]
	}

	// if it's the first item of the right side, return it
	if right[0].pos == pos {
		return right[0]
	}

	// if we want a higher angle than the highest from the ring, the first angle is the right one
	if pos > right[len(right)-1].pos {
		return left[0]
	}

	// if the requested position is higher than the highest in the left, the item is in the right side
	if pos > left[len(left)-1].pos {
		size := len(right)
		if size == 1 {
			return right[0]
		}

		l, r := right[:size/2], right[size/2:]
		return bsearch(pos, l, r)
	}

	// not on the right side, has to be on the left side
	size := len(left)
	if size == 1 {
		return left[0]
	}
	l, r := left[:size/2], left[size/2:]
	return bsearch(pos, l, r)
}

// positionFor calculates all the positions in the ring based. The numPoints indicates how many positions to calculate.
// The slice length of the result matches the numPoints.
func positionsFor(endpoint string, numPoints int) []position {
	res := make([]position, 0, numPoints)
	for i := 0; i < numPoints; i++ {
		h := crc32.NewIEEE()
		h.Write([]byte(endpoint))
		h.Write([]byte{byte(i)})
		hash := h.Sum32()
		pos := hash % maxPositions
		res = append(res, position(pos))
	}

	return res
}

// positionsForEndpoints calculates all the positions for all the given endpoints
func positionsForEndpoints(endpoints []string, weight int) []ringItem {
	var items []ringItem
	positions := map[position]bool{} // tracking the used positions
	for _, endpoint := range endpoints {
		// for this initial implementation, we don't allow endpoints to have custom weights
		for _, pos := range positionsFor(endpoint, weight) {
			// if this position is occupied already, skip this item
			if _, found := positions[pos]; found {
				continue
			}
			positions[pos] = true

			item := ringItem{
				pos:      pos,
				endpoint: endpoint,
			}
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].pos < items[j].pos
	})

	return items
}

func (h *hashRing) equal(candidate *hashRing) bool {
	if candidate == nil {
		return false
	}

	if len(h.items) != len(candidate.items) {
		return false
	}
	for i := range candidate.items {
		if h.items[i].endpoint != candidate.items[i].endpoint {
			return false
		}
		if h.items[i].pos != candidate.items[i].pos {
			return false
		}
	}
	return true
}

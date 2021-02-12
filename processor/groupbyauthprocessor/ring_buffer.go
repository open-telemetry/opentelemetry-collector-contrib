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

package groupbyauthprocessor

// ringBuffer keeps an in-memory bounded buffer with the in-flight tokens
type ringBuffer struct {
	index        int
	size         int
	tokens       []string
	tokenToIndex map[string]int // key is token, value is the index on the 'tokens' slice
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		index:        -1, // the first span to be received will be placed at position '0'
		size:         size,
		tokens:       make([]string, size),
		tokenToIndex: make(map[string]int),
	}
}

func (r *ringBuffer) put(token string) string {
	// calculates the item in the ring that we'll store the trace
	r.index = (r.index + 1) % r.size

	// see if the ring has an item already
	evicted := r.tokens[r.index]

	if evicted != "" {
		// clear space for the new item
		r.delete(evicted)
	}

	// place the traceID in memory
	r.tokens[r.index] = token
	r.tokenToIndex[token] = r.index

	return evicted
}

func (r *ringBuffer) contains(token string) bool {
	_, found := r.tokenToIndex[token]
	return found
}

func (r *ringBuffer) delete(token string) bool {
	index, found := r.tokenToIndex[token]
	if !found {
		return false
	}

	delete(r.tokenToIndex, token)
	r.tokens[index] = ""
	return true
}

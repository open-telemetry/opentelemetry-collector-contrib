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

package websocketprocessor

import "sync"

// channelSet is a collection of byte channels where adding, removing, and writing to
// the channels is synchronized.
type channelSet struct {
	i       int
	mu      sync.Mutex
	chanmap map[int]chan []byte
}

func newChannelSet() *channelSet {
	return &channelSet{
		chanmap: map[int]chan []byte{},
	}
}

// add adds the channel to the channelSet and returns a key (just an int) used to
// remove the channel later.
func (c *channelSet) add(ch chan []byte) int {
	c.mu.Lock()
	idx := c.i
	c.chanmap[idx] = ch
	c.i++
	c.mu.Unlock()
	return idx
}

// writeBytes writes the passed in bytes to all of the channels in the
// channelSet.
func (c *channelSet) writeBytes(bytes []byte) {
	c.mu.Lock()
	for _, ch := range c.chanmap {
		ch <- bytes
	}
	c.mu.Unlock()
}

// closeAndRemove closes then removes the channel associated with the passed in
// key. Panics if an invalid key is passed in.
func (c *channelSet) closeAndRemove(key int) {
	c.mu.Lock()
	close(c.chanmap[key])
	delete(c.chanmap, key)
	c.mu.Unlock()
}

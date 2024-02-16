// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor"

import "sync"

// channelSet is a collection of byte channels where adding, removing, and writing to
// the channels is synchronized.
type channelSet struct {
	i       int
	mu      sync.RWMutex
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
	c.mu.RLock()
	for _, ch := range c.chanmap {
		ch <- bytes
	}
	c.mu.RUnlock()
}

// closeAndRemove closes then removes the channel associated with the passed in
// key. Panics if an invalid key is passed in.
func (c *channelSet) closeAndRemove(key int) {
	c.mu.Lock()
	close(c.chanmap[key])
	delete(c.chanmap, key)
	c.mu.Unlock()
}

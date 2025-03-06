// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ttlmap // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"

import (
	"sync"
	"time"
)

// TTLMap is a map that evicts entries after the configured ttl has elapsed.
type TTLMap struct {
	md            *ttlMapData
	done          chan struct{}
	sweepInterval int64
}

// New creates a TTLMap. The sweepIntervalSeconds arg indicates how often
// entries are checked for expiration. The maxAgeSeconds arg indicates how long
// entries can persist before getting evicted. Call Start() on the returned
// TTLMap to begin periodic sweeps which check for expiration and evict entries
// as needed.
// done is the channel that will be used to signal to the timer to stop its work.
func New(sweepIntervalSeconds int64, maxAgeSeconds int64, done chan struct{}) *TTLMap {
	return &TTLMap{
		sweepInterval: sweepIntervalSeconds,
		md:            newTTLMapData(maxAgeSeconds),
		done:          done,
	}
}

// Start starts periodic sweeps for expired entries in the underlying map.
func (m *TTLMap) Start() {
	go func() {
		ticker := time.NewTicker(time.Duration(m.sweepInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case now := <-ticker.C:
				m.md.sweep(now.Unix())
			case <-m.done:
				return
			}
		}
	}()
}

// Put adds the passed-in key and value to the underlying map. The current time
// is attached to the entry for periodic expiration checking and eviction when
// necessary.
func (m *TTLMap) Put(k string, v any) {
	m.md.put(k, v, time.Now().Unix())
}

// Get returns the object in the underlying map at the given key. If there is no
// value at that key, Get returns nil.
func (m *TTLMap) Get(k string) any {
	return m.md.get(k)
}

func (m *TTLMap) Shutdown() {
	if m.done != nil {
		close(m.done)
	}
}

type entry struct {
	v          any
	createTime int64
}

type ttlMapData struct {
	m      map[string]entry
	maxAge int64
	mux    sync.Mutex
}

func newTTLMapData(maxAgeSeconds int64) *ttlMapData {
	return &ttlMapData{
		maxAge: maxAgeSeconds,
		m:      map[string]entry{},
		mux:    sync.Mutex{},
	}
}

func (d *ttlMapData) put(k string, v any, currTime int64) {
	d.mux.Lock()
	d.m[k] = entry{v: v, createTime: currTime}
	d.mux.Unlock()
}

func (d *ttlMapData) get(k string) any {
	d.mux.Lock()
	defer d.mux.Unlock()
	entry, ok := d.m[k]
	if !ok {
		return nil
	}
	return entry.v
}

func (d *ttlMapData) sweep(currTime int64) {
	d.mux.Lock()
	for k, v := range d.m {
		if currTime-v.createTime > d.maxAge {
			delete(d.m, k)
		}
	}
	d.mux.Unlock()
}

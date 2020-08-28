// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translation

import (
	"sync"
	"time"
)

// ttlMap is a map that evicts entries after the configured ttl has elapsed.
type ttlMap struct {
	md            *ttlMapData
	sweepInterval int64
}

// newTTLMap creates a ttlMap. The maxAgeSeconds arg indicates how long
// entries can persist before getting evicted. The sweepIntervalSeconds arg
// indicates how often entries are checked for expiration. Call start() on the
// returned ttlMap to begin periodic sweeps which check for expiration and evict
// entries as needed.
func newTTLMap(maxAgeSeconds int64, sweepIntervalSeconds int64) *ttlMap {
	return &ttlMap{
		md:            newTTLMapData(maxAgeSeconds),
		sweepInterval: sweepIntervalSeconds,
	}
}

func (m *ttlMap) start() {
	go func() {
		d := time.Duration(m.sweepInterval) * time.Second
		for range time.Tick(d) {
			m.md.sweep(time.Now().Unix())
		}
	}()
}

func (m *ttlMap) put(k string, v interface{}) {
	m.md.put(k, v, time.Now().Unix())
}

func (m *ttlMap) get(k string) interface{} {
	return m.md.get(k)
}

type entry struct {
	createTime int64
	v          interface{}
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

func (d *ttlMapData) put(k string, v interface{}, currTime int64) {
	d.mux.Lock()
	d.m[k] = entry{v: v, createTime: currTime}
	d.mux.Unlock()
}

func (d *ttlMapData) get(k string) interface{} {
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

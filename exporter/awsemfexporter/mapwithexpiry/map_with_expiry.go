// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mapwithexpiry

import (
	"sync"
	"time"
)

type mapEntry struct {
	creation time.Time
	content  interface{}
}

// MapWithExpiry act like a map which provide a method to clean up expired entries
type MapWithExpiry struct {
	lock    *sync.Mutex
	ttl     time.Duration
	entries map[interface{}]*mapEntry
}

func NewMapWithExpiry(ttl time.Duration) *MapWithExpiry {
	return &MapWithExpiry{lock: &sync.Mutex{}, ttl: ttl, entries: make(map[interface{}]*mapEntry)}
}

func (m *MapWithExpiry) CleanUp(now time.Time) {
	for k, v := range m.entries {
		if now.Sub(v.creation) >= m.ttl {
			delete(m.entries, k)
		}
	}
}

func (m *MapWithExpiry) Get(key interface{}) (interface{}, bool) {
	res, ok := m.entries[key]
	if ok {
		return res.content, true
	}
	return nil, false
}

func (m *MapWithExpiry) Set(key interface{}, content interface{}) {
	m.entries[key] = &mapEntry{content: content, creation: time.Now()}
}

func (m *MapWithExpiry) Size() int {
	return len(m.entries)
}

func (m *MapWithExpiry) Lock() {
	m.lock.Lock()
}

func (m *MapWithExpiry) Unlock() {
	m.lock.Unlock()
}

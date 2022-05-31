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

package store // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor/internal/store"

import (
	"container/list"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	defaultTTL      = 2 * time.Second
	defaultMaxItems = 1000
)

var (
	ErrTooManyItems = errors.New("too many items")
)

type Config struct {
	TTL      time.Duration `mapstructure:"ttl"`
	MaxItems int           `mapstructure:"max_items"`
}

func (c *Config) validate() error {
	if c.TTL < 0 {
		return fmt.Errorf("invalid ttl %v, use a positive value", c.TTL)
	}
	if c.MaxItems < 0 {
		return fmt.Errorf("invalid max_items %v, use a positive value", c.MaxItems)
	}
	return nil
}

var _ Store = (*store)(nil)

type store struct {
	l   *list.List
	mtx sync.Mutex
	m   map[string]*list.Element

	onComplete Callback
	onExpire   Callback

	ttl      time.Duration
	maxItems int
}

// NewStore creates a Store to build service graphs. The store caches edges, each representing a
// request between two services. Once an edge is complete its metrics can be collected. Edges that
// have not found their pair are deleted after ttl time.
func NewStore(cfg Config, onComplete, onExpire Callback) (Store, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid store config: %w", err)
	}

	ttl, maxItems := defaultTTL, defaultMaxItems
	if cfg.TTL != 0 {
		ttl = cfg.TTL
	}
	if cfg.MaxItems != 0 {
		maxItems = cfg.MaxItems
	}

	s := &store{
		l: list.New(),
		m: make(map[string]*list.Element),

		onComplete: onComplete,
		onExpire:   onExpire,

		ttl:      ttl,
		maxItems: maxItems,
	}

	return s, nil
}

func (s *store) len() int {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.l.Len()
}

// tryEvictHead checks if the oldest item (head of list) can be evicted and will delete it if so.
// Returns true if the head was evicted.
//
// Must be called holding lock.
func (s *store) tryEvictHead() bool {
	head := s.l.Front()
	if head == nil {
		// list is empty
		return false
	}

	headEdge := head.Value.(*Edge)
	if !headEdge.isExpired() {
		return false
	}

	s.onExpire(headEdge)
	delete(s.m, headEdge.key)
	s.l.Remove(head)

	return true
}

// UpsertEdge fetches an Edge from the store and updates it using the given callback. If the Edge
// doesn't exist yet, it creates a new one with the default TTL.
// If the Edge is complete after applying the callback, it's completed and removed.
func (s *store) UpsertEdge(key string, update Callback) (bool, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if storedEdge, ok := s.m[key]; ok {
		edge := storedEdge.Value.(*Edge)
		update(edge)

		if edge.isComplete() {
			s.onComplete(edge)
			delete(s.m, key)
			s.l.Remove(storedEdge)
		}

		return false, nil
	}

	// Check we can add new edges
	if s.l.Len() >= s.maxItems {
		// TODO: try to evict expired items
		return false, ErrTooManyItems
	}

	edge := newEdge(key, s.ttl)
	ele := s.l.PushBack(edge)
	s.m[key] = ele
	update(edge)

	return true, nil
}

// Expire evicts all expired items in the store.
func (s *store) Expire() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for s.tryEvictHead() {
	}
}

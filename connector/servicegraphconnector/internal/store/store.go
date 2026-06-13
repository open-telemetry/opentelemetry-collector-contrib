// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/store"

import (
	"container/list"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

var ErrTooManyItems = errors.New("too many items")

type Callback func(e *Edge)

type Key struct {
	tid pcommon.TraceID
	sid pcommon.SpanID
}

func (k *Key) SpanIDIsEmpty() bool {
	return k.sid.IsEmpty()
}

func NewKey(tid pcommon.TraceID, sid pcommon.SpanID) Key {
	return Key{tid: tid, sid: sid}
}

type Store struct {
	l   *list.List
	mtx sync.Mutex
	m   map[Key]*list.Element
	// pending maps a producer key to consumer keys that referenced the producer
	// via Links() but arrived before the producer. Reconciled when producer is seen.
	pending map[Key][]Key

	onComplete Callback
	onExpire   Callback

	ttl      time.Duration
	maxItems int
}

// NewStore creates a Store to build service graphs. The store caches edges, each representing a
// request between two services. Once an edge is complete its metrics can be collected. Edges that
// have not found their pair are deleted after ttl time.
func NewStore(ttl time.Duration, maxItems int, onComplete, onExpire Callback) *Store {
	s := &Store{
		l:       list.New(),
		m:       make(map[Key]*list.Element),
		pending: make(map[Key][]Key),

		onComplete: onComplete,
		onExpire:   onExpire,

		ttl:      ttl,
		maxItems: maxItems,
	}

	return s
}

// Len is only used for testing.
func (s *Store) Len() int {
	return s.l.Len()
}

// UpsertEdge fetches an Edge from the store and updates it using the given callback. If the Edge
// doesn't exist yet, it creates a new one with the default TTL.
// If the Edge is complete after applying the callback, it's completed and removed.
func (s *Store) UpsertEdge(key Key, update Callback) (isNew bool, err error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Helper to reconcile pending consumers for a producer key. Assumes lock held.
	reconcile := func(producerKey Key) {
		prodElem, ok := s.m[producerKey]
		if !ok {
			// no producer stored yet
			return
		}
		prodEdge := prodElem.Value.(*Edge)

		consumers := s.pending[producerKey]
		for _, cKey := range consumers {
			cElem, ok := s.m[cKey]
			if !ok {
				continue
			}

			cEdge := cElem.Value.(*Edge)
			// copy client/producer info into consumer edge
			cEdge.ClientService = prodEdge.ClientService
			cEdge.ClientLatencySec = prodEdge.ClientLatencySec
			cEdge.ConnectionType = prodEdge.ConnectionType
			cEdge.Failed = cEdge.Failed || prodEdge.Failed

			if cEdge.isComplete() {
				s.onComplete(cEdge)
				delete(s.m, cKey)
				s.l.Remove(cElem)
			}
		}
		// After reconciling pending consumers, remove the producer entry from
		// the store to avoid duplicate completions on expiry. Consumers have
		// already been completed with the producer's client info.
		delete(s.pending, producerKey)
		// remove producer element so it doesn't expire later and produce extra metrics
		delete(s.m, producerKey)
		s.l.Remove(prodElem)
	}

	if storedEdge, ok := s.m[key]; ok {
		edge := storedEdge.Value.(*Edge)
		update(edge)

		// If this updated edge is a producer and there are pending consumers,
		// reconcile them. We consider this key as a producer key when there
		// are pending entries mapped to it.
		if consumers, ok := s.pending[key]; ok && len(consumers) > 0 {
			reconcile(key)
			// Do not delete the producer edge here; keep it to serve future
			// consumers until it expires.
			return false, nil
		}

		if edge.isComplete() {
			s.onComplete(edge)
			delete(s.m, key)
			s.l.Remove(storedEdge)
		}

		return false, nil
	}

	// Creating a new edge
	edge := newEdge(key, s.ttl)
	update(edge)

	// If this is a consumer edge that references a producer via ProducerKey,
	// either reconcile immediately if the producer exists, or register this
	// consumer in the pending index so it can be reconciled later.
	if !edge.ProducerKey.SpanIDIsEmpty() {
		producerKey := edge.ProducerKey
		if prodElem, ok := s.m[producerKey]; ok {
			// Producer already present: copy client info and complete if possible.
			prodEdge := prodElem.Value.(*Edge)
			edge.ClientService = prodEdge.ClientService
			edge.ClientLatencySec = prodEdge.ClientLatencySec
			edge.ConnectionType = prodEdge.ConnectionType
			edge.Failed = edge.Failed || prodEdge.Failed

			if edge.isComplete() {
				s.onComplete(edge)
				return true, nil
			}
		} else {
			// Producer not present: register pending consumer for later reconciliation.
			s.pending[producerKey] = append(s.pending[producerKey], key)
		}
	}

	// Check we can add new edges
	if s.l.Len() >= s.maxItems {
		// TODO: try to evict expired items
		return false, ErrTooManyItems
	}

	ele := s.l.PushBack(edge)
	s.m[key] = ele

	// If this is a producer and there are pending consumers, reconcile now.
	if consumers, ok := s.pending[key]; ok && len(consumers) > 0 {
		reconcile(key)
	}

	return true, nil
}

// Expire evicts all expired items in the store.
func (s *Store) Expire() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Iterates until no more items can be evicted
	for s.tryEvictHead() {
	}
}

// tryEvictHead checks if the oldest item (head of list) can be evicted and will delete it if so.
// Returns true if the head was evicted.
//
// Must be called holding lock.
func (s *Store) tryEvictHead() bool {
	head := s.l.Front()
	if head == nil {
		return false // list is empty
	}

	headEdge := head.Value.(*Edge)
	if !headEdge.isExpired() {
		return false
	}

	// If this edge is a pending consumer, remove it from the pending map.
	if !headEdge.ProducerKey.SpanIDIsEmpty() {
		pKey := headEdge.ProducerKey
		// remove headEdge.Key from s.pending[pKey]
		if listKeys, ok := s.pending[pKey]; ok {
			newList := listKeys[:0]
			for _, k := range listKeys {
				if k != headEdge.Key {
					newList = append(newList, k)
				}
			}
			if len(newList) == 0 {
				delete(s.pending, pKey)
			} else {
				s.pending[pKey] = newList
			}
		}
	}

	s.onExpire(headEdge)
	delete(s.m, headEdge.Key)
	s.l.Remove(head)

	return true
}

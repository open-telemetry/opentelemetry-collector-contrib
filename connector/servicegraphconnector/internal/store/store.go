// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package store // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/store"

import (
	"container/list"
	"errors"
	"maps"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

var ErrTooManyItems = errors.New("too many items")

type Callback func(e *Edge)

type keyKind uint8

const (
	standardKey keyKind = iota
	linkedConsumerKey
)

type Key struct {
	kind keyKind

	traceID pcommon.TraceID
	spanID  pcommon.SpanID

	linkedTraceID pcommon.TraceID
	linkedSpanID  pcommon.SpanID
}

func (k Key) SpanIDIsEmpty() bool {
	return k.spanID.IsEmpty()
}

func NewKey(traceID pcommon.TraceID, spanID pcommon.SpanID) Key {
	return Key{
		kind:    standardKey,
		traceID: traceID,
		spanID:  spanID,
	}
}

func NewLinkedConsumerKey(
	consumerTraceID pcommon.TraceID,
	consumerSpanID pcommon.SpanID,
	producerTraceID pcommon.TraceID,
	producerSpanID pcommon.SpanID,
) Key {
	return Key{
		kind:          linkedConsumerKey,
		traceID:       consumerTraceID,
		spanID:        consumerSpanID,
		linkedTraceID: producerTraceID,
		linkedSpanID:  producerSpanID,
	}
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
	reconcile := func(producerKey Key, prodEdge *Edge) {
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
			prodEdge.IsMatched = true

			maps.Copy(cEdge.Dimensions, prodEdge.Dimensions)
			maps.Copy(cEdge.Peer, prodEdge.Peer)

			if cEdge.isComplete() {
				s.onComplete(cEdge)
				delete(s.m, cKey)
				s.l.Remove(cElem)
			}
		}
		// After reconciling pending consumers, clear the pending index for this producer.
		delete(s.pending, producerKey)
	}

	if storedEdge, ok := s.m[key]; ok {
		edge := storedEdge.Value.(*Edge)
		update(edge)

		// If this updated edge is a producer and there are pending consumers,
		// reconcile them. We consider this key as a producer key when there
		// are pending entries mapped to it.
		if consumers, ok := s.pending[key]; ok && len(consumers) > 0 {
			reconcile(key, edge)
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
	// we want to reconcile immediately if the producer exists.
	// If the producer is missing, we flag it to be registered as pending
	// ONLY AFTER it passes the capacity check.
	var isPendingConsumer bool
	if !edge.ProducerKey.SpanIDIsEmpty() {
		producerKey := edge.ProducerKey
		if prodElem, ok := s.m[producerKey]; ok {
			// Producer already present: copy client info and complete if possible.
			prodEdge := prodElem.Value.(*Edge)
			edge.ClientService = prodEdge.ClientService
			edge.ClientLatencySec = prodEdge.ClientLatencySec
			edge.ConnectionType = prodEdge.ConnectionType
			edge.Failed = edge.Failed || prodEdge.Failed
			prodEdge.IsMatched = true

			maps.Copy(edge.Dimensions, prodEdge.Dimensions)
			maps.Copy(edge.Peer, prodEdge.Peer)
		} else {
			// Producer not present: mark for deferred registration.
			isPendingConsumer = true
		}
	}

	// Restore the fast-path for edges that become complete immediately
	// (e.g., single-span Database requests) or after matching a producer above.
	if edge.isComplete() {
		s.onComplete(edge)
		return true, nil
	}

	// If this is a producer and there are pending consumers, reconcile now.
	// We reconcile before checking capacity so that completed consumers are freed.
	if consumers, ok := s.pending[key]; ok && len(consumers) > 0 {
		reconcile(key, edge)
	}

	// Check we can add new edges (Evict if necessary)
	if s.l.Len() >= s.maxItems && !s.tryEvictHead() {
		return false, ErrTooManyItems
	}

	ele := s.l.PushBack(edge)
	s.m[key] = ele

	// Now that the edge is safely in the store, register its pending status
	if isPendingConsumer {
		s.pending[edge.ProducerKey] = append(s.pending[edge.ProducerKey], key)
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

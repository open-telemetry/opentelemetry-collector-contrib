// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/tracetracker/cache.go

package tracetracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/tracetracker"

import (
	"container/list"
	"sync"
	"time"
)

type CacheKey struct {
	dimName  string
	dimValue string
	value    string
}

type cacheElem struct {
	LastSeen time.Time
	Obj      *CacheKey
}

type TimeoutCache struct {
	sync.Mutex

	// How long to keep sending metrics for a particular service name after it
	// is last seen
	timeout time.Duration
	// A linked list of keys sorted by time last seen
	keysByTime *list.List
	// Which keys are active currently.  The value is an entry in the
	// keysByTime linked list so that it can be quickly accessed and
	// moved to the back of the list.
	keysActive map[CacheKey]*list.Element

	// Internal metrics
	ActiveCount int64
	PurgedCount int64

	maxSize         int64
	maxSizeExpiryTS time.Time
}

// returns whether the cache is full
func (t *TimeoutCache) IsFull() bool {
	t.Lock()
	defer t.Unlock()
	if time.Now().Before(t.maxSizeExpiryTS) {
		return int64(len(t.keysActive)) >= t.maxSize
	}
	return false
}

func (t *TimeoutCache) SetMaxSize(maxSize int64, now time.Time) {
	t.Lock()
	defer t.Unlock()
	t.maxSize = maxSize
	t.maxSizeExpiryTS = now.Add(time.Hour * 1)
}

// RunIfKeyDoesNotExist locks and runs the supplied function if the key does not exist.
// Be careful not to perform cache operations inside of this function because they will deadlock
func (t *TimeoutCache) RunIfKeyDoesNotExist(o *CacheKey, fn func()) {
	t.Lock()
	defer t.Unlock()
	if _, ok := t.keysActive[*o]; ok {
		return
	}
	fn()
}

// UpdateOrCreate
func (t *TimeoutCache) UpdateOrCreate(o *CacheKey, now time.Time) (isNew bool) {
	t.Lock()
	defer t.Unlock()
	if timeElm, ok := t.keysActive[*o]; ok {
		if timeElm.Value.(*cacheElem).LastSeen.Before(now) {
			timeElm.Value.(*cacheElem).LastSeen = now
			t.keysByTime.MoveToFront(timeElm)
		}
	} else {
		isNew = true
		elm := t.keysByTime.PushFront(&cacheElem{
			LastSeen: now,
			Obj:      o,
		})
		t.keysActive[*o] = elm
		t.ActiveCount++
	}
	return
}

// UpdateIfExists
func (t *TimeoutCache) UpdateIfExists(o *CacheKey, now time.Time) bool {
	t.Lock()
	defer t.Unlock()
	var timeElm *list.Element
	var exists bool
	if timeElm, exists = t.keysActive[*o]; exists {
		timeElm.Value.(*cacheElem).LastSeen = now
		t.keysByTime.MoveToFront(timeElm)
	}
	return exists
}

func (t *TimeoutCache) GetPurgeable(now time.Time) []*CacheKey {
	t.Lock()
	defer t.Unlock()

	var candidates []*CacheKey
	elm := t.keysByTime.Back()
	for elm != nil {
		e := elm.Value.(*cacheElem)
		// If this one isn't timed out, nothing else in the list is either.
		if now.Sub(e.LastSeen) < t.timeout {
			break
		}

		candidates = append(candidates, e.Obj)

		elm = elm.Prev()
	}

	return candidates
}

func (t *TimeoutCache) Delete(key *CacheKey) {
	t.Lock()
	defer t.Unlock()

	elem, ok := t.keysActive[*key]
	if ok {
		t.keysByTime.Remove(elem)
		delete(t.keysActive, *key)

		t.ActiveCount--
		t.PurgedCount++
	}
}

// PurgeOld
func (t *TimeoutCache) PurgeOld(now time.Time, onPurge func(*CacheKey)) {
	t.Lock()
	defer t.Unlock()
	for {
		elm := t.keysByTime.Back()
		if elm == nil {
			break
		}
		e := elm.Value.(*cacheElem)
		// If this one isn't timed out, nothing else in the list is either.
		if now.Sub(e.LastSeen) < t.timeout {
			break
		}

		t.keysByTime.Remove(elm)
		delete(t.keysActive, *e.Obj)
		onPurge(e.Obj)

		t.ActiveCount--
		t.PurgedCount++
	}
}

func (t *TimeoutCache) GetActiveCount() int64 {
	t.Lock()
	defer t.Unlock()
	return t.ActiveCount
}

func (t *TimeoutCache) GetPurgedCount() int64 {
	t.Lock()
	defer t.Unlock()
	return t.PurgedCount
}

func NewTimeoutCache(timeout time.Duration) *TimeoutCache {
	return &TimeoutCache{
		timeout:    timeout,
		keysByTime: list.New(),
		keysActive: make(map[CacheKey]*list.Element),
	}
}

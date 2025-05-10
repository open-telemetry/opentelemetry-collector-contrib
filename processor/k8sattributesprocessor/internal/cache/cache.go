package cache

import (
	"sync"
	"time"
)

const (
	DEFAULT_CACHE_EXPIRATION_INTERVAL = 5 * time.Minute
	DEFAULT_CACHE_SIZE                = 256
)

type entry struct {
	value       interface{}
	expiryTime  time.Time
	ackWaitTime time.Time
}

// SyncMapWithExpiry is a sync.Map with expiry functionality
type SyncMapWithExpiry struct {
	data sync.Map
	ttl  time.Duration
}

// NewSyncMapWithExpiry creates a new SyncMapWithExpiry
func NewSyncMapWithExpiry(ttl time.Duration) *SyncMapWithExpiry {
	m := &SyncMapWithExpiry{
		ttl: ttl,
	}
	go m.cleanupExpiredEntries()
	return m
}

// Store adds an item to the map with the current time
func (m *SyncMapWithExpiry) Store(key, value interface{}, expiryTime, ackWaitTime time.Duration) {
	if expiryTime <= 0 {
		expiryTime = m.ttl
	}
	m.data.Store(key, entry{
		value:       value,
		expiryTime:  time.Now().Add(expiryTime),
		ackWaitTime: time.Now().Add(ackWaitTime),
	})
}

// Load retrieves an item from the map, considering expiry
func (m *SyncMapWithExpiry) Load(key interface{}) (value interface{}, ok bool, expiryTime, akWaitTime time.Duration) {
	if e, ok := m.data.Load(key); ok {
		if entry, valid := e.(entry); valid && entry.expiryTime.After(time.Now()) {
			return entry.value, true, entry.expiryTime.Sub(time.Now()), entry.ackWaitTime.Sub(time.Now())
		}
		// If the entry has expired, delete it
		m.data.Delete(key)
	}
	return nil, false, 0, 0
}

// Delete removes an item from the map
func (m *SyncMapWithExpiry) Delete(key interface{}) {
	m.data.Delete(key)
}

func (m *SyncMapWithExpiry) Range(f func(key, value interface{}, expiryTime, ackWaitTime time.Duration) bool) {
	m.data.Range(func(key, value interface{}) bool {
		return f(key, value.(entry).value, value.(entry).expiryTime.Sub(time.Now()), value.(entry).ackWaitTime.Sub(time.Now()))
	})
}

// cleanupExpiredEntries periodically cleans up expired entries
func (m *SyncMapWithExpiry) cleanupExpiredEntries() {
	for {
		time.Sleep(m.ttl)
		m.data.Range(func(key, value interface{}) bool {
			if entry, ok := value.(entry); ok && entry.expiryTime.Before(time.Now()) {
				m.data.Delete(key)
			}
			return true
		})
	}
}

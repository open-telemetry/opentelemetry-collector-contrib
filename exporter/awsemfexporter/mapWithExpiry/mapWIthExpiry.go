package mapWithExpiry

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
	lock	*sync.Mutex
	ttl    time.Duration
	entries map[string]*mapEntry
}

func NewMapWithExpiry(ttl time.Duration) *MapWithExpiry {
	return &MapWithExpiry{lock: &sync.Mutex{}, ttl: ttl, entries: make(map[string]*mapEntry)}
}

func (m *MapWithExpiry) CleanUp(now time.Time) {
	m.lock.Lock()
	for k, v := range m.entries {
		if now.Sub(v.creation) >= m.ttl {
			delete(m.entries, k)
		}
	}
	m.lock.Unlock()
}

func (m *MapWithExpiry) Get(key string) (interface{}, bool) {
	m.lock.Lock()
	res, ok := m.entries[key]
	m.lock.Unlock()
	if ok {
		return res.content, true
	}
	return nil, false
}

func (m *MapWithExpiry) Set(key string, content interface{}) {
	m.lock.Lock()
	m.entries[key] = &mapEntry{content: content, creation: time.Now()}
	m.lock.Unlock()
}

func (m *MapWithExpiry) Size() int {
	return len(m.entries)
}
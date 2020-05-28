package mapWithExpiry

import "time"

type mapEntry struct {
	creation time.Time
	content  interface{}
}

// MapWithExpiry act like a map which provide a method to clean up expired entries
type MapWithExpiry struct {
	ttl    time.Duration
	entris map[string]*mapEntry
}

func NewMapWithExpiry(ttl time.Duration) *MapWithExpiry {
	return &MapWithExpiry{ttl: ttl, entris: make(map[string]*mapEntry)}
}

func (m *MapWithExpiry) CleanUp(now time.Time) {
	for k, v := range m.entris {
		if now.Sub(v.creation) >= m.ttl {
			delete(m.entris, k)
		}
	}
}

func (m *MapWithExpiry) Get(key string) (interface{}, bool) {
	res, ok := m.entris[key]
	if ok {
		return res.content, true
	}
	return nil, false
}

func (m *MapWithExpiry) Set(key string, content interface{}) {
	m.entris[key] = &mapEntry{content: content, creation: time.Now()}
}

func (m *MapWithExpiry) Size() int {
	return len(m.entris)
}

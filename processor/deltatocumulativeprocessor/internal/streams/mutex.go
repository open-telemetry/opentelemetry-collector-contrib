// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type mtx[T any, M Map[T]] struct {
	Map M
	sync.RWMutex
}

func (m *mtx[T, M]) Load(id identity.Stream) (T, bool) {
	m.RLock()
	defer m.RUnlock()
	return m.Map.Load(id)
}

func (m *mtx[T, M]) Store(id identity.Stream, v T) {
	m.Lock()
	m.Map.Store(id, v)
	m.Unlock()
}

func (m *mtx[T, M]) Delete(id identity.Stream) {
	m.Lock()
	m.Map.Delete(id)
	m.Unlock()
}

func (m *mtx[T, M]) Items() func(yield func(identity.Stream, T) bool) bool {
	return func(yield func(identity.Stream, T) bool) bool {
		m.RLock()
		defer m.RUnlock()
		return m.Map.Items()(yield)
	}
}

func (m *mtx[T, M]) Len() int {
	m.RLock()
	defer m.RUnlock()
	return m.Map.Len()
}

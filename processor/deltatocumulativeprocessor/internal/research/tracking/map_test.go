// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package atomic_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type Addable struct{}

func (*Addable) Add(int64) int64 { panic("unreachable") }

type AtomicInt64 struct {
	atomic.Int64
}

func BenchmarkMap(b *testing.B) {
	b.Run("value=atomic.Int64", new(AtomicInt64).benchmark)
	b.Run("value=MutexInt64", new(MutexInt64).benchmark)
}

func BenchmarkAdd(b *testing.B) {
	b.Run("type=mutex", func(b *testing.B) {
		v := new(MutexInt64)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				v.Add(1)
			}
		})
	})

	b.Run("type=atomic", func(b *testing.B) {
		v := new(atomic.Int64)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				v.Add(1)
			}
		})
	})
}

type MutexInt64 struct {
	mtx sync.Mutex
	v   int64
}

func (m *MutexInt64) Add(delta int64) int64 {
	m.mtx.Lock()
	m.v += delta
	out := m.v
	m.mtx.Unlock()
	return out
}

func makeids(n int) []identity.Stream {
	ids := make([]identity.Stream, max(1, n/10))
	for i := range ids {
		dp := pmetric.NewNumberDataPoint()
		dp.Attributes().PutInt("i", int64(i))
		ids[i] = identity.OfStream(identity.Metric{}, dp)
	}
	return ids
}

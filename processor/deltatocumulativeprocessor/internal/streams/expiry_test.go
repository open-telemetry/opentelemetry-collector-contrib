// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"
	exp "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/clock"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
)

func TestExpiry(t *testing.T) {
	fake := clock.Fake()
	clock.Change(fake)
	staleness.NowFunc = func() time.Time { return fake.Now() }

	tm := TimeMap[data.Number]{
		Map: make(exp.HashMap[data.Number]),
		add: make(map[identity.Stream]time.Time),
		del: make(map[identity.Stream]time.Time),
	}
	const maxStale = time.Minute
	exp := streams.ExpireAfter(tm, maxStale)

	var mtx sync.Mutex
	go func() {
		for {
			mtx.Lock()
			next := exp.ExpireOldEntries()
			mtx.Unlock()
			<-next
		}
	}()

	sum := random.Sum()
	mtx.Lock()
	now := fake.Now()
	for i := 0; i < 10; i++ {
		r := rand.Intn(10)
		now = now.Add(time.Duration(r) * time.Second)
		fake.Set(now)

		id, dp := sum.Stream()
		err := exp.Store(id, dp)
		require.NoError(t, err)
	}
	mtx.Unlock()

	go func() {
		for {
			now = now.Add(time.Second)
			fake.Set(now)
			time.Sleep(2 * time.Millisecond)
		}
	}()

	for {
		mtx.Lock()
		n := tm.Len()
		mtx.Unlock()
		if n == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	for id := range tm.add {
		add := tm.add[id]
		del := tm.del[id]
		require.Equal(t, maxStale, del.Sub(add))
	}
}

type TimeMap[T any] struct {
	streams.Map[T]

	add map[streams.Ident]time.Time
	del map[streams.Ident]time.Time
}

func (t TimeMap[T]) Store(id streams.Ident, v T) error {
	t.add[id] = clock.Now()
	return t.Map.Store(id, v)
}

func (t TimeMap[T]) Delete(id streams.Ident) {
	t.del[id] = clock.Now()
	t.Map.Delete(id)
}

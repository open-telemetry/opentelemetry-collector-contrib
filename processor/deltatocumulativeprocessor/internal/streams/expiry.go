// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
)

func ExpireAfter[T any](ctx context.Context, items streams.Map[T], ttl time.Duration) streams.Map[T] {
	exp := Expiry[T]{
		mtx: mtx[T, *staleness.Staleness[T]]{Map: staleness.NewStaleness(ttl, items)},
		sig: make(chan struct{}),
	}
	go exp.Start(ctx)
	return &exp
}

type Expiry[T any] struct {
	mtx[T, *staleness.Staleness[T]]
	sig chan struct{}
}

func (e *Expiry[T]) Start(ctx context.Context) {
	for {
		e.mtx.Lock()
		e.Map.ExpireOldEntries()
		e.mtx.Unlock()

		n := e.Map.Len()
		switch {
		case n == 0:
			// no more items: sleep until next write
			select {
			case <-e.sig:
			case <-ctx.Done():
				return
			}
		case n > 0:
			// sleep until earliest possible next expiry time
			_, ts := e.Map.Next()
			at := time.Until(ts.Add(e.Map.Max))

			select {
			case <-time.After(at):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (e *Expiry[T]) Store(id identity.Stream, v T) {
	e.mtx.Lock()
	e.mtx.Store(id, v)
	e.mtx.Unlock()

	// "try-send" to notify possibly sleeping expiry routine
	select {
	case e.sig <- struct{}{}:
	default:
	}
}

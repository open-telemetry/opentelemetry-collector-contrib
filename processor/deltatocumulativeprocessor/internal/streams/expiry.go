// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/staleness"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/clock"
)

func ExpireAfter[T any](items streams.Map[T], ttl time.Duration) Expiry[T] {
	exp := Expiry[T]{
		Staleness: *staleness.NewStaleness[T](ttl, items),
		sig:       make(chan struct{}),
	}
	return exp
}

var _ streams.Map[any] = (*Expiry[any])(nil)

type Expiry[T any] struct {
	staleness.Staleness[T]
	sig chan struct{}
}

func (e Expiry[T]) ExpireOldEntries() <-chan struct{} {
	e.Staleness.ExpireOldEntries()

	n := e.Staleness.Len()
	sig := make(chan struct{})

	go func() {
		switch {
		case n == 0:
			<-e.sig
		case n > 0:
			expires := e.Staleness.Next().Add(e.Max)
			until := clock.Until(expires)
			clock.Sleep(until)
		}
		close(sig)
	}()
	return sig
}

func (e Expiry[T]) Store(id identity.Stream, v T) error {
	err := e.Staleness.Store(id, v)

	// "try-send" to notify possibly sleeping expiry routine
	select {
	case e.sig <- struct{}{}:
	default:
	}

	return err
}

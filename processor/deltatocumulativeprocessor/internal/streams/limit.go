// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
)

func Limit[T any](m Map[T], max int) LimitMap[T] {
	return LimitMap[T]{
		Map: m, Max: max,
		Evictor: EvictorFunc(func() (identity.Stream, bool) {
			return identity.Stream{}, false
		}),
	}
}

type LimitMap[T any] struct {
	Max int

	Evictor streams.Evictor
	streams.Map[T]
}

func (m LimitMap[T]) Store(id identity.Stream, v T) error {
	_, exist := m.Map.Load(id)

	var errEv error
	// if not already tracked and no space: try to evict
	if !exist && m.Map.Len() >= m.Max {
		errl := ErrLimit(m.Max)
		gone, ok := m.Evictor.Evict()
		if !ok {
			// if no eviction possible, fail as there is no space
			return errl
		}
		errEv = ErrEvicted{ErrLimit: errl, Ident: gone}
	}

	// there was space, or we made space: store it
	if err := m.Map.Store(id, v); err != nil {
		return err
	}

	// we may have evicted something, let the caller know
	return errEv
}

type ErrLimit int

func (e ErrLimit) Error() string {
	return fmt.Sprintf("stream limit of %d reached", e)
}

func AtLimit(err error) bool {
	var errLimit ErrLimit
	return errors.As(err, &errLimit)
}

type ErrEvicted struct {
	ErrLimit
	Ident Ident
}

func (e ErrEvicted) Error() string {
	return fmt.Sprintf("%s. evicted stream %s", e.ErrLimit, e.Ident)
}

type EvictorFunc func() (identity.Stream, bool)

func (ev EvictorFunc) Evict() (identity.Stream, bool) {
	return ev()
}

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
	return LimitMap[T]{Map: m, Max: max}
}

type LimitMap[T any] struct {
	Max int

	Evictor streams.Evictor
	streams.Map[T]
}

func (m LimitMap[T]) Store(id identity.Stream, v T) error {
	if m.Map.Len() < m.Max {
		return m.Map.Store(id, v)
	}

	errl := ErrLimit(m.Max)
	if m.Evictor != nil {
		gone := m.Evictor.Evict()
		if err := m.Map.Store(id, v); err != nil {
			return err
		}
		return ErrEvicted{ErrLimit: errl, id: gone}
	}
	return errl
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
	id Ident
}

func (e ErrEvicted) Error() string {
	return fmt.Sprintf("%s. evicted stream %s", e.ErrLimit, e.id)
}

func (e ErrEvicted) Unwrap() error {
	return e.ErrLimit
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
)

func Limit[T any](m Map[T], max int) Map[T] {
	return LimitMap[T]{Map: m, Max: max}
}

type LimitMap[T any] struct {
	Max int
	streams.Map[T]
}

func (m LimitMap[T]) Store(id identity.Stream, v T) error {
	if m.Map.Len() >= m.Max {
		return ErrLimit(m.Max)
	}
	return m.Map.Store(id, v)
}

type ErrLimit int

func (e ErrLimit) Error() string {
	return fmt.Sprintf("stream limit of %d reached", e)
}

func AtLimit(err error) bool {
	var errLimit ErrLimit
	return errors.As(err, &errLimit)
}

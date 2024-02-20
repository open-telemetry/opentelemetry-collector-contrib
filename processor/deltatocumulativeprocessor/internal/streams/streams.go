// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
)

type Ident = identity.Stream

type (
	Seq[T any] streams.Seq[T]
	Map[T any] streams.Map[T]
)

func EmptyMap[T any]() streams.HashMap[T] {
	return streams.EmptyMap[T]()
}
func SyncMap[T any](m streams.Map[T]) streams.Map[T] {
	return streams.SyncMap(m)
}

// Aggregators keep a running aggregate on a per-series basis
type Aggregator[D data.Point[D]] interface {
	// Start any long-running operations.
	// Must be called once and before any Aggregate() call
	Start(context.Context) error
	// Aggregate sample D of series
	Aggregate(context.Context, Ident, D) (D, error)
}

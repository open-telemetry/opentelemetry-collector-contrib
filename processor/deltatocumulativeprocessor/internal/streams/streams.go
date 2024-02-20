// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"context"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
)

type Ident = identity.Stream

type (
	Seq[T any] streams.Seq[T]
	Map[T any] streams.Map[T]
)

func EmptyMap[T any]() streams.Map[T] {
	return streams.EmptyMap[T]()
}

// Aggregators keep a running aggregate on a per-series basis
type Aggregator[D data.Point[D]] interface {
	// Aggregate sample D of series
	Aggregate(context.Context, Ident, D) (D, error)
}

func ExpireAfter[T any](ctx context.Context, items streams.Map[T], ttl time.Duration) streams.Map[T] {
	return streams.ExpireAfter(ctx, items, ttl)
}

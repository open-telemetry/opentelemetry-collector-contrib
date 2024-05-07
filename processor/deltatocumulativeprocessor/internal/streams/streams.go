// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
)

type Ident = identity.Stream

type (
	Seq[T any] streams.Seq[T]
	Map[T any] streams.Map[T]
)

type Aggregator[D data.Point[D]] interface {
	Aggregate(Ident, D) (D, error)
}

func IntoAggregator[D data.Point[D]](m Map[D]) MapAggr[D] {
	return MapAggr[D]{Map: m}
}

type MapAggr[D data.Point[D]] struct {
	Map[D]
}

func (a MapAggr[D]) Aggregate(id Ident, dp D) (D, error) {
	err := a.Map.Store(id, dp)
	v, _ := a.Map.Load(id)
	return v, err
}

type Evictor = streams.Evictor

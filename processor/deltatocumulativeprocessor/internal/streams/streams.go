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

func EmptyMap[T any]() streams.HashMap[T] {
	return streams.EmptyMap[T]()
}

type Aggregator[D data.Point[D]] interface {
	Aggregate(Ident, D) (D, error)
}

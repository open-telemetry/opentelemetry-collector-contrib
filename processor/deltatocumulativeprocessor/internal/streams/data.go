// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
)

// Samples returns an Iterator over each sample of all streams in the metric
func Samples[D data.Point[D]](m metrics.Data[D]) Seq[D] {
	mid := m.Ident()

	return func(yield func(Ident, D) bool) bool {
		for i := 0; i < m.Len(); i++ {
			dp := m.At(i)
			id := identity.OfStream(mid, dp)
			if !yield(id, dp) {
				break
			}
		}
		return false
	}
}

// Aggregate each point and replace it by the result
func Aggregate[D data.Point[D]](m metrics.Data[D], aggr Aggregator[D]) error {
	var errs error

	// for id, dp := range Samples(m)
	Samples(m)(func(id Ident, dp D) bool {
		next, err := aggr.Aggregate(id, dp)
		if err != nil {
			errs = errors.Join(errs, Error(id, err))
			return true
		}
		next.CopyTo(dp)
		return true
	})

	return errs
}

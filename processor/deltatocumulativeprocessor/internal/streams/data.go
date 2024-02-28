// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
)

// Iterator as per https://go.dev/wiki/RangefuncExperiment
type Iter[V any] func(yield func(Ident, V) bool)

// Samples returns an Iterator over each sample of all streams in the metric
func Samples[D data.Point[D]](m metrics.Data[D]) Iter[D] {
	mid := m.Ident()

	return func(yield func(Ident, D) bool) {
		for i := 0; i < m.Len(); i++ {
			dp := m.At(i)
			id := Identify(mid, dp.Attributes())
			if !yield(id, dp) {
				break
			}
		}
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

func Identify(metric metrics.Ident, attrs pcommon.Map) Ident {
	return Ident{metric: metric, attrs: pdatautil.MapHash(attrs)}
}

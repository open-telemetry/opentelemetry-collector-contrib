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

type filterable[D data.Point[D]] interface {
	metrics.Data[D]
	Filter(func(D) bool)
}

// Apply does dps[i] = fn(dps[i]) for each item in dps.
// If fn returns [streams.Drop], the datapoint is removed from dps instead.
// If fn returns another error, the datapoint is also removed and the error returned eventually
func Apply[P data.Point[P], List filterable[P]](dps List, fn func(Ident, P) (P, error)) error {
	var errs error

	mid := dps.Ident()
	dps.Filter(func(dp P) bool {
		id := identity.OfStream(mid, dp)
		next, err := fn(id, dp)
		if err != nil {
			if !errors.Is(err, Drop) {
				errs = errors.Join(errs, err)
			}
			return false
		}

		next.CopyTo(dp)
		return true
	})

	return errs
}

// Drop signals the current item (stream or datapoint) is to be dropped
var Drop = errors.New("stream dropped") //nolint:revive // Drop is a good name for a signal, see fs.SkipAll

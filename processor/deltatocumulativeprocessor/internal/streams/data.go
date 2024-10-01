// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/putil/pslice"
)

func Datapoints[P data.Point[P], List metrics.Data[P]](dps List) func(func(identity.Stream, P) bool) {
	return func(yield func(identity.Stream, P) bool) {
		mid := dps.Ident()
		pslice.All(dps)(func(dp P) bool {
			id := identity.OfStream(mid, dp)
			return yield(id, dp)
		})
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
				err = Error(id, err)
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

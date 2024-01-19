package streams

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
)

func Samples[D data.Point[D]](m metrics.Data[D], fn func(id Ident, dp D)) {
	mid := m.Ident()

	for i := 0; i < m.Len(); i++ {
		dp := m.At(i)
		id := Ident{metric: mid, attrs: pdatautil.MapHash(dp.Attributes())}
		fn(id, dp)
	}
}

func Update[D data.Point[D]](m metrics.Data[D], aggr Aggregator[D]) error {
	var errs error

	Samples(m, func(id Ident, dp D) {
		new, err := aggr.Aggregate(id, dp)
		if err != nil {
			errs = errors.Join(errs, Error(id, err))
			return
		}
		new.CopyTo(dp)
	})

	return errs
}

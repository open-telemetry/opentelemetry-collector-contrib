// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
)

var rdp data.Number
var rid streams.Ident

func BenchmarkApply(b *testing.B) {
	b.Run("fn", func(b *testing.B) {
		dps := generate(b.N)
		b.ResetTimer()

		i := 0
		_ = streams.Apply(dps, func(id identity.Stream, dp data.Number) (data.Number, error) {
			i++
			dp.Add(dp)
			rid = id
			rdp = dp
			if i%2 != 0 {
				return dp, streams.Drop
			}
			return dp, nil
		})
	})

	b.Run("remove-if", func(b *testing.B) {
		dps := generate(b.N)
		mid := dps.id.Metric()
		b.ResetTimer()

		i := 0
		dps.dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
			i++
			ndp := data.Number{NumberDataPoint: dp}
			ndp.Add(ndp)
			rid = identity.OfStream(mid, ndp)
			rdp = ndp
			return i%2 == 0
		})
	})
}

func TestDrop(t *testing.T) {
	const total = 1000
	dps := generate(total)

	want := pmetric.NewNumberDataPointSlice()
	maybe := func(_ streams.Ident, dp data.Number) (data.Number, error) {
		if rand.Intn(2) == 1 {
			dp.NumberDataPoint.CopyTo(want.AppendEmpty())
			return dp, nil
		}
		return dp, streams.Drop
	}

	err := streams.Apply(dps, maybe)
	require.NoError(t, err)

	require.Equal(t, want, dps.dps)
}

func generate(n int) *Data {
	id, ndp := random.Sum().Stream()
	dps := Data{id: id, dps: pmetric.NewNumberDataPointSlice()}
	for i := 0; i < n; i++ {
		dp := dps.dps.AppendEmpty()
		ndp.NumberDataPoint.CopyTo(dp)
		dp.SetIntValue(int64(i))
	}
	return &dps
}

type Data struct {
	id  streams.Ident
	dps pmetric.NumberDataPointSlice
}

func (l Data) At(i int) data.Number {
	return data.Number{NumberDataPoint: l.dps.At(i)}
}

func (l Data) Len() int {
	return l.dps.Len()
}

func (l Data) Ident() metrics.Ident {
	return l.id.Metric()
}

func (l *Data) Filter(expr func(data.Number) bool) {
	l.dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
		return !expr(data.Number{NumberDataPoint: dp})
	})
}

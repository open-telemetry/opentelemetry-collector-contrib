// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
)

var (
	rdp data.Number
	rid streams.Ident
)

func BenchmarkSamples(b *testing.B) {
	b.Run("iterfn", func(b *testing.B) {
		dps := generate(b.N)
		b.ResetTimer()

		streams.Datapoints(dps)(func(id streams.Ident, dp data.Number) bool {
			rdp = dp
			rid = id
			return true
		})
	})

	b.Run("iface", func(b *testing.B) {
		dps := generate(b.N)
		mid := dps.id.Metric()
		b.ResetTimer()

		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			rid = identity.OfStream(mid, dp)
			rdp = dp
		}
	})

	b.Run("loop", func(b *testing.B) {
		dps := generate(b.N)
		mid := dps.id.Metric()
		b.ResetTimer()

		for i := range dps.dps {
			dp := dps.dps[i]
			rid = identity.OfStream(mid, dp)
			rdp = dp
		}
	})
}

func TestAggregate(t *testing.T) {
	const total = 1000
	dps := generate(total)

	// inv aggregator inverts each sample
	inv := aggr(func(_ streams.Ident, n data.Number) (data.Number, error) {
		dp := n.Clone()
		dp.SetIntValue(-dp.IntValue())
		return dp, nil
	})

	err := streams.Apply(dps, inv.Aggregate)
	require.NoError(t, err)

	// check that all samples are inverted
	for i := 0; i < total; i++ {
		require.Equal(t, int64(-i), dps.dps[i].IntValue())
	}
}

func TestDrop(t *testing.T) {
	const total = 1000
	dps := generate(total)

	var want []data.Number
	maybe := aggr(func(_ streams.Ident, dp data.Number) (data.Number, error) {
		if rand.Intn(2) == 1 {
			want = append(want, dp)
			return dp, nil
		}
		return dp, streams.Drop
	})

	err := streams.Apply(dps, maybe.Aggregate)
	require.NoError(t, err)

	require.Equal(t, want, dps.dps)
}

func generate(n int) *Data {
	id, ndp := random.Sum().Stream()
	dps := Data{id: id, dps: make([]data.Number, n)}
	for i := range dps.dps {
		dp := ndp.Clone()
		dp.SetIntValue(int64(i))
		dps.dps[i] = dp
	}
	return &dps
}

type Data struct {
	id  streams.Ident
	dps []data.Number
}

func (l Data) At(i int) data.Number {
	return l.dps[i]
}

func (l Data) Len() int {
	return len(l.dps)
}

func (l Data) Ident() metrics.Ident {
	return l.id.Metric()
}

func (l *Data) Filter(expr func(data.Number) bool) {
	var next []data.Number
	for _, dp := range l.dps {
		if expr(dp) {
			next = append(next, dp)
		}
	}
	l.dps = next
}

type aggr func(streams.Ident, data.Number) (data.Number, error)

func (a aggr) Aggregate(id streams.Ident, dp data.Number) (data.Number, error) {
	return a(id, dp)
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams_test

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
)

var rdp data.Number
var rid streams.Ident

func BenchmarkSamples(b *testing.B) {
	b.Run("iterfn", func(b *testing.B) {
		dps := generate(b.N)
		b.ResetTimer()

		streams.Samples[data.Number](dps)(func(id streams.Ident, dp data.Number) bool {
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
			rid = streams.Identify(mid, dp.Attributes())
			rdp = dp
		}
	})

	b.Run("loop", func(b *testing.B) {
		dps := generate(b.N)
		mid := dps.id.Metric()
		b.ResetTimer()

		for i := range dps.dps {
			dp := dps.dps[i]
			rid = streams.Identify(mid, dp.Attributes())
			rdp = dp
		}
	})
}

func generate(n int) Data {
	id, ndp := random.Sum().Stream()
	dps := Data{id: id, dps: make([]data.Number, n)}
	for i := range dps.dps {
		dp := ndp.Clone()
		dp.SetIntValue(int64(i))
		dps.dps[i] = dp
	}
	return dps
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

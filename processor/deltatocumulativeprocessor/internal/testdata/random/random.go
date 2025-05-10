// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package random

import (
	"math"
	"math/rand"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

type Point[Self any] interface {
	data.Typed[Self]

	SetTimestamp(pcommon.Timestamp)
}

type Metric[P Point[P]] struct {
	metrics.Metric
}

func New[P Point[P]]() Metric[P] {
	metric := pmetric.NewMetric()
	metric.SetName(randStr())
	metric.SetDescription(randStr())
	metric.SetUnit(randStr())
	return Metric[P]{Metric: metrics.From(Resource(), Scope(), metric)}
}

func Sum() Metric[data.Number] {
	metric := New[data.Number]()
	metric.SetEmptySum()
	return metric
}

func Histogram() Metric[data.Histogram] {
	metric := New[data.Histogram]()
	metric.SetEmptyHistogram()
	return metric
}

func Exponential() Metric[data.ExpHistogram] {
	metric := New[data.ExpHistogram]()
	metric.SetEmptyExponentialHistogram()
	return metric
}

func (m Metric[P]) Stream() (streams.Ident, P) {
	var dp P = data.Zero[P]()

	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	for i := 0; i < 10; i++ {
		dp.Attributes().PutStr(randStr(), randStr())
	}
	id := identity.OfStream(m.Ident(), dp)

	return id, dp
}

func Resource() pcommon.Resource {
	return ResourceN(10)
}

func ResourceN(n int) pcommon.Resource {
	res := pcommon.NewResource()
	Attributes(n).MoveTo(res.Attributes())
	return res
}

func Scope() pcommon.InstrumentationScope {
	return ScopeN(3)
}

func ScopeN(n int) pcommon.InstrumentationScope {
	scope := pcommon.NewInstrumentationScope()
	scope.SetName(randStr())
	scope.SetVersion(randStr())
	Attributes(n).MoveTo(scope.Attributes())
	return scope
}

func Attributes(n int) pcommon.Map {
	m := pcommon.NewMap()
	for i := 0; i < n; i++ {
		m.PutStr(randStr(), randStr())
	}
	return m
}

func randStr() string {
	return strconv.FormatInt(randInt(), 16)
}

func randInt() int64 {
	return int64(rand.Intn(math.MaxInt16))
}

func randFloat() float64 {
	return float64(randInt()) / float64(randInt())
}

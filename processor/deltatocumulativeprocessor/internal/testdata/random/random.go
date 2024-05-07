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

func Sum() Metric {
	metric := pmetric.NewMetric()
	metric.SetEmptySum()
	metric.SetName(randStr())
	metric.SetDescription(randStr())
	metric.SetUnit(randStr())
	return Metric{Metric: metrics.From(Resource(), Scope(), metric)}
}

type Metric struct {
	metrics.Metric
}

func (m Metric) Stream() (streams.Ident, data.Number) {
	dp := pmetric.NewNumberDataPoint()
	dp.SetIntValue(int64(randInt()))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	for i := 0; i < 10; i++ {
		dp.Attributes().PutStr(randStr(), randStr())
	}
	id := identity.OfStream(m.Ident(), dp)

	return id, data.Number{NumberDataPoint: dp}
}

func Resource() pcommon.Resource {
	res := pcommon.NewResource()
	for i := 0; i < 10; i++ {
		res.Attributes().PutStr(randStr(), randStr())
	}
	return res
}

func Scope() pcommon.InstrumentationScope {
	scope := pcommon.NewInstrumentationScope()
	scope.SetName(randStr())
	scope.SetVersion(randStr())
	for i := 0; i < 3; i++ {
		scope.Attributes().PutStr(randStr(), randStr())
	}
	return scope
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

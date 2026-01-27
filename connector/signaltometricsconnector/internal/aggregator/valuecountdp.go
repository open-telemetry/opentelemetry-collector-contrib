// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregator // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/aggregator"
import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"
)

// valueCountDP is a wrapper DP to aggregate all datapoints that record
// value and count.
type valueCountDP struct {
	expHistogramDP      *exponentialHistogramDP
	explicitHistogramDP *explicitHistogramDP
}

func newValueCountDP[K any](
	md model.MetricDef[K],
	attrs pcommon.Map,
) *valueCountDP {
	var dp valueCountDP
	if md.Key.Type == pmetric.MetricTypeExponentialHistogram {
		dp.expHistogramDP = newExponentialHistogramDP(
			attrs, md.ExponentialHistogram.MaxSize,
		)
	}
	if md.Key.Type == pmetric.MetricTypeHistogram {
		dp.explicitHistogramDP = newExplicitHistogramDP(
			attrs, md.ExplicitHistogram.Buckets,
		)
	}
	return &dp
}

func (dp *valueCountDP) Aggregate(value float64, count int64) {
	if dp.expHistogramDP != nil {
		dp.expHistogramDP.Aggregate(value, count)
	}
	if dp.explicitHistogramDP != nil {
		dp.explicitHistogramDP.Aggregate(value, count)
	}
}

func (dp *valueCountDP) Copy(
	timestamp time.Time,
	destExpHist pmetric.ExponentialHistogram,
	destExplicitHist pmetric.Histogram,
) {
	if dp.expHistogramDP != nil {
		dp.expHistogramDP.Copy(timestamp, destExpHist.DataPoints().AppendEmpty())
	}
	if dp.explicitHistogramDP != nil {
		dp.explicitHistogramDP.Copy(timestamp, destExplicitHist.DataPoints().AppendEmpty())
	}
}

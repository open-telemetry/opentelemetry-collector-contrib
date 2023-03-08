// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
)

type counterFactory[K any] struct {
	matchExprs  map[string]expr.BoolExpr[K]
	metricInfos map[string]MetricInfo
}

func (f *counterFactory[K]) newCounter() *counter[K] {
	return &counter[K]{
		matchExprs:  f.matchExprs,
		metricInfos: f.metricInfos,
		counts:      make(map[string]uint64, len(f.metricInfos)),
		timestamp:   time.Now(),
	}
}

type counter[K any] struct {
	matchExprs  map[string]expr.BoolExpr[K]
	metricInfos map[string]MetricInfo
	counts      map[string]uint64
	timestamp   time.Time
}

func (c *counter[K]) update(ctx context.Context, tCtx K) error {
	var errors error
	for name := range c.metricInfos {
		// No conditions, so match all.
		if c.matchExprs[name] == nil {
			c.counts[name]++
			continue
		}

		if match, err := c.matchExprs[name].Eval(ctx, tCtx); err != nil {
			errors = multierr.Append(errors, err)
		} else if match {
			c.counts[name]++
		}
	}
	return errors
}

func (c *counter[K]) appendMetricsTo(metricSlice pmetric.MetricSlice) {
	for name, info := range c.metricInfos {
		countMetric := metricSlice.AppendEmpty()
		countMetric.SetName(name)
		countMetric.SetDescription(info.Description)
		sum := countMetric.SetEmptySum()
		// The delta value is always positive, so a value accumulated downstream is monotonic
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := sum.DataPoints().AppendEmpty()
		dp.SetIntValue(int64(c.counts[name]))
		// TODO determine appropriate start time
		dp.SetTimestamp(pcommon.NewTimestampFromTime(c.timestamp))
	}
}

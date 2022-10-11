// Copyright  The OpenTelemetry Authors
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

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"fmt"
	"time"

	"errors"
	"regexp"

	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var reg = regexp.MustCompile(`(\w+)_(\w+)_(\w+)\z`)

func FromTimeSeries(tss []prompb.TimeSeries, settings Settings) (pmetric.Metrics, error) {
	pms := pmetric.NewMetrics()
	for _, ts := range tss {
		empty := pms.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		pm := pmetric.NewMetric()
		metricName, err := finalName(ts.Labels)
		if err != nil {
			return pms, err
		}
		pm.SetName(metricName)
		settings.Logger.Debug("Metric name", zap.String("metric_name", pm.Name()))
		match := reg.FindStringSubmatch(metricName)
		metricsType := ""
		if len(match) > 1 {
			lastSuffixInMetricName := match[len(match)-1]
			if IsValidSuffix(lastSuffixInMetricName) {
				metricsType = lastSuffixInMetricName
				if len(match) > 2 {
					secondSuffixInMetricName := match[len(match)-2]
					if IsValidUnit(secondSuffixInMetricName) {
						pm.SetUnit(secondSuffixInMetricName)
					}
				}
			} else if IsValidUnit(lastSuffixInMetricName) {
				pm.SetUnit(lastSuffixInMetricName)
			}
		}
		settings.Logger.Debug("Metric unit", zap.String("metric name", pm.Name()), zap.String("metric_unit", pm.Unit()))
		for _, s := range ts.Samples {
			ppoint := pmetric.NewNumberDataPoint()
			ppoint.SetDoubleValue(s.Value)
			ppoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, s.Timestamp*int64(time.Millisecond))))
			if ppoint.Timestamp().AsTime().Before(time.Now().Add(-time.Duration(settings.TimeThreshold) * time.Hour)) {
				settings.Logger.Debug("Metric older than the threshold", zap.String("metric name", pm.Name()), zap.Time("metric_timestamp", ppoint.Timestamp().AsTime()))
				continue
			}
			for _, l := range ts.Labels {
				labelName := l.Name
				if l.Name == nameStr {
					labelName = "key_name"
				}
				ppoint.Attributes().PutStr(labelName, l.Value)
			}
			if IsValidCumulativeSuffix(metricsType) {
				pm.SetEmptySum()
				pm.Sum().SetIsMonotonic(true)
				pm.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				ppoint.CopyTo(pm.Sum().DataPoints().AppendEmpty())
			} else {
				pm.SetEmptyGauge()
				ppoint.CopyTo(pm.Gauge().DataPoints().AppendEmpty())
			}
			settings.Logger.Debug("Metric sample",
				zap.String("metric_name", pm.Name()),
				zap.String("metric_unit", pm.Unit()),
				zap.Float64("metric_value", ppoint.DoubleValue()),
				zap.Time("metric_timestamp", ppoint.Timestamp().AsTime()),
				zap.String("metric_labels", fmt.Sprintf("%#v", ppoint.Attributes())),
			)
		}
		pm.MoveTo(empty)
	}
	return pms, nil
}

func finalName(labels []prompb.Label) (ret string, err error) {
	for _, label := range labels {
		if label.Name == nameStr {
			return label.Value, nil
		}
	}
	return "", errors.New("label name not found")
}

// IsValidSuffix - remote write
func IsValidSuffix(suffix string) bool {
	switch suffix {
	case
		"max",
		"sum",
		"count",
		"total":
		return true
	}
	return false
}

// IsValidCumulativeSuffix - remote write
func IsValidCumulativeSuffix(suffix string) bool {
	switch suffix {
	case
		"sum",
		"count",
		"total":
		return true
	}
	return false
}

// IsValidUnit - remote write
func IsValidUnit(unit string) bool {
	switch unit {
	case
		"seconds",
		"bytes":
		return true
	}
	return false
}

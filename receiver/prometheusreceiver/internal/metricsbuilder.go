// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricBuilder struct {
	families             map[string]*metricFamily
	mc                   MetadataCache
	useStartTimeMetric   bool
	startTimeMetricRegex *regexp.Regexp
	startTime            float64
	logger               *zap.Logger
}

// newMetricBuilder creates a MetricBuilder which is allowed to feed all the datapoints from a single prometheus
// scraped page by calling its AddDataPoint function, and turn them into a pmetric.Metrics object.
// by calling its Build function
func newMetricBuilder(mc MetadataCache, useStartTimeMetric bool, startTimeMetricRegex *regexp.Regexp, logger *zap.Logger) *metricBuilder {
	return &metricBuilder{
		families:             map[string]*metricFamily{},
		mc:                   mc,
		logger:               logger,
		useStartTimeMetric:   useStartTimeMetric,
		startTimeMetricRegex: startTimeMetricRegex,
	}
}

func (b *metricBuilder) matchStartTimeMetric(metricName string) bool {
	if b.startTimeMetricRegex != nil {
		return b.startTimeMetricRegex.MatchString(metricName)
	}

	return metricName == startTimeMetricName
}

// AddDataPoint is for feeding prometheus data values in its processing order
func (b *metricBuilder) AddDataPoint(ls labels.Labels, t int64, v float64) error {
	// Any datapoint with duplicate labels MUST be rejected per:
	// * https://github.com/open-telemetry/wg-prometheus/issues/44
	// * https://github.com/open-telemetry/opentelemetry-collector/issues/3407
	// as Prometheus rejects such too as of version 2.16.0, released on 2020-02-13.
	var dupLabels []string
	for i := 0; i < len(ls)-1; i++ {
		if ls[i].Name == ls[i+1].Name {
			dupLabels = append(dupLabels, ls[i].Name)
		}
	}
	if len(dupLabels) != 0 {
		sort.Strings(dupLabels)
		return fmt.Errorf("invalid sample: non-unique label names: %q", dupLabels)
	}

	metricName := ls.Get(model.MetricNameLabel)
	if metricName == "" {
		return errMetricNameNotFound
	}

	// See https://www.prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series
	// up: 1 if the instance is healthy, i.e. reachable, or 0 if the scrape failed.
	// But it can also be a staleNaN, which is inserted when the target goes away.
	if metricName == scrapeUpMetricName && v != 1.0 && !value.IsStaleNaN(v) {
		if v == 0.0 {
			b.logger.Warn("Failed to scrape Prometheus endpoint",
				zap.Int64("scrape_timestamp", t),
				zap.Stringer("target_labels", ls))
		} else {
			b.logger.Warn("The 'up' metric contains invalid value",
				zap.Float64("value", v),
				zap.Int64("scrape_timestamp", t),
				zap.Stringer("target_labels", ls))
		}
	}

	if b.useStartTimeMetric && b.matchStartTimeMetric(metricName) {
		b.startTime = v
	}

	curMF, ok := b.families[metricName]
	if !ok {
		familyName := normalizeMetricName(metricName)
		if mf, ok := b.families[familyName]; ok && mf.includesMetric(metricName) {
			curMF = mf
		} else {
			curMF = newMetricFamily(metricName, b.mc, b.logger)
			b.families[curMF.name] = curMF
		}
	}

	return curMF.Add(metricName, ls, t, v)
}

// getMetrics returns all metrics to the given slice.
// The only error returned by this function is errNoDataToBuild.
func (b *metricBuilder) getMetrics(resource pcommon.Resource) (pmetric.Metrics, error) {
	if len(b.families) == 0 {
		return pmetric.Metrics{}, errNoDataToBuild
	}

	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics().AppendEmpty()
	resource.CopyTo(rms.Resource())
	metrics := rms.ScopeMetrics().AppendEmpty().Metrics()

	for _, mf := range b.families {
		mf.appendMetric(metrics)
	}

	return md, nil
}

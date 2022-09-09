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
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	targetMetricName = "target_info"
	traceIDKey       = "trace_id"
	spanIDKey        = "span_id"
)

type transaction struct {
	isNew                bool
	ctx                  context.Context
	families             map[string]*metricFamily
	mc                   MetadataCache
	startTime            float64
	useStartTimeMetric   bool
	startTimeMetricRegex *regexp.Regexp
	sink                 consumer.Metrics
	externalLabels       labels.Labels
	nodeResource         pcommon.Resource
	logger               *zap.Logger
	job, instance        string
	jobsMap              *JobsMap
	obsrecv              *obsreport.Receiver
}

func newTransaction(
	ctx context.Context,
	jobsMap *JobsMap,
	useStartTimeMetric bool,
	startTimeMetricRegex *regexp.Regexp,
	sink consumer.Metrics,
	externalLabels labels.Labels,
	settings component.ReceiverCreateSettings,
	obsrecv *obsreport.Receiver) *transaction {
	return &transaction{
		ctx:                  ctx,
		families:             make(map[string]*metricFamily),
		isNew:                true,
		sink:                 sink,
		jobsMap:              jobsMap,
		useStartTimeMetric:   useStartTimeMetric,
		startTimeMetricRegex: startTimeMetricRegex,
		externalLabels:       externalLabels,
		logger:               settings.Logger,
		obsrecv:              obsrecv,
	}
}

// Append always returns 0 to disable label caching.
func (t *transaction) Append(ref storage.SeriesRef, labels labels.Labels, atMs int64, value float64) (storage.SeriesRef, error) {
	select {
	case <-t.ctx.Done():
		return 0, errTransactionAborted
	default:
	}

	if len(t.externalLabels) != 0 {
		labels = append(labels, t.externalLabels...)
		sort.Sort(labels)
	}

	if t.isNew {
		if err := t.initTransaction(labels); err != nil {
			return 0, err
		}
	}

	// For the `target_info` metric we need to convert it to resource attributes.
	metricName := labels.Get(model.MetricNameLabel)
	if metricName == targetMetricName {
		return 0, t.AddTargetInfo(labels)
	}

	return 0, t.AddDataPoint(labels, atMs, value)
}

func (t *transaction) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return 0, nil
}

func (t *transaction) matchStartTimeMetric(metricName string) bool {
	if t.startTimeMetricRegex != nil {
		return t.startTimeMetricRegex.MatchString(metricName)
	}

	return metricName == startTimeMetricName
}

// getMetrics returns all metrics to the given slice.
// The only error returned by this function is errNoDataToBuild.
func (t *transaction) getMetrics(resource pcommon.Resource) (pmetric.Metrics, error) {
	if len(t.families) == 0 {
		return pmetric.Metrics{}, errNoDataToBuild
	}

	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics().AppendEmpty()
	resource.CopyTo(rms.Resource())
	metrics := rms.ScopeMetrics().AppendEmpty().Metrics()

	for _, mf := range t.families {
		mf.appendMetric(metrics)
	}

	return md, nil
}

func (t *transaction) initTransaction(labels labels.Labels) error {
	target, ok := scrape.TargetFromContext(t.ctx)
	if !ok {
		return errors.New("unable to find target in context")
	}
	t.mc, ok = scrape.MetricMetadataStoreFromContext(t.ctx)
	if !ok {
		return errors.New("unable to find MetricMetadataStore in context")
	}

	t.job, t.instance = labels.Get(model.JobLabel), labels.Get(model.InstanceLabel)
	if t.job == "" || t.instance == "" {
		return errNoJobInstance
	}
	t.nodeResource = CreateResource(t.job, t.instance, target.DiscoveredLabels())
	t.isNew = false
	return nil
}

func (t *transaction) Commit() error {
	if t.isNew {
		return nil
	}

	ctx := t.obsrecv.StartMetricsOp(t.ctx)

	md, err := t.getMetrics(t.nodeResource)
	if err != nil {
		t.obsrecv.EndMetricsOp(ctx, dataformat, 0, err)
		return err
	}

	if t.useStartTimeMetric {
		if t.startTime == 0.0 {
			err = errNoStartTimeMetrics
			t.obsrecv.EndMetricsOp(ctx, dataformat, 0, err)
			return err
		}
		// Otherwise adjust the startTimestamp for all the metrics.
		t.adjustStartTimestamp(md)
	} else {
		NewMetricsAdjuster(t.jobsMap.get(t.job, t.instance), t.logger).AdjustMetrics(md)
	}

	numPoints := md.DataPointCount()
	if numPoints > 0 {
		if err = t.sink.ConsumeMetrics(ctx, md); err != nil {
			return err
		}
	}

	t.obsrecv.EndMetricsOp(ctx, dataformat, numPoints, nil)
	return nil
}

func (t *transaction) Rollback() error {
	return nil
}

func (t *transaction) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	//TODO: implement this func
	return 0, nil
}

func (t *transaction) AddTargetInfo(labels labels.Labels) error {
	attrs := t.nodeResource.Attributes()

	for _, lbl := range labels {
		if lbl.Name == model.JobLabel || lbl.Name == model.InstanceLabel || lbl.Name == model.MetricNameLabel {
			continue
		}

		attrs.UpsertString(lbl.Name, lbl.Value)
	}

	return nil
}

func pdataTimestampFromFloat64(ts float64) pcommon.Timestamp {
	secs := int64(ts)
	nanos := int64((ts - float64(secs)) * 1e9)
	return pcommon.NewTimestampFromTime(time.Unix(secs, nanos))
}

func (t *transaction) adjustStartTimestamp(metrics pmetric.Metrics) {
	startTimeTs := pdataTimestampFromFloat64(t.startTime)
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)
				switch metric.DataType() {
				case pmetric.MetricDataTypeGauge:
					continue

				case pmetric.MetricDataTypeSum:
					dataPoints := metric.Sum().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						dp.SetStartTimestamp(startTimeTs)
					}

				case pmetric.MetricDataTypeSummary:
					dataPoints := metric.Summary().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						dp.SetStartTimestamp(startTimeTs)
					}

				case pmetric.MetricDataTypeHistogram:
					dataPoints := metric.Histogram().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						dp := dataPoints.At(l)
						dp.SetStartTimestamp(startTimeTs)
					}

				default:
					t.logger.Warn("Unknown metric type", zap.String("type", metric.DataType().String()))
				}
			}
		}
	}
}

// AddDataPoint is for feeding prometheus data values in its processing order
func (t *transaction) AddDataPoint(ls labels.Labels, ts int64, v float64) error {
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
			t.logger.Warn("Failed to scrape Prometheus endpoint",
				zap.Int64("scrape_timestamp", ts),
				zap.Stringer("target_labels", ls))
		} else {
			t.logger.Warn("The 'up' metric contains invalid value",
				zap.Float64("value", v),
				zap.Int64("scrape_timestamp", ts),
				zap.Stringer("target_labels", ls))
		}
	}

	if t.useStartTimeMetric && t.matchStartTimeMetric(metricName) {
		t.startTime = v
	}

	curMF, ok := t.families[metricName]
	if !ok {
		familyName := normalizeMetricName(metricName)
		if mf, ok := t.families[familyName]; ok && mf.includesMetric(metricName) {
			curMF = mf
		} else {
			curMF = newMetricFamily(metricName, t.mc, t.logger)
			t.families[curMF.name] = curMF
		}
	}

	return curMF.Add(metricName, ls, ts, v)
}

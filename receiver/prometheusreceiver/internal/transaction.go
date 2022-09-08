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
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
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
	useStartTimeMetric   bool
	startTimeMetricRegex *regexp.Regexp
	sink                 consumer.Metrics
	externalLabels       labels.Labels
	nodeResource         pcommon.Resource
	logger               *zap.Logger
	metricBuilder        *metricBuilder
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
func (t *transaction) Append(ref storage.SeriesRef, labels labels.Labels, atMs int64, value float64) (pointCount storage.SeriesRef, err error) {
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

	return 0, t.metricBuilder.AddDataPoint(labels, atMs, value)
}

func (t *transaction) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	metricName := l.Get(model.MetricNameLabel)
	familyName := normalizeMetricName(metricName)
	if f, ok := t.metricBuilder.families[familyName]; ok {
		gk := f.getGroupKey(l)
		mg := f.groups[gk]
		_, exists := mg.exemplars[e.Value]
		if exists {
			return 0, nil
		}
		exemplar := pmetric.NewExemplar()
		exemplar.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(e.Ts)))
		exemplar.SetDoubleVal(e.Value)
		for _, lb := range e.Labels {
			switch lb.Name {
			case traceIDKey:
				var tid [16]byte
				b, _ := hex.DecodeString(lb.Value)
				copyToLowerBytes(tid[:], b)
				exemplar.SetTraceID(tid)
			case spanIDKey:
				var sid [8]byte
				b, _ := hex.DecodeString(lb.Value)
				copyToLowerBytes(sid[:], b)
				exemplar.SetSpanID(sid)
			default:
				exemplar.FilteredAttributes().UpsertString(lb.Name, lb.Value)
			}
		}
		t.metricBuilder.families[familyName].groups[gk].exemplars[e.Value] = exemplar
	}
	return 0, nil
}

func copyToLowerBytes(dst []byte, src []byte) {
	for i := 1; i <= len(src); i++ {
		dst[len(dst)-i] = src[len(src)-i]
	}
}

func (t *transaction) initTransaction(labels labels.Labels) error {
	target, ok := scrape.TargetFromContext(t.ctx)
	if !ok {
		return errors.New("unable to find target in context")
	}
	metaStore, ok := scrape.MetricMetadataStoreFromContext(t.ctx)
	if !ok {
		return errors.New("unable to find MetricMetadataStore in context")
	}

	job, instance := labels.Get(model.JobLabel), labels.Get(model.InstanceLabel)
	if job == "" || instance == "" {
		return errNoJobInstance
	}
	if t.jobsMap != nil {
		t.job = job
		t.instance = instance
	}
	t.nodeResource = CreateResource(job, instance, target.DiscoveredLabels())
	t.metricBuilder = newMetricBuilder(metaStore, t.useStartTimeMetric, t.startTimeMetricRegex, t.logger)
	t.isNew = false
	return nil
}

func (t *transaction) Commit() error {
	if t.isNew {
		return nil
	}

	ctx := t.obsrecv.StartMetricsOp(t.ctx)

	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics().AppendEmpty()
	t.nodeResource.CopyTo(rms.Resource())
	metrics := rms.ScopeMetrics().AppendEmpty().Metrics()

	err := t.metricBuilder.appendMetrics(metrics)
	if err != nil {
		t.obsrecv.EndMetricsOp(ctx, dataformat, 0, err)
		return err
	}

	if t.useStartTimeMetric {
		if t.metricBuilder.startTime == 0.0 {
			err = errNoStartTimeMetrics
			t.obsrecv.EndMetricsOp(ctx, dataformat, 0, err)
			return err
		}
		// Otherwise adjust the startTimestamp for all the metrics.
		t.adjustStartTimestamp(metrics)
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

func (t *transaction) adjustStartTimestamp(metricsL pmetric.MetricSlice) {
	startTimeTs := pdataTimestampFromFloat64(t.metricBuilder.startTime)
	for i := 0; i < metricsL.Len(); i++ {
		metric := metricsL.At(i)
		switch metric.DataType() {
		case pmetric.MetricDataTypeGauge:
			continue

		case pmetric.MetricDataTypeSum:
			dataPoints := metric.Sum().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dp := dataPoints.At(j)
				dp.SetStartTimestamp(startTimeTs)
			}

		case pmetric.MetricDataTypeSummary:
			dataPoints := metric.Summary().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dp := dataPoints.At(j)
				dp.SetStartTimestamp(startTimeTs)
			}

		case pmetric.MetricDataTypeHistogram:
			dataPoints := metric.Histogram().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dp := dataPoints.At(j)
				dp.SetStartTimestamp(startTimeTs)
			}

		default:
			t.logger.Warn("Unknown metric type", zap.String("type", metric.DataType().String()))
		}
	}
}

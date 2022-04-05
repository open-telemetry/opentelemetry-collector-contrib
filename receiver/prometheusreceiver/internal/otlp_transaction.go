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
	"sync/atomic"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

type transactionPdata struct {
	id                   int64
	isNew                bool
	ctx                  context.Context
	useStartTimeMetric   bool
	startTimeMetricRegex string
	sink                 consumer.Metrics
	externalLabels       labels.Labels
	nodeResource         *pdata.Resource
	logger               *zap.Logger
	receiverID           config.ComponentID
	metricBuilder        *metricBuilderPdata
	job, instance        string
	jobsMap              *JobsMapPdata
	obsrecv              *obsreport.Receiver
	startTimeMs          int64
}

type txConfig struct {
	jobsMap              *JobsMapPdata
	useStartTimeMetric   bool
	startTimeMetricRegex string
	receiverID           config.ComponentID
	sink                 consumer.Metrics
	externalLabels       labels.Labels
	settings             component.ReceiverCreateSettings
}

func newTransactionPdata(ctx context.Context, txc *txConfig) *transactionPdata {
	return &transactionPdata{
		id:                   atomic.AddInt64(&idSeq, 1),
		ctx:                  ctx,
		isNew:                true,
		sink:                 txc.sink,
		jobsMap:              txc.jobsMap,
		useStartTimeMetric:   txc.useStartTimeMetric,
		startTimeMetricRegex: txc.startTimeMetricRegex,
		receiverID:           txc.receiverID,
		externalLabels:       txc.externalLabels,
		logger:               txc.settings.Logger,
		obsrecv:              obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: txc.receiverID, Transport: transport, ReceiverCreateSettings: txc.settings}),
	}
}

// Append always returns 0 to disable label caching.
func (t *transactionPdata) Append(ref storage.SeriesRef, labels labels.Labels, atMs int64, value float64) (pointCount storage.SeriesRef, err error) {
	select {
	case <-t.ctx.Done():
		return 0, errTransactionAborted
	default:
	}

	if len(t.externalLabels) != 0 {
		labels = append(labels, t.externalLabels...)
	}

	if t.isNew {
		t.startTimeMs = atMs
		if err := t.initTransaction(labels); err != nil {
			return 0, err
		}
	}

	return 0, t.metricBuilder.AddDataPoint(labels, atMs, value)
}

func (t *transactionPdata) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return 0, nil
}

func (t *transactionPdata) initTransaction(labels labels.Labels) error {
	metadataCache, err := getMetadataCache(t.ctx)
	if err != nil {
		return err
	}

	job, instance := labels.Get(model.JobLabel), labels.Get(model.InstanceLabel)
	if job == "" || instance == "" {
		return errNoJobInstance
	}
	if t.jobsMap != nil {
		t.job = job
		t.instance = instance
	}
	t.nodeResource = CreateNodeAndResourcePdata(job, instance, metadataCache.SharedLabels().Get(model.SchemeLabel))
	t.metricBuilder = newMetricBuilderPdata(metadataCache, t.useStartTimeMetric, t.startTimeMetricRegex, t.logger, t.startTimeMs)
	t.isNew = false
	return nil
}

func (t *transactionPdata) Commit() error {
	if t.isNew {
		return nil
	}

	t.startTimeMs = -1

	ctx := t.obsrecv.StartMetricsOp(t.ctx)
	metricsL, numPoints, _, err := t.metricBuilder.Build()
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
		t.adjustStartTimestampPdata(metricsL)
	} else {
		// TODO: Derive numPoints in this case.
		_ = NewMetricsAdjusterPdata(t.jobsMap.get(t.job, t.instance), t.logger).AdjustMetricSlice(metricsL)
	}

	if metricsL.Len() > 0 {
		metrics := t.metricSliceToMetrics(metricsL)
		t.sink.ConsumeMetrics(ctx, *metrics)
	}

	t.obsrecv.EndMetricsOp(ctx, dataformat, numPoints, nil)
	return nil
}

func (t *transactionPdata) Rollback() error {
	t.startTimeMs = -1
	return nil
}

func pdataTimestampFromFloat64(ts float64) pdata.Timestamp {
	secs := int64(ts)
	nanos := int64((ts - float64(secs)) * 1e9)
	return pdata.NewTimestampFromTime(time.Unix(secs, nanos))
}

func (t transactionPdata) adjustStartTimestampPdata(metricsL *pdata.MetricSlice) {
	startTimeTs := pdataTimestampFromFloat64(t.metricBuilder.startTime)
	for i := 0; i < metricsL.Len(); i++ {
		metric := metricsL.At(i)
		switch metric.DataType() {
		case pdata.MetricDataTypeGauge:
			continue

		case pdata.MetricDataTypeSum:
			dataPoints := metric.Sum().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dp := dataPoints.At(j)
				dp.SetStartTimestamp(startTimeTs)
			}

		case pdata.MetricDataTypeSummary:
			dataPoints := metric.Summary().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dp := dataPoints.At(j)
				dp.SetStartTimestamp(startTimeTs)
			}

		case pdata.MetricDataTypeHistogram:
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

func (t *transactionPdata) metricSliceToMetrics(metricsL *pdata.MetricSlice) *pdata.Metrics {
	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics().AppendEmpty()
	ilm := rms.ScopeMetrics().AppendEmpty()
	metricsL.CopyTo(ilm.Metrics())
	t.nodeResource.CopyTo(rms.Resource())
	return &metrics
}

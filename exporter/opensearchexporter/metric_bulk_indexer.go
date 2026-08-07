// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v4/opensearchutil"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type metricBulkIndexer struct {
	bulkAction  string
	pipeline    string
	model       mappingModel
	errs        []error
	bulkIndexer opensearchutil.BulkIndexer
}

func newMetricBulkIndexer(bulkAction string, model mappingModel, pipeline string) *metricBulkIndexer {
	return &metricBulkIndexer{bulkAction: bulkAction, pipeline: pipeline, model: model, errs: nil, bulkIndexer: nil}
}

func (mbi *metricBulkIndexer) start(client *opensearchapi.Client) error {
	var startErr error
	mbi.bulkIndexer, startErr = newOpenSearchBulkIndexer(client, mbi.onIndexerError, mbi.pipeline)
	return startErr
}

func (mbi *metricBulkIndexer) joinedError() error {
	return errors.Join(mbi.errs...)
}

func (mbi *metricBulkIndexer) close(ctx context.Context) {
	closeErr := mbi.bulkIndexer.Close(ctx)
	if closeErr != nil {
		mbi.errs = append(mbi.errs, closeErr)
	}
}

func (mbi *metricBulkIndexer) onIndexerError(_ context.Context, indexerErr error) {
	if indexerErr != nil {
		mbi.appendPermanentError(consumererror.NewPermanent(indexerErr))
	}
}

func (mbi *metricBulkIndexer) appendPermanentError(e error) {
	mbi.errs = append(mbi.errs, consumererror.NewPermanent(e))
}

func (mbi *metricBulkIndexer) appendRetryMetricError(err error, metrics pmetric.Metrics) {
	mbi.errs = append(mbi.errs, consumererror.NewMetrics(err, metrics))
}

func (mbi *metricBulkIndexer) submit(ctx context.Context, md pmetric.Metrics, ir *indexResolver, cfg *Config, timestamp time.Time) {
	keys := ir.extractPlaceholderKeys(cfg.MetricsIndex)
	timeSuffix := ir.calculateTimeSuffix(cfg.MetricsIndexTimeFormat, timestamp)
	resourceMetrics := md.ResourceMetrics()

	for i := 0; i < resourceMetrics.Len(); i++ {
		rm := resourceMetrics.At(i)
		resource := rm.Resource()
		resourceAttrs := ir.collectResourceAttributes(resource, keys)
		scopeMetrics := rm.ScopeMetrics()

		for j := 0; j < scopeMetrics.Len(); j++ {
			sm := scopeMetrics.At(j)
			scopeAttrs := ir.collectScopeAttributes(sm.Scope(), keys)
			metrics := sm.Metrics()

			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				for _, dp := range metricDataPoints(metric) {
					indexName := ir.resolveIndexName(cfg.MetricsIndex, cfg.MetricsIndexFallback, dp.Attributes(), keys, scopeAttrs, resourceAttrs, timeSuffix)
					mbi.processItem(ctx, indexName, resource, rm.SchemaUrl(), sm.Scope(), sm.SchemaUrl(), metric, dp)
				}
			}
		}
	}
}

// metricDataPoints returns the data points of a metric regardless of its type.
func metricDataPoints(metric pmetric.Metric) []metricDataPoint {
	var out []metricDataPoint
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i))
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i))
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i))
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := metric.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i))
		}
	case pmetric.MetricTypeSummary:
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			out = append(out, dps.At(i))
		}
	}
	return out
}

func (mbi *metricBulkIndexer) processItem(
	ctx context.Context,
	indexName string,
	resource pcommon.Resource,
	resourceSchemaURL string,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	metric pmetric.Metric,
	dp metricDataPoint,
) {
	payload, err := mbi.model.encodeMetric(resource, scope, scopeSchemaURL, metric, dp)
	if err != nil {
		mbi.appendPermanentError(err)
	} else {
		ItemFailureHandler := func(_ context.Context, _ opensearchutil.BulkIndexerItem, resp opensearchapi.BulkRespItem, itemErr error) {
			// Setup error handler. The handler handles the per item response status based on the
			// selective ACKing in the bulk response.
			mbi.processItemFailure(resp, itemErr, makeMetric(resource, resourceSchemaURL, scope, scopeSchemaURL, metric, dp))
		}
		bi := mbi.newBulkIndexerItem(payload, indexName)
		bi.OnFailure = ItemFailureHandler
		err = mbi.bulkIndexer.Add(ctx, bi)
		if err != nil {
			mbi.appendRetryMetricError(err, makeMetric(resource, resourceSchemaURL, scope, scopeSchemaURL, metric, dp))
		}
	}
}

// makeMetric builds a pmetric.Metrics holding a single data point of the given
// metric, preserving the metric identity and type-specific properties.
func makeMetric(
	resource pcommon.Resource,
	resourceSchemaURL string,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	metric pmetric.Metric,
	dp metricDataPoint,
) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	resource.CopyTo(rm.Resource())
	rm.SetSchemaUrl(resourceSchemaURL)
	sm := rm.ScopeMetrics().AppendEmpty()

	sm.SetSchemaUrl(scopeSchemaURL)
	scope.CopyTo(sm.Scope())
	m := sm.Metrics().AppendEmpty()
	m.SetName(metric.Name())
	m.SetDescription(metric.Description())
	m.SetUnit(metric.Unit())

	switch dp := dp.(type) {
	case pmetric.NumberDataPoint:
		if metric.Type() == pmetric.MetricTypeSum {
			sum := m.SetEmptySum()
			sum.SetAggregationTemporality(metric.Sum().AggregationTemporality())
			sum.SetIsMonotonic(metric.Sum().IsMonotonic())
			dp.CopyTo(sum.DataPoints().AppendEmpty())
		} else {
			dp.CopyTo(m.SetEmptyGauge().DataPoints().AppendEmpty())
		}
	case pmetric.HistogramDataPoint:
		histogram := m.SetEmptyHistogram()
		histogram.SetAggregationTemporality(metric.Histogram().AggregationTemporality())
		dp.CopyTo(histogram.DataPoints().AppendEmpty())
	case pmetric.ExponentialHistogramDataPoint:
		histogram := m.SetEmptyExponentialHistogram()
		histogram.SetAggregationTemporality(metric.ExponentialHistogram().AggregationTemporality())
		dp.CopyTo(histogram.DataPoints().AppendEmpty())
	case pmetric.SummaryDataPoint:
		dp.CopyTo(m.SetEmptySummary().DataPoints().AppendEmpty())
	}

	return metrics
}

func (mbi *metricBulkIndexer) processItemFailure(resp opensearchapi.BulkRespItem, itemErr error, metrics pmetric.Metrics) {
	switch {
	case shouldRetryEvent(resp.Status):
		// Recoverable OpenSearch error
		mbi.appendRetryMetricError(responseAsError(resp), metrics)
	case resp.Status != 0 && itemErr == nil:
		// Non-recoverable OpenSearch error while indexing document
		mbi.appendPermanentError(responseAsError(resp))
	default:
		// Encoding error. We didn't even attempt to send the event
		mbi.appendPermanentError(itemErr)
	}
}

func (mbi *metricBulkIndexer) newBulkIndexerItem(document []byte, indexName string) opensearchutil.BulkIndexerItem {
	body := bytes.NewReader(document)
	item := opensearchutil.BulkIndexerItem{Action: mbi.bulkAction, Index: indexName, Body: body}
	return item
}

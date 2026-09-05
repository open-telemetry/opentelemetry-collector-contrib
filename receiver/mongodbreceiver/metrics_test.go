// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

// findMetric returns the emitted metric with the given name, failing the test if absent.
func findMetric(t *testing.T, md pmetric.Metrics, name string) pmetric.Metric {
	t.Helper()
	require.Equal(t, 1, md.ResourceMetrics().Len())
	sms := md.ResourceMetrics().At(0).ScopeMetrics()
	require.Equal(t, 1, sms.Len())
	ms := sms.At(0).Metrics()
	for i := 0; i < ms.Len(); i++ {
		if ms.At(i).Name() == name {
			return ms.At(i)
		}
	}
	t.Fatalf("metric %q not found among emitted metrics", name)
	return pmetric.Metric{}
}

// sumIntByAttr maps an attribute value to its int data-point value for a Sum metric.
func sumIntByAttr(t *testing.T, m pmetric.Metric, attrKey string) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	dps := m.Sum().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		v, ok := dp.Attributes().Get(attrKey)
		require.True(t, ok, "data point missing attribute %q", attrKey)
		out[v.Str()] = dp.IntValue()
	}
	return out
}

// newWTScraper builds a scraper with all five WiredTiger metrics enabled.
func newWTScraper(t *testing.T) *mongodbScraper {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsBuilderConfig.Metrics.MongodbWtLogWrite.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.MongodbWtLogOperationCount.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.MongodbWtLogSyncTime.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.MongodbWtFsyncCount.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.MongodbWtConcurrentTransactionTicketInUse.Enabled = true
	return newMongodbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
}

func TestRecordWTLogWrite(t *testing.T) {
	s := newWTScraper(t)
	doc, err := loadAdminStatusAsMap()
	require.NoError(t, err)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordWTLogWrite(now, doc, errs)
	require.NoError(t, errs.Combine())

	m := findMetric(t, s.mb.Emit(), "mongodb.wt.log.write")
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	require.Equal(t, 1, m.Sum().DataPoints().Len())
	require.Equal(t, int64(1048576000), m.Sum().DataPoints().At(0).IntValue())
}

func TestRecordWTLogOperations(t *testing.T) {
	s := newWTScraper(t)
	doc, err := loadAdminStatusAsMap()
	require.NoError(t, err)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordWTLogOperations(now, doc, errs)
	require.NoError(t, errs.Combine())

	m := findMetric(t, s.mb.Emit(), "mongodb.wt.log.operation.count")
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	byType := sumIntByAttr(t, m, "mongodb.wt.log.operation.type")
	require.Equal(t, map[string]int64{
		"write": 12345,
		"sync":  2345,
		"flush": 8100,
	}, byType)
}

func TestRecordWTLogSyncTime(t *testing.T) {
	s := newWTScraper(t)
	doc, err := loadAdminStatusAsMap()
	require.NoError(t, err)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordWTLogSyncTime(now, doc, errs)
	require.NoError(t, errs.Combine())

	m := findMetric(t, s.mb.Emit(), "mongodb.wt.log.sync.time")
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	require.Equal(t, 1, m.Sum().DataPoints().Len())
	// Source field is 5_000_000 microseconds; emitted as 5.0 seconds.
	require.InDelta(t, 5.0, m.Sum().DataPoints().At(0).DoubleValue(), 1e-9)
}

func TestRecordWTFsyncCount(t *testing.T) {
	s := newWTScraper(t)
	doc, err := loadAdminStatusAsMap()
	require.NoError(t, err)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordWTFsyncCount(now, doc, errs)
	require.NoError(t, errs.Combine())

	m := findMetric(t, s.mb.Emit(), "mongodb.wt.fsync.count")
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	require.Equal(t, 1, m.Sum().DataPoints().Len())
	require.Equal(t, int64(42), m.Sum().DataPoints().At(0).IntValue())
}

// TestRecordWTConcurrentTransactionsOutLegacy exercises the pre-8.0 path that reads
// serverStatus.wiredTiger.concurrentTransactions.{read,write}.out (present in admin.json).
func TestRecordWTConcurrentTransactionsOutLegacy(t *testing.T) {
	s := newWTScraper(t)
	doc, err := loadAdminStatusAsMap()
	require.NoError(t, err)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordWTConcurrentTransactionsOut(now, doc, errs)
	require.NoError(t, errs.Combine())

	m := findMetric(t, s.mb.Emit(), "mongodb.wt.concurrent_transaction.ticket.in_use")
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	byDir := sumIntByAttr(t, m, "mongodb.wt.concurrent_transaction.ticket.type")
	require.Equal(t, map[string]int64{"read": 3, "write": 1}, byDir)
}

// TestRecordWTConcurrentTransactionsOut80 exercises the MongoDB 8.0+ path where the field
// was renamed to serverStatus.queues.execution.{read,write}.out. The receiver must read the
// queues.execution path preferentially, even when the legacy WiredTiger path is also present.
func TestRecordWTConcurrentTransactionsOut80(t *testing.T) {
	// Document with the 8.0 queues.execution path AND a conflicting legacy path, to assert
	// the 8.0 path wins.
	doc := bson.M{
		"queues": bson.M{
			"execution": bson.M{
				"read":  bson.M{"out": int64(7)},
				"write": bson.M{"out": int64(4)},
			},
		},
		"storageEngine": bson.M{"name": "wiredTiger"},
		"wiredTiger": bson.M{
			"concurrentTransactions": bson.M{
				"read":  bson.M{"out": int64(999)},
				"write": bson.M{"out": int64(999)},
			},
		},
	}
	s := newWTScraper(t)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordWTConcurrentTransactionsOut(now, doc, errs)
	require.NoError(t, errs.Combine())

	m := findMetric(t, s.mb.Emit(), "mongodb.wt.concurrent_transaction.ticket.in_use")
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	byDir := sumIntByAttr(t, m, "mongodb.wt.concurrent_transaction.ticket.type")
	require.Equal(t, map[string]int64{"read": 7, "write": 4}, byDir)
}

// TestRecordWTConcurrentTransactionsAttributeDisabled verifies the metric is a non-monotonic
// Sum, not a Gauge: when the read/write attribute is disabled the two data points collapse by
// addition (7 + 4 = 11), not by averaging (which a Gauge would do, yielding 5).
func TestRecordWTConcurrentTransactionsAttributeDisabled(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsBuilderConfig.Metrics.MongodbWtConcurrentTransactionTicketInUse.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.MongodbWtConcurrentTransactionTicketInUse.EnabledAttributes = []metadata.MongodbWtConcurrentTransactionTicketInUseMetricAttributeKey{}
	s := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	doc := bson.M{
		"storageEngine": bson.M{"name": "wiredTiger"},
		"wiredTiger": bson.M{
			"concurrentTransactions": bson.M{
				"read":  bson.M{"out": int64(7)},
				"write": bson.M{"out": int64(4)},
			},
		},
	}
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordWTConcurrentTransactionsOut(now, doc, errs)
	require.NoError(t, errs.Combine())

	m := findMetric(t, s.mb.Emit(), "mongodb.wt.concurrent_transaction.ticket.in_use")
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	require.Equal(t, 1, m.Sum().DataPoints().Len())
	require.Equal(t, int64(11), m.Sum().DataPoints().At(0).IntValue())
}

// TestRecordWTConcurrentTransactionsNonWiredTiger80 asserts that on MongoDB 8.0+ the
// queues.execution path is still gated by the storage engine: a non-WiredTiger engine
// (e.g. inMemory) that exposes queues.execution must not emit the metric.
func TestRecordWTConcurrentTransactionsNonWiredTiger80(t *testing.T) {
	doc := bson.M{
		"queues": bson.M{
			"execution": bson.M{
				"read":  bson.M{"out": int64(7)},
				"write": bson.M{"out": int64(4)},
			},
		},
		"storageEngine": bson.M{"name": "inMemory"},
	}
	s := newWTScraper(t)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordWTConcurrentTransactionsOut(now, doc, errs)
	require.NoError(t, errs.Combine())

	md := s.mb.Emit()
	require.Equal(t, 0, md.ResourceMetrics().Len())
}

// TestRecordWTMetricsNonWiredTiger asserts the log/fsync metrics emit nothing when the
// storage engine is not WiredTiger.
func TestRecordWTMetricsNonWiredTiger(t *testing.T) {
	doc := bson.M{"storageEngine": bson.M{"name": "inMemory"}}
	s := newWTScraper(t)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordWTLogWrite(now, doc, errs)
	s.recordWTLogOperations(now, doc, errs)
	s.recordWTLogSyncTime(now, doc, errs)
	s.recordWTFsyncCount(now, doc, errs)
	s.recordWTConcurrentTransactionsOut(now, doc, errs)
	require.NoError(t, errs.Combine())

	md := s.mb.Emit()
	require.Equal(t, 0, md.ResourceMetrics().Len())
}

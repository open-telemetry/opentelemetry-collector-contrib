// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

const (
	oplogOldestSecond = 1756425600
	oplogNewestSecond = 1756512000
	oplogWindow       = float64(oplogNewestSecond - oplogOldestSecond)
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

// metricNames returns the names of every emitted metric.
func metricNames(md pmetric.Metrics) []string {
	var names []string
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		sms := md.ResourceMetrics().At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				names = append(names, ms.At(k).Name())
			}
		}
	}
	return names
}

// newReplicaSetScraper builds a scraper with every replica set and oplog metric enabled.
func newReplicaSetScraper(t *testing.T) *mongodbScraper {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	metrics := &cfg.MetricsBuilderConfig.Metrics
	metrics.MongodbOplogUsage.Enabled = true
	metrics.MongodbOplogLimit.Enabled = true
	metrics.MongodbOplogWindow.Enabled = true
	metrics.MongodbReplicaSetHeadroom.Enabled = true
	metrics.MongodbReplicaSetLag.Enabled = true
	metrics.MongodbReplicaSetMemberCount.Enabled = true
	metrics.MongodbReplicaSetMemberStatus.Enabled = true
	return newMongodbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
}

func loadReplicaSetMembers(t *testing.T) bson.A {
	t.Helper()
	status, err := loadTestFileAsMap("./testdata/replSetGetStatus.json")
	require.NoError(t, err)
	members, ok := status[replicaSetMembersKey].(bson.A)
	require.True(t, ok)
	return members
}

func TestRecordOplogUsage(t *testing.T) {
	s := newReplicaSetScraper(t)
	doc, err := loadTestFileAsMap("./testdata/oplogStats.json")
	require.NoError(t, err)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordOplogUsage(now, doc, errs)
	s.recordOplogLimit(now, doc, errs)
	require.NoError(t, errs.Combine())

	emitted := s.mb.Emit()
	usage := findMetric(t, emitted, "mongodb.oplog.usage")
	require.Equal(t, pmetric.MetricTypeSum, usage.Type())
	require.False(t, usage.Sum().IsMonotonic())
	require.Equal(t, 1, usage.Sum().DataPoints().Len())
	require.Equal(t, int64(1073741824), usage.Sum().DataPoints().At(0).IntValue())

	limit := findMetric(t, emitted, "mongodb.oplog.limit")
	require.Equal(t, 1, limit.Sum().DataPoints().Len())
	require.Equal(t, int64(2147483648), limit.Sum().DataPoints().At(0).IntValue())
}

func TestRecordOplogWindow(t *testing.T) {
	s := newReplicaSetScraper(t)
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordOplogWindow(now, bson.Timestamp{T: oplogOldestSecond, I: 1}, bson.Timestamp{T: oplogNewestSecond, I: 4})

	m := findMetric(t, s.mb.Emit(), "mongodb.oplog.window")
	require.Equal(t, pmetric.MetricTypeGauge, m.Type())
	require.Equal(t, 1, m.Gauge().DataPoints().Len())
	require.InDelta(t, oplogWindow, m.Gauge().DataPoints().At(0).DoubleValue(), 0)
}

func TestRecordReplicaSetMemberCount(t *testing.T) {
	s := newReplicaSetScraper(t)
	members := loadReplicaSetMembers(t)
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordReplicaSetMemberCount(now, members)

	m := findMetric(t, s.mb.Emit(), "mongodb.replica_set.member.count")
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	require.False(t, m.Sum().IsMonotonic())

	dps := m.Sum().DataPoints()
	// Every known state is reported, so a state no member is in reads as zero.
	require.Equal(t, len(replicaSetMemberStates), dps.Len())
	counts := map[string]int64{}
	for i := 0; i < dps.Len(); i++ {
		state, ok := dps.At(i).Attributes().Get("mongodb.replica_set.member.state")
		require.True(t, ok)
		counts[state.Str()] = dps.At(i).IntValue()
	}
	require.Equal(t, int64(1), counts[metadata.AttributeMongodbReplicaSetMemberStatePrimary.String()])
	require.Equal(t, int64(1), counts[metadata.AttributeMongodbReplicaSetMemberStateSecondary.String()])
	require.Equal(t, int64(1), counts[metadata.AttributeMongodbReplicaSetMemberStateArbiter.String()])
	require.Equal(t, int64(0), counts[metadata.AttributeMongodbReplicaSetMemberStateDown.String()])
}

func TestRecordReplicaSetMemberCountUnreachableMember(t *testing.T) {
	// MongoDB renders the state of a member it cannot reach as "(not reachable/healthy)" rather
	// than "DOWN", so the member state must be read from the numeric field.
	s := newReplicaSetScraper(t)
	members := bson.A{
		bson.D{
			bson.E{Key: replicaSetMemberNameKey, Value: "mongo-0:27017"},
			bson.E{Key: replicaSetMemberStateKey, Value: int32(1)},
			bson.E{Key: "stateStr", Value: "PRIMARY"},
		},
		bson.D{
			bson.E{Key: replicaSetMemberNameKey, Value: "mongo-1:27017"},
			bson.E{Key: replicaSetMemberStateKey, Value: int32(8)},
			bson.E{Key: "stateStr", Value: "(not reachable/healthy)"},
		},
	}

	s.recordReplicaSetMemberCount(pcommon.NewTimestampFromTime(time.Now()), members)

	m := findMetric(t, s.mb.Emit(), "mongodb.replica_set.member.count")
	dps := m.Sum().DataPoints()
	counts := map[string]int64{}
	total := int64(0)
	for i := 0; i < dps.Len(); i++ {
		state, ok := dps.At(i).Attributes().Get("mongodb.replica_set.member.state")
		require.True(t, ok)
		counts[state.Str()] = dps.At(i).IntValue()
		total += dps.At(i).IntValue()
	}
	require.Equal(t, int64(1), counts[metadata.AttributeMongodbReplicaSetMemberStateDown.String()])
	require.Equal(t, int64(1), counts[metadata.AttributeMongodbReplicaSetMemberStatePrimary.String()])
	// Every member is accounted for; none is dropped by an unrecognized state rendering.
	require.Equal(t, int64(len(members)), total)
}

func TestRecordReplicaSetMemberStatus(t *testing.T) {
	s := newReplicaSetScraper(t)
	members := loadReplicaSetMembers(t)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordReplicaSetMemberStatus(now, members, errs)
	require.NoError(t, errs.Combine())

	m := findMetric(t, s.mb.Emit(), "mongodb.replica_set.member.status")
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	require.False(t, m.Sum().IsMonotonic())

	// A timeseries is produced for every state: 1 for the instance's current state, 0 for the rest.
	dps := m.Sum().DataPoints()
	require.Equal(t, len(replicaSetMemberStates), dps.Len())
	values := map[string]int64{}
	for i := 0; i < dps.Len(); i++ {
		state, ok := dps.At(i).Attributes().Get("mongodb.replica_set.member.state")
		require.True(t, ok)
		values[state.Str()] = dps.At(i).IntValue()
	}
	require.Equal(t, int64(1), values[metadata.AttributeMongodbReplicaSetMemberStatePrimary.String()])
	require.Equal(t, int64(0), values[metadata.AttributeMongodbReplicaSetMemberStateSecondary.String()])
	require.Equal(t, int64(0), values[metadata.AttributeMongodbReplicaSetMemberStateDown.String()])
	var total int64
	for _, v := range values {
		total += v
	}
	require.Equal(t, int64(1), total, "exactly one state may be set to 1")
}

func TestRecordReplicaSetMemberStatusWithoutSelf(t *testing.T) {
	s := newReplicaSetScraper(t)
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordReplicaSetMemberStatus(now, bson.A{bson.D{bson.E{Key: replicaSetMemberStateKey, Value: int32(1)}}}, errs)

	require.Error(t, errs.Combine())
	require.Empty(t, metricNames(s.mb.Emit()))
}

func TestRecordReplicaSetLag(t *testing.T) {
	s := newReplicaSetScraper(t)
	members := loadReplicaSetMembers(t)
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordReplicaSetLag(now, replicaSetLags(members))

	m := findMetric(t, s.mb.Emit(), "mongodb.replica_set.lag")
	require.Equal(t, pmetric.MetricTypeGauge, m.Type())
	dps := m.Gauge().DataPoints()
	require.Equal(t, 2, dps.Len())

	lags := map[string]float64{}
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		member, ok := dp.Attributes().Get("mongodb.replica_set.member.name")
		require.True(t, ok)
		lagType, ok := dp.Attributes().Get("mongodb.replica_set.lag.type")
		require.True(t, ok)
		lags[member.Str()+"/"+lagType.Str()] = dp.DoubleValue()
	}
	require.Equal(t, map[string]float64{
		"mongo-1:27017/" + metadata.AttributeMongodbReplicaSetLagTypeApplied.String(): 1.5,
		"mongo-1:27017/" + metadata.AttributeMongodbReplicaSetLagTypeDurable.String(): 3,
	}, lags)
}

func TestRecordReplicaSetLagWithoutDurableTime(t *testing.T) {
	s := newReplicaSetScraper(t)
	members := bson.A{
		bson.D{
			bson.E{Key: replicaSetMemberNameKey, Value: "mongo-0:27017"},
			bson.E{Key: replicaSetMemberStateKey, Value: int32(1)},
			bson.E{Key: replicaSetMemberOptimeKey, Value: bson.DateTime(1756512000000)},
		},
		bson.D{
			bson.E{Key: replicaSetMemberNameKey, Value: "mongo-1:27017"},
			bson.E{Key: replicaSetMemberStateKey, Value: int32(2)},
			bson.E{Key: replicaSetMemberOptimeKey, Value: bson.DateTime(1756511998500)},
		},
	}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordReplicaSetLag(now, replicaSetLags(members))

	m := findMetric(t, s.mb.Emit(), "mongodb.replica_set.lag")
	require.Equal(t, 1, m.Gauge().DataPoints().Len())
	lagType, ok := m.Gauge().DataPoints().At(0).Attributes().Get("mongodb.replica_set.lag.type")
	require.True(t, ok)
	require.Equal(t, metadata.AttributeMongodbReplicaSetLagTypeApplied.String(), lagType.Str())
}

func TestRecordReplicaSetHeadroom(t *testing.T) {
	s := newReplicaSetScraper(t)
	members := loadReplicaSetMembers(t)
	now := pcommon.NewTimestampFromTime(time.Now())

	s.recordReplicaSetHeadroom(now, replicaSetLags(members), oplogWindow)

	m := findMetric(t, s.mb.Emit(), "mongodb.replica_set.headroom")
	require.Equal(t, pmetric.MetricTypeGauge, m.Type())
	require.Equal(t, 1, m.Gauge().DataPoints().Len())
	dp := m.Gauge().DataPoints().At(0)
	require.InDelta(t, oplogWindow-1.5, dp.DoubleValue(), 0)
	member, ok := dp.Attributes().Get("mongodb.replica_set.member.name")
	require.True(t, ok)
	require.Equal(t, "mongo-1:27017", member.Str())
}

func TestCollectReplicaSetMetrics(t *testing.T) {
	s := newReplicaSetScraper(t)
	status, err := loadTestFileAsMap("./testdata/replSetGetStatus.json")
	require.NoError(t, err)
	oplogStats, err := loadTestFileAsMap("./testdata/oplogStats.json")
	require.NoError(t, err)

	fc := &fakeClient{}
	fc.On("OplogStats", mock.Anything).Return(oplogStats, nil)
	fc.On("OplogBounds", mock.Anything).
		Return(bson.Timestamp{T: oplogOldestSecond}, bson.Timestamp{T: oplogNewestSecond}, nil)
	fc.On("RunCommand", mock.Anything, adminDatabase, bson.M{replSetGetStatusCommand: 1}).Return(status, nil)
	s.client = fc

	errs := &scrapererror.ScrapeErrors{}
	s.collectReplicaSetMetrics(t.Context(), pcommon.NewTimestampFromTime(time.Now()), errs)
	require.NoError(t, errs.Combine())

	require.ElementsMatch(t, []string{
		"mongodb.oplog.limit",
		"mongodb.oplog.usage",
		"mongodb.oplog.window",
		"mongodb.replica_set.headroom",
		"mongodb.replica_set.lag",
		"mongodb.replica_set.member.count",
		"mongodb.replica_set.member.status",
	}, metricNames(s.mb.Emit()))
}

func TestCollectReplicaSetMetricsOnSecondary(t *testing.T) {
	s := newReplicaSetScraper(t)
	status, err := loadTestFileAsMap("./testdata/replSetGetStatusSecondary.json")
	require.NoError(t, err)
	oplogStats, err := loadTestFileAsMap("./testdata/oplogStats.json")
	require.NoError(t, err)

	fc := &fakeClient{}
	fc.On("OplogStats", mock.Anything).Return(oplogStats, nil)
	fc.On("OplogBounds", mock.Anything).
		Return(bson.Timestamp{T: oplogOldestSecond}, bson.Timestamp{T: oplogNewestSecond}, nil)
	fc.On("RunCommand", mock.Anything, adminDatabase, bson.M{replSetGetStatusCommand: 1}).Return(status, nil)
	s.client = fc

	errs := &scrapererror.ScrapeErrors{}
	s.collectReplicaSetMetrics(t.Context(), pcommon.NewTimestampFromTime(time.Now()), errs)
	require.NoError(t, errs.Combine())

	// Lag and headroom are only reported by the primary.
	require.ElementsMatch(t, []string{
		"mongodb.oplog.limit",
		"mongodb.oplog.usage",
		"mongodb.oplog.window",
		"mongodb.replica_set.member.count",
		"mongodb.replica_set.member.status",
	}, metricNames(s.mb.Emit()))
}

func TestCollectReplicaSetMetricsWithoutReplication(t *testing.T) {
	s := newReplicaSetScraper(t)
	noReplication := mongo.CommandError{Code: 76, Name: "NoReplicationEnabled", Message: "not running with --replSet"}

	fc := &fakeClient{}
	fc.On("OplogStats", mock.Anything).Return(nil, noReplication)
	fc.On("OplogBounds", mock.Anything).Return(bson.Timestamp{}, bson.Timestamp{}, mongo.ErrNoDocuments)
	fc.On("RunCommand", mock.Anything, adminDatabase, bson.M{replSetGetStatusCommand: 1}).Return(nil, noReplication)
	s.client = fc

	errs := &scrapererror.ScrapeErrors{}
	s.collectReplicaSetMetrics(t.Context(), pcommon.NewTimestampFromTime(time.Now()), errs)

	// A standalone or a mongos exposes no replica set state; the scrape must not fail over it.
	require.NoError(t, errs.Combine())
	require.Empty(t, metricNames(s.mb.Emit()))
}

func TestCollectReplicaSetMetricsDisabled(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	s := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	// The mock has no expectations, so it fails the test if any command is issued.
	s.client = &fakeClient{}

	errs := &scrapererror.ScrapeErrors{}
	s.collectReplicaSetMetrics(t.Context(), pcommon.NewTimestampFromTime(time.Now()), errs)

	require.NoError(t, errs.Combine())
	require.Empty(t, metricNames(s.mb.Emit()))
}

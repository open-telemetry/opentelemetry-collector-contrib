// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func (m *metricMongodbCollectionCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
}

func (m *metricMongodbConnectionCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string, connectionTypeAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
	dp.Attributes().PutStr("type", connectionTypeAttributeValue)
}

func (m *metricMongodbDataSize) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
}

func (m *metricMongodbDocumentOperationCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string, operationAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
	dp.Attributes().PutStr("operation", operationAttributeValue)
}

func (m *metricMongodbExtentCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
}

func (m *metricMongodbIndexAccessCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string, collectionAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
	dp.Attributes().PutStr("collection", collectionAttributeValue)
}

func (m *metricMongodbIndexCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
}

func (m *metricMongodbIndexSize) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
}

func (m *metricMongodbLockAcquireCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string, lockTypeAttributeValue string, lockModeAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
	dp.Attributes().PutStr("lock_type", lockTypeAttributeValue)
	dp.Attributes().PutStr("lock_mode", lockModeAttributeValue)
}

func (m *metricMongodbLockAcquireTime) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string, lockTypeAttributeValue string, lockModeAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
	dp.Attributes().PutStr("lock_type", lockTypeAttributeValue)
	dp.Attributes().PutStr("lock_mode", lockModeAttributeValue)
}

func (m *metricMongodbLockAcquireWaitCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string, lockTypeAttributeValue string, lockModeAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
	dp.Attributes().PutStr("lock_type", lockTypeAttributeValue)
	dp.Attributes().PutStr("lock_mode", lockModeAttributeValue)
}

func (m *metricMongodbLockDeadlockCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string, lockTypeAttributeValue string, lockModeAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
	dp.Attributes().PutStr("lock_type", lockTypeAttributeValue)
	dp.Attributes().PutStr("lock_mode", lockModeAttributeValue)
}

func (m *metricMongodbMemoryUsage) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string, memoryTypeAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
	dp.Attributes().PutStr("type", memoryTypeAttributeValue)
}

func (m *metricMongodbObjectCount) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
}

func (m *metricMongodbStorageSize) recordDataPointDatabaseAttr(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("database", databaseAttributeValue)
}

// RecordMongodbCollectionCountDataPointDatabaseAttr adds a data point to mongodb.collection.count metric.
func (mb *MetricsBuilder) RecordMongodbCollectionCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	mb.metricMongodbCollectionCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue)
}

// RecordMongodbDataSizeDataPointDatabaseAttr adds a data point to mongodb.data.size metric.
func (mb *MetricsBuilder) RecordMongodbDataSizeDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	mb.metricMongodbDataSize.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue)
}

// RecordMongodbDocumentOperationCountDataPointDatabaseAttr adds a data point to mongodb.document.operation.count metric.
func (mb *MetricsBuilder) RecordMongodbDocumentOperationCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string, operationAttributeValue AttributeOperation) {
	mb.metricMongodbDocumentOperationCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue, operationAttributeValue.String())
}

// RecordMongodbConnectionCountDataPointDatabaseAttr adds a data point to mongodb.connection.count metric.
func (mb *MetricsBuilder) RecordMongodbConnectionCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string, connectionTypeAttributeValue AttributeConnectionType) {
	mb.metricMongodbConnectionCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue, connectionTypeAttributeValue.String())
}

// RecordMongodbExtentCountDataPointDatabaseAttr adds a data point to mongodb.extent.count metric.
func (mb *MetricsBuilder) RecordMongodbExtentCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	mb.metricMongodbExtentCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue)
}

// RecordMongodbIndexAccessCountDataPointDatabaseAttr adds a data point to mongodb.index.access.count metric.
func (mb *MetricsBuilder) RecordMongodbIndexAccessCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string, collectionAttributeValue string) {
	mb.metricMongodbIndexAccessCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue, collectionAttributeValue)
}

// RecordMongodbIndexCountDataPointDatabaseAttr adds a data point to mongodb.index.count metric.
func (mb *MetricsBuilder) RecordMongodbIndexCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	mb.metricMongodbIndexCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue)
}

// RecordMongodbIndexSizeDataPointDatabaseAttr adds a data point to mongodb.index.size metric.
func (mb *MetricsBuilder) RecordMongodbIndexSizeDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	mb.metricMongodbIndexSize.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue)
}

// RecordMongodbLockAcquireCountDataPointDatabaseAttr adds a data point to mongodb.lock.acquire.count metric.
func (mb *MetricsBuilder) RecordMongodbLockAcquireCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string, lockTypeAttributeValue AttributeLockType, lockModeAttributeValue AttributeLockMode) {
	mb.metricMongodbLockAcquireCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue, lockTypeAttributeValue.String(), lockModeAttributeValue.String())
}

// RecordMongodbLockAcquireTimeDataPointDatabaseAttr adds a data point to mongodb.lock.acquire.time metric.
func (mb *MetricsBuilder) RecordMongodbLockAcquireTimeDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string, lockTypeAttributeValue AttributeLockType, lockModeAttributeValue AttributeLockMode) {
	mb.metricMongodbLockAcquireTime.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue, lockTypeAttributeValue.String(), lockModeAttributeValue.String())
}

// RecordMongodbLockAcquireWaitCountDataPointDatabaseAttr adds a data point to mongodb.lock.acquire.wait_count metric.
func (mb *MetricsBuilder) RecordMongodbLockAcquireWaitCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string, lockTypeAttributeValue AttributeLockType, lockModeAttributeValue AttributeLockMode) {
	mb.metricMongodbLockAcquireWaitCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue, lockTypeAttributeValue.String(), lockModeAttributeValue.String())
}

// RecordMongodbLockDeadlockCountDataPointDatabaseAttr adds a data point to mongodb.lock.deadlock.count metric.
func (mb *MetricsBuilder) RecordMongodbLockDeadlockCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string, lockTypeAttributeValue AttributeLockType, lockModeAttributeValue AttributeLockMode) {
	mb.metricMongodbLockDeadlockCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue, lockTypeAttributeValue.String(), lockModeAttributeValue.String())
}

// RecordMongodbMemoryUsageDataPointDatabaseAttr adds a data point to mongodb.memory.usage metric.
func (mb *MetricsBuilder) RecordMongodbMemoryUsageDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string, memoryTypeAttributeValue AttributeMemoryType) {
	mb.metricMongodbMemoryUsage.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue, memoryTypeAttributeValue.String())
}

// RecordMongodbObjectCountDataPointDatabaseAttr adds a data point to mongodb.object.count metric.
func (mb *MetricsBuilder) RecordMongodbObjectCountDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	mb.metricMongodbObjectCount.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue)
}

// RecordMongodbStorageSizeDataPointDatabaseAttr adds a data point to mongodb.storage.size metric.
func (mb *MetricsBuilder) RecordMongodbStorageSizeDataPointDatabaseAttr(ts pcommon.Timestamp, val int64, databaseAttributeValue string) {
	mb.metricMongodbStorageSize.recordDataPointDatabaseAttr(mb.startTime, ts, val, databaseAttributeValue)
}

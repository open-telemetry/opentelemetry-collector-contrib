// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

var errKeyNotFound = errors.New("could not find key for metric")

var operationsMap = map[string]metadata.AttributeOperation{
	"insert":   metadata.AttributeOperationInsert,
	"queries":  metadata.AttributeOperationQuery,
	"update":   metadata.AttributeOperationUpdate,
	"remove":   metadata.AttributeOperationDelete,
	"getmore":  metadata.AttributeOperationGetmore,
	"commands": metadata.AttributeOperationCommand,
}

var documentMap = map[string]metadata.AttributeOperation{
	"inserted": metadata.AttributeOperationInsert,
	"updated":  metadata.AttributeOperationUpdate,
	"deleted":  metadata.AttributeOperationDelete,
}

var lockTypeMap = map[string]metadata.AttributeLockType{
	"ParallelBatchWriterMode":    metadata.AttributeLockTypeParallelBatchWriteMode,
	"ReplicationStateTransition": metadata.AttributeLockTypeReplicationStateTransition,
	"Global":                     metadata.AttributeLockTypeGlobal,
	"Database":                   metadata.AttributeLockTypeDatabase,
	"Collection":                 metadata.AttributeLockTypeCollection,
	"Mutex":                      metadata.AttributeLockTypeMutex,
	"Metadata":                   metadata.AttributeLockTypeMetadata,
	"oplog":                      metadata.AttributeLockTypeOplog,
}

var lockModeMap = map[string]metadata.AttributeLockMode{
	"R": metadata.AttributeLockModeShared,
	"W": metadata.AttributeLockModeExclusive,
	"r": metadata.AttributeLockModeIntentShared,
	"w": metadata.AttributeLockModeIntentExclusive,
}

const (
	collectMetricError          = "failed to collect metric %s: %w"
	collectMetricWithAttributes = "failed to collect metric %s with attribute(s) %s: %w"
)

// DBStats
func (s *mongodbScraper) recordCollections(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"collections"}
	metricName := "mongodb.collection.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	if s.removeDatabaseAttr {
		s.mb.RecordMongodbCollectionCountDataPoint(now, val)
	} else {
		s.mb.RecordMongodbCollectionCountDataPointDatabaseAttr(now, val, dbName)
	}
}

func (s *mongodbScraper) recordDataSize(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dataSize"}
	metricName := "mongodb.data.size"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	if s.removeDatabaseAttr {
		s.mb.RecordMongodbDataSizeDataPoint(now, val)
	} else {
		s.mb.RecordMongodbDataSizeDataPointDatabaseAttr(now, val, dbName)
	}
}

func (s *mongodbScraper) recordStorageSize(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageSize"}
	metricName := "mongodb.storage.size"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	if s.removeDatabaseAttr {
		s.mb.RecordMongodbStorageSizeDataPoint(now, val)
	} else {
		s.mb.RecordMongodbStorageSizeDataPointDatabaseAttr(now, val, dbName)
	}
}

func (s *mongodbScraper) recordObjectCount(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"objects"}
	metricName := "mongodb.object.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	if s.removeDatabaseAttr {
		s.mb.RecordMongodbObjectCountDataPoint(now, val)
	} else {
		s.mb.RecordMongodbObjectCountDataPointDatabaseAttr(now, val, dbName)
	}
}

func (s *mongodbScraper) recordIndexCount(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexes"}
	metricName := "mongodb.index.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	if s.removeDatabaseAttr {
		s.mb.RecordMongodbIndexCountDataPoint(now, val)
	} else {
		s.mb.RecordMongodbIndexCountDataPointDatabaseAttr(now, val, dbName)
	}
}

func (s *mongodbScraper) recordIndexSize(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexSize"}
	metricName := "mongodb.index.size"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	if s.removeDatabaseAttr {
		s.mb.RecordMongodbIndexSizeDataPoint(now, val)
	} else {
		s.mb.RecordMongodbIndexSizeDataPointDatabaseAttr(now, val, dbName)
	}
}

func (s *mongodbScraper) recordExtentCount(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	// Mongo version 4.4+ no longer returns numExtents since it is part of the obsolete MMAPv1
	// https://www.mongodb.com/docs/manual/release-notes/4.4-compatibility/#mmapv1-cleanup
	mongo44, _ := version.NewVersion("4.4")
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo44) {
		metricPath := []string{"numExtents"}
		metricName := "mongodb.extent.count"
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
			return
		}
		if s.removeDatabaseAttr {
			s.mb.RecordMongodbExtentCountDataPoint(now, val)
		} else {
			s.mb.RecordMongodbExtentCountDataPointDatabaseAttr(now, val, dbName)
		}
	}
}

// ServerStatus
func (s *mongodbScraper) recordConnections(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	for ctVal, ct := range metadata.MapAttributeConnectionType {
		metricPath := []string{"connections", ctVal}
		metricName := "mongodb.connection.count"
		metricAttributes := fmt.Sprintf("%s, %s", ctVal, dbName)
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
			continue
		}
		if s.removeDatabaseAttr {
			s.mb.RecordMongodbConnectionCountDataPoint(now, val, ct)
		} else {
			s.mb.RecordMongodbConnectionCountDataPointDatabaseAttr(now, val, dbName, ct)
		}
	}
}

func (s *mongodbScraper) recordMemoryUsage(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	for mtVal, mt := range metadata.MapAttributeMemoryType {
		metricPath := []string{"mem", mtVal}
		metricName := "mongodb.memory.usage"
		metricAttributes := fmt.Sprintf("%s, %s", mtVal, dbName)
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
			continue
		}
		// convert from mebibytes to bytes
		memUsageBytes := val * int64(1048576)
		if s.removeDatabaseAttr {
			s.mb.RecordMongodbMemoryUsageDataPoint(now, memUsageBytes, mt)
		} else {
			s.mb.RecordMongodbMemoryUsageDataPointDatabaseAttr(now, memUsageBytes, dbName, mt)
		}
	}
}

func (s *mongodbScraper) recordDocumentOperations(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	for operationKey, metadataKey := range documentMap {
		metricPath := []string{"metrics", "document", operationKey}
		metricName := "mongodb.document.operation.count"
		metricAttributes := fmt.Sprintf("%s, %s", operationKey, dbName)
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
			continue
		}
		if s.removeDatabaseAttr {
			s.mb.RecordMongodbDocumentOperationCountDataPoint(now, val, metadataKey)
		} else {
			s.mb.RecordMongodbDocumentOperationCountDataPointDatabaseAttr(now, val, dbName, metadataKey)
		}
	}
}

func (s *mongodbScraper) recordSessionCount(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine for session count"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "session", "open session count"}
	metricName := "mongodb.session.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	s.mb.RecordMongodbSessionCountDataPoint(now, val)
}

func (s *mongodbScraper) recordLatencyTime(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	for operationVal, operation := range metadata.MapAttributeOperationLatency {
		metricPath := []string{"opLatencies", operationVal + "s", "latency"}
		metricName := "mongodb.operation.latency.time"
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, operationVal, err))
			continue
		}
		s.mb.RecordMongodbOperationLatencyTimeDataPoint(now, val, operation)
	}
}

// Admin Stats
func (s *mongodbScraper) recordOperations(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	for operationVal, operation := range metadata.MapAttributeOperation {
		metricPath := []string{"opcounters", operationVal}
		metricName := "mongodb.operation.count"
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, operationVal, err))
			continue
		}
		s.mb.RecordMongodbOperationCountDataPoint(now, val, operation)
	}
}

func (s *mongodbScraper) recordOperationsRepl(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	for operationVal, operation := range metadata.MapAttributeOperation {
		metricPath := []string{"opcountersRepl", operationVal}
		metricName := "mongodb.operation.repl.count"
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, operationVal, err))
			continue
		}
		s.mb.RecordMongodbOperationReplCountDataPoint(now, val, operation)
	}
}

func (s *mongodbScraper) recordCacheOperations(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine for cache operations"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "pages read into cache"}
	metricName := "mongodb.cache.operations"
	cacheMissVal, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(2, fmt.Errorf(collectMetricWithAttributes, metricName, "miss, hit", err))
		return
	}
	s.mb.RecordMongodbCacheOperationsDataPoint(now, cacheMissVal, metadata.AttributeTypeMiss)

	cacheHitPath := []string{"wiredTiger", "cache", "pages requested from the cache"}
	cacheHitName := "mongodb.cache.operations"
	cacheHitVal, err := collectMetric(doc, cacheHitPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, cacheHitName, "hit", err))
		return
	}

	cacheHits := cacheHitVal - cacheMissVal
	s.mb.RecordMongodbCacheOperationsDataPoint(now, cacheHits, metadata.AttributeTypeHit)
}

func (s *mongodbScraper) recordGlobalLockTime(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "totalTime"}
	metricName := "mongodb.global_lock.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	heldTimeMilliseconds := val / 1000
	s.mb.RecordMongodbGlobalLockTimeDataPoint(now, heldTimeMilliseconds)
}

func (s *mongodbScraper) recordCursorCount(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "cursor", "open", "total"}
	metricName := "mongodb.cursor.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	s.mb.RecordMongodbCursorCountDataPoint(now, val)
}

func (s *mongodbScraper) recordCursorTimeoutCount(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "cursor", "timedOut"}
	metricName := "mongodb.cursor.timeout.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	s.mb.RecordMongodbCursorTimeoutCountDataPoint(now, val)
}

func (s *mongodbScraper) recordNetworkCount(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	networkRecorderMap := map[string]func(pcommon.Timestamp, int64){
		"bytesIn":     s.mb.RecordMongodbNetworkIoReceiveDataPoint,
		"bytesOut":    s.mb.RecordMongodbNetworkIoTransmitDataPoint,
		"numRequests": s.mb.RecordMongodbNetworkRequestCountDataPoint,
	}
	for networkKey, recorder := range networkRecorderMap {
		metricPath := []string{"network", networkKey}
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricError, networkKey, err))
			continue
		}
		recorder(now, val)
	}
}

func (s *mongodbScraper) recordUptime(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"uptimeMillis"}
	metricName := "mongodb.uptime"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	s.mb.RecordMongodbUptimeDataPoint(now, val)
}

func (s *mongodbScraper) recordHealth(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"ok"}
	metricName := "mongodb.health"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	s.mb.RecordMongodbHealthDataPoint(now, val)
}

// Lock Metrics are only supported by MongoDB v3.2+
func (s *mongodbScraper) recordLockAcquireCounts(now pcommon.Timestamp, doc bson.M, dBName string, errs *scrapererror.ScrapeErrors) {
	mongo32, _ := version.NewVersion("3.2")
	if s.mongoVersion.LessThan(mongo32) {
		return
	}
	mongo42, _ := version.NewVersion("4.2")
	for lockTypeKey, lockTypeAttribute := range lockTypeMap {
		for lockModeKey, lockModeAttribute := range lockModeMap {
			// Continue if the lock type is not supported by current server's MongoDB version
			if s.mongoVersion.LessThan(mongo42) && (lockTypeKey == "ParallelBatchWriterMode" || lockTypeKey == "ReplicationStateTransition") {
				continue
			}
			metricPath := []string{"locks", lockTypeKey, "acquireCount", lockModeKey}
			metricName := "mongodb.lock.acquire.count"
			metricAttributes := fmt.Sprintf("%s, %s, %s", dBName, lockTypeAttribute.String(), lockModeAttribute.String())
			val, err := collectMetric(doc, metricPath)
			// MongoDB only publishes this lock metric is it is available.
			// Do not raise error when key is not found
			if errors.Is(err, errKeyNotFound) {
				continue
			}
			if err != nil {
				errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
				continue
			}
			if s.removeDatabaseAttr {
				s.mb.RecordMongodbLockAcquireCountDataPoint(now, val, lockTypeAttribute, lockModeAttribute)
			} else {
				s.mb.RecordMongodbLockAcquireCountDataPointDatabaseAttr(now, val, dBName, lockTypeAttribute, lockModeAttribute)
			}
		}
	}
}

func (s *mongodbScraper) recordLockAcquireWaitCounts(now pcommon.Timestamp, doc bson.M, dBName string, errs *scrapererror.ScrapeErrors) {
	mongo32, _ := version.NewVersion("3.2")
	if s.mongoVersion.LessThan(mongo32) {
		return
	}
	mongo42, _ := version.NewVersion("4.2")
	for lockTypeKey, lockTypeAttribute := range lockTypeMap {
		for lockModeKey, lockModeAttribute := range lockModeMap {
			// Continue if the lock type is not supported by current server's MongoDB version
			if s.mongoVersion.LessThan(mongo42) && (lockTypeKey == "ParallelBatchWriterMode" || lockTypeKey == "ReplicationStateTransition") {
				continue
			}
			metricPath := []string{"locks", lockTypeKey, "acquireWaitCount", lockModeKey}
			metricName := "mongodb.lock.acquire.wait_count"
			metricAttributes := fmt.Sprintf("%s, %s, %s", dBName, lockTypeAttribute.String(), lockModeAttribute.String())
			val, err := collectMetric(doc, metricPath)
			// MongoDB only publishes this lock metric is it is available.
			// Do not raise error when key is not found
			if errors.Is(err, errKeyNotFound) {
				continue
			}
			if err != nil {
				errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
				continue
			}
			if s.removeDatabaseAttr {
				s.mb.RecordMongodbLockAcquireWaitCountDataPoint(now, val, lockTypeAttribute, lockModeAttribute)
			} else {
				s.mb.RecordMongodbLockAcquireWaitCountDataPointDatabaseAttr(now, val, dBName, lockTypeAttribute, lockModeAttribute)
			}
		}
	}
}

func (s *mongodbScraper) recordLockTimeAcquiringMicros(now pcommon.Timestamp, doc bson.M, dBName string, errs *scrapererror.ScrapeErrors) {
	mongo32, _ := version.NewVersion("3.2")
	if s.mongoVersion.LessThan(mongo32) {
		return
	}
	mongo42, _ := version.NewVersion("4.2")
	for lockTypeKey, lockTypeAttribute := range lockTypeMap {
		for lockModeKey, lockModeAttribute := range lockModeMap {
			// Continue if the lock type is not supported by current server's MongoDB version
			if s.mongoVersion.LessThan(mongo42) && (lockTypeKey == "ParallelBatchWriterMode" || lockTypeKey == "ReplicationStateTransition") {
				continue
			}
			metricPath := []string{"locks", lockTypeKey, "timeAcquiringMicros", lockModeKey}
			metricName := "mongodb.lock.acquire.time"
			metricAttributes := fmt.Sprintf("%s, %s, %s", dBName, lockTypeAttribute.String(), lockModeAttribute.String())
			val, err := collectMetric(doc, metricPath)
			// MongoDB only publishes this lock metric is it is available.
			// Do not raise error when key is not found
			if errors.Is(err, errKeyNotFound) {
				continue
			}
			if err != nil {
				errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
				continue
			}
			if s.removeDatabaseAttr {
				s.mb.RecordMongodbLockAcquireTimeDataPoint(now, val, lockTypeAttribute, lockModeAttribute)
			} else {
				s.mb.RecordMongodbLockAcquireTimeDataPointDatabaseAttr(now, val, dBName, lockTypeAttribute, lockModeAttribute)
			}
		}
	}
}

func (s *mongodbScraper) recordLockDeadlockCount(now pcommon.Timestamp, doc bson.M, dBName string, errs *scrapererror.ScrapeErrors) {
	mongo32, _ := version.NewVersion("3.2")
	if s.mongoVersion.LessThan(mongo32) {
		return
	}
	mongo42, _ := version.NewVersion("4.2")
	for lockTypeKey, lockTypeAttribute := range lockTypeMap {
		for lockModeKey, lockModeAttribute := range lockModeMap {
			// Continue if the lock type is not supported by current server's MongoDB version
			if s.mongoVersion.LessThan(mongo42) && (lockTypeKey == "ParallelBatchWriterMode" || lockTypeKey == "ReplicationStateTransition") {
				continue
			}
			metricPath := []string{"locks", lockTypeKey, "deadlockCount", lockModeKey}
			metricName := "mongodb.lock.deadlock.count"
			metricAttributes := fmt.Sprintf("%s, %s, %s", dBName, lockTypeAttribute.String(), lockModeAttribute.String())
			val, err := collectMetric(doc, metricPath)
			// MongoDB only publishes this lock metric is it is available.
			// Do not raise error when key is not found
			if errors.Is(err, errKeyNotFound) {
				continue
			}
			if err != nil {
				errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
				continue
			}
			if s.removeDatabaseAttr {
				s.mb.RecordMongodbLockDeadlockCountDataPoint(now, val, lockTypeAttribute, lockModeAttribute)
			} else {
				s.mb.RecordMongodbLockDeadlockCountDataPointDatabaseAttr(now, val, dBName, lockTypeAttribute, lockModeAttribute)
			}
		}
	}
}

// Index Stats
func (s *mongodbScraper) recordIndexAccess(now pcommon.Timestamp, documents []bson.M, dbName string, collectionName string, errs *scrapererror.ScrapeErrors) {
	metricName := "mongodb.index.access.count"
	var indexAccessTotal int64
	for _, doc := range documents {
		metricAttributes := fmt.Sprintf("%s, %s", dbName, collectionName)
		indexAccess, ok := doc["accesses"].(bson.M)["ops"]
		if !ok {
			err := errors.New("could not find key for index access metric")
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
			return
		}
		indexAccessValue, err := parseInt(indexAccess)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
			return
		}
		indexAccessTotal += indexAccessValue
	}
	if s.removeDatabaseAttr {
		s.mb.RecordMongodbIndexAccessCountDataPoint(now, indexAccessTotal, collectionName)
	} else {
		s.mb.RecordMongodbIndexAccessCountDataPointDatabaseAttr(now, indexAccessTotal, dbName, collectionName)
	}
}

// Top Stats
func (s *mongodbScraper) recordOperationTime(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricName := "mongodb.operation.time"
	collectionPathNames, err := digForCollectionPathNames(doc)
	if err != nil {
		errs.AddPartial(len(operationsMap), fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	operationTimeValues, err := aggregateOperationTimeValues(doc, collectionPathNames, operationsMap)
	if err != nil {
		errs.AddPartial(len(operationsMap), fmt.Errorf(collectMetricError, metricName, err))
		return
	}

	for operationName, metadataOperationName := range operationsMap {
		operationValue, ok := operationTimeValues[operationName]
		if !ok {
			err := errors.New("could not find key for operation name")
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, operationName, err))
			continue
		}
		s.mb.RecordMongodbOperationTimeDataPoint(now, operationValue, metadataOperationName)
	}
}

func aggregateOperationTimeValues(document bson.M, collectionPathNames []string, operationMap map[string]metadata.AttributeOperation) (map[string]int64, error) {
	operationTotals := map[string]int64{}
	for _, collectionPathName := range collectionPathNames {
		for operationName := range operationMap {
			value, err := getOperationTimeValues(document, collectionPathName, operationName)
			if err != nil {
				return nil, err
			}
			operationTotals[operationName] += value
		}
	}
	return operationTotals, nil
}

func getOperationTimeValues(document bson.M, collectionPathName, operation string) (int64, error) {
	rawValue, err := dig(document, []string{"totals", collectionPathName, operation, "time"})
	if err != nil {
		return 0, err
	}
	return parseInt(rawValue)
}

func digForCollectionPathNames(document bson.M) ([]string, error) {
	docTotals, ok := document["totals"].(bson.M)
	if !ok {
		return nil, errKeyNotFound
	}
	var collectionPathNames []string
	for collectionPathName := range docTotals {
		if collectionPathName != "note" {
			collectionPathNames = append(collectionPathNames, collectionPathName)
		}
	}
	return collectionPathNames, nil
}

func digForIndexNames(document bson.M) ([]string, error) {
	docIndexes, ok := document["storageStats"].(bson.M)
	if ok {
		docIndexes, ok = docIndexes["indexSizes"].(bson.M)
	}
	if !ok {
		return nil, errKeyNotFound
	}
	var indexNames []string
	for indexName := range docIndexes {
		indexNames = append(indexNames, indexName)
	}
	return indexNames, nil
}

func collectMetric(document bson.M, path []string) (int64, error) {
	metric, err := dig(document, path)
	if err != nil {
		return 0, err
	}
	return parseInt(metric)
}

func dig(document bson.M, path []string) (any, error) {
	curItem, remainingPath := path[0], path[1:]
	value := document[curItem]
	if value == nil {
		return 0, errKeyNotFound
	}
	if len(remainingPath) == 0 {
		return value, nil
	}
	return dig(value.(bson.M), remainingPath)
}

func parseInt(val any) (int64, error) {
	switch v := val.(type) {
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	case bool:
		if v {
			return int64(1), nil
		}
		return int64(0), nil
	default:
		return 0, fmt.Errorf("could not parse value as int: %v", reflect.TypeOf(val))
	}
}

func (s *mongodbScraper) recordTopStats(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	collectionPathNames, err := digForCollectionPathNames(doc)
	if err != nil {
		errs.AddPartial(len(collectionPathNames), fmt.Errorf("failed to collect top stats metrics: %w", err))
		return
	}
	doc = doc["totals"].(bson.M)
	for _, cpname := range collectionPathNames {
		database, collection, ok := strings.Cut(cpname, ".")
		if ok {
			docmap := doc[cpname].(bson.M)
			// usage
			s.recordMongodbUsageCommandsCount(now, docmap, database, collection, errs) // ps
			s.recordMongodbUsageCommandsTime(now, docmap, database, collection, errs)
			s.recordMongodbUsageGetmoreCount(now, docmap, database, collection, errs) // ps
			s.recordMongodbUsageGetmoreTime(now, docmap, database, collection, errs)
			s.recordMongodbUsageInsertCount(now, docmap, database, collection, errs) // ps
			s.recordMongodbUsageInsertTime(now, docmap, database, collection, errs)
			s.recordMongodbUsageQueriesCount(now, docmap, database, collection, errs) // ps
			s.recordMongodbUsageQueriesTime(now, docmap, database, collection, errs)
			s.recordMongodbUsageReadlockCount(now, docmap, database, collection, errs) // ps
			s.recordMongodbUsageReadlockTime(now, docmap, database, collection, errs)
			s.recordMongodbUsageRemoveCount(now, docmap, database, collection, errs) // ps
			s.recordMongodbUsageRemoveTime(now, docmap, database, collection, errs)
			s.recordMongodbUsageTotalCount(now, docmap, database, collection, errs) // ps
			s.recordMongodbUsageTotalTime(now, docmap, database, collection, errs)
			s.recordMongodbUsageUpdateCount(now, docmap, database, collection, errs) // ps
			s.recordMongodbUsageUpdateTime(now, docmap, database, collection, errs)
			s.recordMongodbUsageWritelockCount(now, docmap, database, collection, errs) // ps
			s.recordMongodbUsageWritelockTime(now, docmap, database, collection, errs)
		}

	}
}

/////////////////////////////////////////////NEW METRICS////////////////////////////////////

func (s *mongodbScraper) recordMongodbAssertsMsgps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"asserts", "msg"}
	metricName := "mongodb.asserts.msgps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbAssertsMsgpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbAssertsRegularps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"asserts", "regular"}
	metricName := "mongodb.asserts.regularps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbAssertsRegularpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbAssertsRolloversps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"asserts", "rollovers"}
	metricName := "mongodb.asserts.rolloversps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbAssertsRolloverspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbAssertsUserps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"asserts", "user"}
	metricName := "mongodb.asserts.userps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbAssertsUserpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbAssertsWarningps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"asserts", "warning"}
	metricName := "mongodb.asserts.warningps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbAssertsWarningpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbBackgroundflushingAverageMs(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"backgroundFlushing", "average_ms"}
	metricName := "mongodb.backgroundflushing.average_ms"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbBackgroundflushingAverageMsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbBackgroundflushingFlushesps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"backgroundFlushing", "flushes"}
	metricName := "mongodb.backgroundflushing.flushesps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbBackgroundflushingFlushespsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbBackgroundflushingLastMs(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"backgroundFlushing", "last_ms"}
	metricName := "mongodb.backgroundflushing.last_ms"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbBackgroundflushingLastMsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbBackgroundflushingTotalMs(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"backgroundFlushing", "total_ms"}
	metricName := "mongodb.backgroundflushing.total_ms"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbBackgroundflushingTotalMsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbChunksJumbo(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"jumbo"}
	metricName := "mongodb.chunks.jumbo"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbChunksJumboDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbChunksTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"total"}
	metricName := "mongodb.chunks.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbChunksTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbCollectionAvgobjsize(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageStats", "avgObjSize"}
	metricName := "mongodb.collection.avgobjsize"
	metricAttributes := fmt.Sprintf("%s, %s", database, collection)
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
		return
	}
	s.mb.RecordMongodbCollectionAvgobjsizeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbCollectionCapped(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageStats", "capped"}
	metricName := "mongodb.collection.capped"
	metricAttributes := fmt.Sprintf("%s, %s", database, collection)
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
		return
	}
	s.mb.RecordMongodbCollectionCappedDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbCollectionObjects(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageStats", "count"}
	metricName := "mongodb.collection.objects"
	metricAttributes := fmt.Sprintf("%s, %s", database, collection)
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
		return
	}
	s.mb.RecordMongodbCollectionObjectsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbCollectionIndexsizes(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricName := "mongodb.collection.indexsizes"
	indexNames, err := digForIndexNames(doc)
	if err != nil {
		errs.AddPartial(len(operationsMap), fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	for _, index := range indexNames {
		metricPath := []string{"storageStats", "indexSizes", index}
		metricAttributes := fmt.Sprintf("%s, %s, %s", database, collection, index)
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
			return
		}
		s.mb.RecordMongodbCollectionIndexsizesDataPoint(now, val, database, collection, index)
	}
}

func (s *mongodbScraper) recordMongodbCollectionMax(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageStats", "max"}
	metricName := "mongodb.collection.max"
	metricAttributes := fmt.Sprintf("%s, %s", database, collection)
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
		return
	}
	s.mb.RecordMongodbCollectionMaxDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbCollectionMaxsize(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageStats", "maxSize"}
	metricName := "mongodb.collection.maxsize"
	metricAttributes := fmt.Sprintf("%s, %s", database, collection)
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
		return
	}
	s.mb.RecordMongodbCollectionMaxsizeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbCollectionNindexes(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageStats", "nindexes"}
	metricName := "mongodb.collection.nindexes"
	metricAttributes := fmt.Sprintf("%s, %s", database, collection)
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
		return
	}
	s.mb.RecordMongodbCollectionNindexesDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbCollectionSize(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageStats", "size"}
	metricName := "mongodb.collection.size"
	metricAttributes := fmt.Sprintf("%s, %s", database, collection)
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
		return
	}
	s.mb.RecordMongodbCollectionSizeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbCollectionStoragesize(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageStats", "storageSize"}
	metricName := "mongodb.collection.storagesize"
	metricAttributes := fmt.Sprintf("%s, %s", database, collection)
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
		return
	}
	s.mb.RecordMongodbCollectionStoragesizeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbConnectionPoolNumascopedconnections(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"numAScopedConnections"}
	metricName := "mongodb.connection_pool.numascopedconnections"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionPoolNumascopedconnectionsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionPoolNumclientconnections(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"numClientConnections"}
	metricName := "mongodb.connection_pool.numclientconnections"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionPoolNumclientconnectionsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionPoolTotalavailable(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"totalAvailable"}
	metricName := "mongodb.connection_pool.totalavailable"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionPoolTotalavailableDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionPoolTotalcreatedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"totalCreated"}
	metricName := "mongodb.connection_pool.totalcreatedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionPoolTotalcreatedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionPoolTotalinuse(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"totalInUse"}
	metricName := "mongodb.connection_pool.totalinuse"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionPoolTotalinuseDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionPoolTotalrefreshing(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"totalRefreshing"}
	metricName := "mongodb.connection_pool.totalrefreshing"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionPoolTotalrefreshingDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionsActive(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"connections", "active"}
	metricName := "mongodb.connections.active"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionsActiveDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionsAvailable(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"connections", "available"}
	metricName := "mongodb.connections.available"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionsAvailableDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionsAwaitingtopologychanges(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"connections", "awaitingTopologyChanges"}
	metricName := "mongodb.connections.awaitingtopologychanges"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionsAwaitingtopologychangesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionsCurrent(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"connections", "current"}
	metricName := "mongodb.connections.current"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionsCurrentDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionsExhausthello(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"connections", "exhaustHello"}
	metricName := "mongodb.connections.exhausthello"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionsExhausthelloDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionsExhaustismaster(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"connections", "exhaustIsMaster"}
	metricName := "mongodb.connections.exhaustismaster"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionsExhaustismasterDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionsLoadbalanced(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	// Mongo version 7.0+ only has loadbalanced connections
	mongo70, _ := version.NewVersion("7.0")
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo70) {
		metricPath := []string{"connections", "loadBalanced"}
		metricName := "mongodb.connections.loadbalanced"
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
			return
		}
		s.mb.RecordMongodbConnectionsLoadbalancedDataPoint(now, val, database)
	}
}

func (s *mongodbScraper) recordMongodbConnectionsRejected(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"connections", "rejected"}
	metricName := "mongodb.connections.rejected"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionsRejectedDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionsThreaded(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"connections", "threaded"}
	metricName := "mongodb.connections.threaded"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionsThreadedDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbConnectionsTotalcreated(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"connections", "totalCreated"}
	metricName := "mongodb.connections.totalcreated"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbConnectionsTotalcreatedDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbCursorsTimedout(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "cursor", "timedOut"}
	metricName := "mongodb.cursors.timedout"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbCursorsTimedoutDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbCursorsTotalopen(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "cursor", "totalOpened"}
	metricName := "mongodb.cursors.totalopen"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbCursorsTotalopenDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurCommits(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "commits"}
	metricName := "mongodb.dur.commits"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurCommitsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurCommitsinwritelock(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "commitsInWriteLock"}
	metricName := "mongodb.dur.commitsinwritelock"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurCommitsinwritelockDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurCompression(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "compression"}
	metricName := "mongodb.dur.compression"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurCompressionDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurEarlycommits(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "earlyCommits"}
	metricName := "mongodb.dur.earlycommits"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurEarlycommitsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurJournaledmb(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "journaledMB"}
	metricName := "mongodb.dur.journaledmb"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurJournaledmbDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurTimemsCommits(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "timeMs", "commits"}
	metricName := "mongodb.dur.timems.commits"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurTimemsCommitsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurTimemsCommitsinwritelock(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "timeMs", "commitsInWriteLock"}
	metricName := "mongodb.dur.timems.commitsinwritelock"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurTimemsCommitsinwritelockDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurTimemsDt(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "timeMs", "dt"}
	metricName := "mongodb.dur.timems.dt"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurTimemsDtDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurTimemsPreplogbuffer(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "timeMs", "prepLogBuffer"}
	metricName := "mongodb.dur.timems.preplogbuffer"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurTimemsPreplogbufferDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurTimemsRemapprivateview(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "timeMs", "remapPrivateView"}
	metricName := "mongodb.dur.timems.remapprivateview"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurTimemsRemapprivateviewDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurTimemsWritetodatafiles(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "timeMs", "writeToDataFiles"}
	metricName := "mongodb.dur.timems.writetodatafiles"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurTimemsWritetodatafilesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurTimemsWritetojournal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "timeMs", "writeToJournal"}
	metricName := "mongodb.dur.timems.writetojournal"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurTimemsWritetojournalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbDurWritetodatafilesmb(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dur", "writeToDataFilesMB"}
	metricName := "mongodb.dur.writetodatafilesmb"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbDurWritetodatafilesmbDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbExtraInfoHeapUsageBytesps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"extra_info", "heap_usage_bytes"}
	metricName := "mongodb.extra_info.heap_usage_bytesps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbExtraInfoHeapUsageBytespsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbExtraInfoPageFaultsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"extra_info", "page_faults"}
	metricName := "mongodb.extra_info.page_faultsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbExtraInfoPageFaultspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbFsynclocked(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"fsyncLocked"}
	metricName := "mongodb.fsynclocked"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbFsynclockedDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbGloballockActiveclientsReaders(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "activeClients", "readers"}
	metricName := "mongodb.globallock.activeclients.readers"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbGloballockActiveclientsReadersDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbGloballockActiveclientsTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "activeClients", "total"}
	metricName := "mongodb.globallock.activeclients.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbGloballockActiveclientsTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbGloballockActiveclientsWriters(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "activeClients", "writers"}
	metricName := "mongodb.globallock.activeclients.writers"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbGloballockActiveclientsWritersDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbGloballockCurrentqueueReaders(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "currentQueue", "readers"}
	metricName := "mongodb.globallock.currentqueue.readers"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbGloballockCurrentqueueReadersDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbGloballockCurrentqueueTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "currentQueue", "total"}
	metricName := "mongodb.globallock.currentqueue.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbGloballockCurrentqueueTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbGloballockCurrentqueueWriters(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "currentQueue", "writers"}
	metricName := "mongodb.globallock.currentqueue.writers"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbGloballockCurrentqueueWritersDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbGloballockLocktime(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "lockTime"}
	metricName := "mongodb.globallock.locktime"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbGloballockLocktimeDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbGloballockRatio(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "ratio"}
	metricName := "mongodb.globallock.ratio"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbGloballockRatioDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbGloballockTotaltime(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "totalTime"}
	metricName := "mongodb.globallock.totaltime"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbGloballockTotaltimeDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbIndexcountersAccessesps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexCounters", "accesses"}
	metricName := "mongodb.indexcounters.accessesps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbIndexcountersAccessespsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbIndexcountersHitsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexCounters", "hits"}
	metricName := "mongodb.indexcounters.hitsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbIndexcountersHitspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbIndexcountersMissesps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexCounters", "misses"}
	metricName := "mongodb.indexcounters.missesps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbIndexcountersMissespsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbIndexcountersMissratio(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexCounters", "missRatio"}
	metricName := "mongodb.indexcounters.missratio"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbIndexcountersMissratioDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbIndexcountersResetsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexCounters", "resets"}
	metricName := "mongodb.indexcounters.resetsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbIndexcountersResetspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksCollectionAcquirecountExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Collection", "acquireCount", "W"}
	metricName := "mongodb.locks.collection.acquirecount.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksCollectionAcquirecountExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksCollectionAcquirecountIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Collection", "acquireCount", "w"}
	metricName := "mongodb.locks.collection.acquirecount.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksCollectionAcquirecountIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksCollectionAcquirecountIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Collection", "acquireCount", "r"}
	metricName := "mongodb.locks.collection.acquirecount.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksCollectionAcquirecountIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksCollectionAcquirecountSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Collection", "acquireCount", "R"}
	metricName := "mongodb.locks.collection.acquirecount.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksCollectionAcquirecountSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksCollectionAcquirewaitcountExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Collection", "acquireWaitCount", "W"}
	metricName := "mongodb.locks.collection.acquirewaitcount.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksCollectionAcquirewaitcountExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksCollectionAcquirewaitcountSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Collection", "acquireWaitCount", "R"}
	metricName := "mongodb.locks.collection.acquirewaitcount.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksCollectionAcquirewaitcountSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksCollectionTimeacquiringmicrosExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Collection", "timeAcquiringMicros", "W"}
	metricName := "mongodb.locks.collection.timeacquiringmicros.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksCollectionTimeacquiringmicrosExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksCollectionTimeacquiringmicrosSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Collection", "timeAcquiringMicros", "R"}
	metricName := "mongodb.locks.collection.timeacquiringmicros.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksCollectionTimeacquiringmicrosSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseAcquirecountExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "acquireCount", "W"}
	metricName := "mongodb.locks.database.acquirecount.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseAcquirecountExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseAcquirecountIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "acquireCount", "w"}
	metricName := "mongodb.locks.database.acquirecount.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseAcquirecountIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseAcquirecountIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "acquireCount", "r"}
	metricName := "mongodb.locks.database.acquirecount.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseAcquirecountIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseAcquirecountSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "acquireCount", "R"}
	metricName := "mongodb.locks.database.acquirecount.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseAcquirecountSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseAcquirewaitcountExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "acquireWaitCount", "W"}
	metricName := "mongodb.locks.database.acquirewaitcount.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseAcquirewaitcountExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseAcquirewaitcountIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "acquireWaitCount", "w"}
	metricName := "mongodb.locks.database.acquirewaitcount.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseAcquirewaitcountIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseAcquirewaitcountIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "acquireWaitCount", "r"}
	metricName := "mongodb.locks.database.acquirewaitcount.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseAcquirewaitcountIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseAcquirewaitcountSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "acquireWaitCount", "R"}
	metricName := "mongodb.locks.database.acquirewaitcount.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseAcquirewaitcountSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseTimeacquiringmicrosExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "timeAcquiringMicros", "W"}
	metricName := "mongodb.locks.database.timeacquiringmicros.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseTimeacquiringmicrosExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseTimeacquiringmicrosIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "timeAcquiringMicros", "w"}
	metricName := "mongodb.locks.database.timeacquiringmicros.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseTimeacquiringmicrosIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseTimeacquiringmicrosIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "timeAcquiringMicros", "r"}
	metricName := "mongodb.locks.database.timeacquiringmicros.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseTimeacquiringmicrosIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksDatabaseTimeacquiringmicrosSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Database", "timeAcquiringMicros", "R"}
	metricName := "mongodb.locks.database.timeacquiringmicros.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksDatabaseTimeacquiringmicrosSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalAcquirecountExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "acquireCount", "W"}
	metricName := "mongodb.locks.global.acquirecount.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalAcquirecountExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalAcquirecountIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "acquireCount", "w"}
	metricName := "mongodb.locks.global.acquirecount.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalAcquirecountIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalAcquirecountIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "acquireCount", "r"}
	metricName := "mongodb.locks.global.acquirecount.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalAcquirecountIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalAcquirecountSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "acquireCount", "R"}
	metricName := "mongodb.locks.global.acquirecount.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalAcquirecountSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalAcquirewaitcountExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "acquireWaitCount", "W"}
	metricName := "mongodb.locks.global.acquirewaitcount.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalAcquirewaitcountExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalAcquirewaitcountIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "acquireWaitCount", "w"}
	metricName := "mongodb.locks.global.acquirewaitcount.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalAcquirewaitcountIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalAcquirewaitcountIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "acquireWaitCount", "r"}
	metricName := "mongodb.locks.global.acquirewaitcount.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalAcquirewaitcountIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalAcquirewaitcountSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "acquireWaitCount", "R"}
	metricName := "mongodb.locks.global.acquirewaitcount.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalAcquirewaitcountSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalTimeacquiringmicrosExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "timeAcquiringMicros", "W"}
	metricName := "mongodb.locks.global.timeacquiringmicros.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalTimeacquiringmicrosExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalTimeacquiringmicrosIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "timeAcquiringMicros", "w"}
	metricName := "mongodb.locks.global.timeacquiringmicros.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalTimeacquiringmicrosIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalTimeacquiringmicrosIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "timeAcquiringMicros", "r"}
	metricName := "mongodb.locks.global.timeacquiringmicros.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalTimeacquiringmicrosIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksGlobalTimeacquiringmicrosSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Global", "timeAcquiringMicros", "R"}
	metricName := "mongodb.locks.global.timeacquiringmicros.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksGlobalTimeacquiringmicrosSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksMetadataAcquirecountExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Metadata", "acquireCount", "W"}
	metricName := "mongodb.locks.metadata.acquirecount.exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksMetadataAcquirecountExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksMetadataAcquirecountSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "Metadata", "acquireCount", "R"}
	metricName := "mongodb.locks.metadata.acquirecount.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksMetadataAcquirecountSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksMmapv1journalAcquirecountIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "MMAPV1Journal", "acquireCount", "w"}
	metricName := "mongodb.locks.mmapv1journal.acquirecount.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksMmapv1journalAcquirecountIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksMmapv1journalAcquirecountIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "MMAPV1Journal", "acquireCount", "r"}
	metricName := "mongodb.locks.mmapv1journal.acquirecount.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksMmapv1journalAcquirecountIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksMmapv1journalAcquirewaitcountIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "MMAPV1Journal", "acquireWaitCount", "w"}
	metricName := "mongodb.locks.mmapv1journal.acquirewaitcount.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksMmapv1journalAcquirewaitcountIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksMmapv1journalAcquirewaitcountIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "MMAPV1Journal", "acquireWaitCount", "r"}
	metricName := "mongodb.locks.mmapv1journal.acquirewaitcount.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksMmapv1journalAcquirewaitcountIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksMmapv1journalTimeacquiringmicrosIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "MMAPV1Journal", "timeAcquiringMicros", "w"}
	metricName := "mongodb.locks.mmapv1journal.timeacquiringmicros.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksMmapv1journalTimeacquiringmicrosIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksMmapv1journalTimeacquiringmicrosIntentSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "MMAPV1Journal", "timeAcquiringMicros", "r"}
	metricName := "mongodb.locks.mmapv1journal.timeacquiringmicros.intent_sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksMmapv1journalTimeacquiringmicrosIntentSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksOplogAcquirecountIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "oplog", "acquireCount", "w"}
	metricName := "mongodb.locks.oplog.acquirecount.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksOplogAcquirecountIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksOplogAcquirecountSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "oplog", "acquireCount", "R"}
	metricName := "mongodb.locks.oplog.acquirecount.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksOplogAcquirecountSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksOplogAcquirewaitcountIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "oplog", "acquireWaitCount", "w"}
	metricName := "mongodb.locks.oplog.acquirewaitcount.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksOplogAcquirewaitcountIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksOplogAcquirewaitcountSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "oplog", "acquireWaitCount", "R"}
	metricName := "mongodb.locks.oplog.acquirewaitcount.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksOplogAcquirewaitcountSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksOplogTimeacquiringmicrosIntentExclusiveps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "oplog", "timeAcquiringMicros", "w"}
	metricName := "mongodb.locks.oplog.timeacquiringmicros.intent_exclusiveps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksOplogTimeacquiringmicrosIntentExclusivepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbLocksOplogTimeacquiringmicrosSharedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"locks", "oplog", "timeAcquiringMicros", "R"}
	metricName := "mongodb.locks.oplog.timeacquiringmicros.sharedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbLocksOplogTimeacquiringmicrosSharedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMemBits(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"mem", "bits"}
	metricName := "mongodb.mem.bits"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMemBitsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMemMapped(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"mem", "mapped"}
	metricName := "mongodb.mem.mapped"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMemMappedDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMemMappedwithjournal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"mem", "mappedWithJournal"}
	metricName := "mongodb.mem.mappedwithjournal"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMemMappedwithjournalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMemResident(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"mem", "resident"}
	metricName := "mongodb.mem.resident"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMemResidentDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMemVirtual(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"mem", "virtual"}
	metricName := "mongodb.mem.virtual"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMemVirtualDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsCountFailedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "count", "failed"}
	metricName := "mongodb.metrics.commands.count.failedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsCountFailedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsCountTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "count", "total"}
	metricName := "mongodb.metrics.commands.count.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsCountTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsCreateindexesFailedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "createIndexes", "failed"}
	metricName := "mongodb.metrics.commands.createindexes.failedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsCreateindexesFailedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsCreateindexesTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "createIndexes", "total"}
	metricName := "mongodb.metrics.commands.createindexes.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsCreateindexesTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsDeleteFailedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "delete", "failed"}
	metricName := "mongodb.metrics.commands.delete.failedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsDeleteFailedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsDeleteTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "delete", "total"}
	metricName := "mongodb.metrics.commands.delete.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsDeleteTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsEvalFailedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "eval", "failed"}
	metricName := "mongodb.metrics.commands.eval.failedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsEvalFailedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsEvalTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "eval", "total"}
	metricName := "mongodb.metrics.commands.eval.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsEvalTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsFindandmodifyFailedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "findAndModify", "failed"}
	metricName := "mongodb.metrics.commands.findandmodify.failedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsFindandmodifyFailedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsFindandmodifyTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "findAndModify", "total"}
	metricName := "mongodb.metrics.commands.findandmodify.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsFindandmodifyTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsInsertFailedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "insert", "failed"}
	metricName := "mongodb.metrics.commands.insert.failedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsInsertFailedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsInsertTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "insert", "total"}
	metricName := "mongodb.metrics.commands.insert.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsInsertTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsUpdateFailedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "update", "failed"}
	metricName := "mongodb.metrics.commands.update.failedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsUpdateFailedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCommandsUpdateTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "commands", "update", "total"}
	metricName := "mongodb.metrics.commands.update.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCommandsUpdateTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCursorOpenNotimeout(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "cursor", "open", "noTimeout"}
	metricName := "mongodb.metrics.cursor.open.notimeout"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCursorOpenNotimeoutDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCursorOpenPinned(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "cursor", "open", "pinned"}
	metricName := "mongodb.metrics.cursor.open.pinned"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCursorOpenPinnedDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCursorOpenTotal(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "cursor", "open", "total"}
	metricName := "mongodb.metrics.cursor.open.total"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCursorOpenTotalDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsCursorTimedoutps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "cursor", "timedOut"}
	metricName := "mongodb.metrics.cursor.timedoutps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsCursorTimedoutpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsDocumentDeletedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "document", "deleted"}
	metricName := "mongodb.metrics.document.deletedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsDocumentDeletedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsDocumentInsertedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "document", "inserted"}
	metricName := "mongodb.metrics.document.insertedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsDocumentInsertedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsDocumentReturnedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "document", "returned"}
	metricName := "mongodb.metrics.document.returnedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsDocumentReturnedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsDocumentUpdatedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "document", "updated"}
	metricName := "mongodb.metrics.document.updatedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsDocumentUpdatedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsGetlasterrorWtimeNumps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "getLastError", "wtime", "num"}
	metricName := "mongodb.metrics.getlasterror.wtime.numps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsGetlasterrorWtimeNumpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsGetlasterrorWtimeTotalmillisps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "getLastError", "wtime", "totalMillis"}
	metricName := "mongodb.metrics.getlasterror.wtime.totalmillisps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsGetlasterrorWtimeTotalmillispsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsGetlasterrorWtimeoutsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "getLastError", "wtimeouts"}
	metricName := "mongodb.metrics.getlasterror.wtimeoutsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsGetlasterrorWtimeoutspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsOperationFastmodps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "operation", "fastmod"}
	metricName := "mongodb.metrics.operation.fastmodps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsOperationFastmodpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsOperationIdhackps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "operation", "idhack"}
	metricName := "mongodb.metrics.operation.idhackps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsOperationIdhackpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsOperationScanandorderps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "operation", "scanAndOrder"}
	metricName := "mongodb.metrics.operation.scanandorderps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsOperationScanandorderpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsOperationWriteconflictsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "operation", "writeConflicts"}
	metricName := "mongodb.metrics.operation.writeconflictsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsOperationWriteconflictspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsQueryexecutorScannedobjectsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "queryExecutor", "scannedObjects"}
	metricName := "mongodb.metrics.queryexecutor.scannedobjectsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsQueryexecutorScannedobjectspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsQueryexecutorScannedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "queryExecutor", "scanned"}
	metricName := "mongodb.metrics.queryexecutor.scannedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsQueryexecutorScannedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsRecordMovesps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "record", "moves"}
	metricName := "mongodb.metrics.record.movesps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsRecordMovespsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplApplyBatchesNumps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "apply", "batches", "num"}
	metricName := "mongodb.metrics.repl.apply.batches.numps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplApplyBatchesNumpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplApplyBatchesTotalmillisps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "apply", "batches", "totalMillis"}
	metricName := "mongodb.metrics.repl.apply.batches.totalmillisps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplApplyBatchesTotalmillispsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplApplyOpsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "apply", "ops"}
	metricName := "mongodb.metrics.repl.apply.opsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplApplyOpspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplBufferCount(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "buffer", "count"}
	metricName := "mongodb.metrics.repl.buffer.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplBufferCountDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplBufferMaxsizebytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "buffer", "maxSizeBytes"}
	metricName := "mongodb.metrics.repl.buffer.maxsizebytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplBufferMaxsizebytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplBufferSizebytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "buffer", "sizeBytes"}
	metricName := "mongodb.metrics.repl.buffer.sizebytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplBufferSizebytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplNetworkBytesps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "network", "bytes"}
	metricName := "mongodb.metrics.repl.network.bytesps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplNetworkBytespsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplNetworkGetmoresNumps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "network", "getmores", "num"}
	metricName := "mongodb.metrics.repl.network.getmores.numps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplNetworkGetmoresNumpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplNetworkGetmoresTotalmillisps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "network", "getmores", "totalMillis"}
	metricName := "mongodb.metrics.repl.network.getmores.totalmillisps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplNetworkGetmoresTotalmillispsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplNetworkOpsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "network", "ops"}
	metricName := "mongodb.metrics.repl.network.opsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplNetworkOpspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplNetworkReaderscreatedps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "network", "readersCreated"}
	metricName := "mongodb.metrics.repl.network.readerscreatedps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplNetworkReaderscreatedpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplPreloadDocsNumps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "preload", "docs", "num"}
	metricName := "mongodb.metrics.repl.preload.docs.numps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplPreloadDocsNumpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplPreloadDocsTotalmillisps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "preload", "docs", "totalMillis"}
	metricName := "mongodb.metrics.repl.preload.docs.totalmillisps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplPreloadDocsTotalmillispsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplPreloadIndexesNumps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "preload", "indexes", "num"}
	metricName := "mongodb.metrics.repl.preload.indexes.numps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplPreloadIndexesNumpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsReplPreloadIndexesTotalmillisps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "repl", "preload", "indexes", "totalMillis"}
	metricName := "mongodb.metrics.repl.preload.indexes.totalmillisps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsReplPreloadIndexesTotalmillispsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsTtlDeleteddocumentsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "ttl", "deletedDocuments"}
	metricName := "mongodb.metrics.ttl.deleteddocumentsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsTTLDeleteddocumentspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbMetricsTtlPassesps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"metrics", "ttl", "passes"}
	metricName := "mongodb.metrics.ttl.passesps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbMetricsTTLPassespsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbNetworkBytesinps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"network", "bytesIn"}
	metricName := "mongodb.network.bytesinps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbNetworkBytesinpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbNetworkBytesoutps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"network", "bytesOut"}
	metricName := "mongodb.network.bytesoutps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbNetworkBytesoutpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbNetworkNumrequestsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"network", "numRequests"}
	metricName := "mongodb.network.numrequestsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbNetworkNumrequestspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersCommandps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcounters", "command"}
	metricName := "mongodb.opcounters.commandps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersCommandpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersDeleteps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcounters", "delete"}
	metricName := "mongodb.opcounters.deleteps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersDeletepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersGetmoreps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcounters", "getmore"}
	metricName := "mongodb.opcounters.getmoreps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersGetmorepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersInsertps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcounters", "insert"}
	metricName := "mongodb.opcounters.insertps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersInsertpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersQueryps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcounters", "query"}
	metricName := "mongodb.opcounters.queryps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersQuerypsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersUpdateps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcounters", "update"}
	metricName := "mongodb.opcounters.updateps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersUpdatepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersreplCommandps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcountersRepl", "command"}
	metricName := "mongodb.opcountersrepl.commandps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersreplCommandpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersreplDeleteps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcountersRepl", "delete"}
	metricName := "mongodb.opcountersrepl.deleteps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersreplDeletepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersreplGetmoreps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcountersRepl", "getmore"}
	metricName := "mongodb.opcountersrepl.getmoreps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersreplGetmorepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersreplInsertps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcountersRepl", "insert"}
	metricName := "mongodb.opcountersrepl.insertps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersreplInsertpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersreplQueryps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcountersRepl", "query"}
	metricName := "mongodb.opcountersrepl.queryps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersreplQuerypsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOpcountersreplUpdateps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opcountersRepl", "update"}
	metricName := "mongodb.opcountersrepl.updateps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOpcountersreplUpdatepsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOplatenciesCommandsLatency(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opLatencies", "commands", "latency"}
	metricName := "mongodb.oplatencies.commands.latency"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOplatenciesCommandsLatencyDataPoint(now, val, database)
	s.mb.RecordMongodbOplatenciesCommandsLatencypsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOplatenciesReadsLatency(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opLatencies", "reads", "latency"}
	metricName := "mongodb.oplatencies.reads.latency"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOplatenciesReadsLatencyDataPoint(now, val, database)
	s.mb.RecordMongodbOplatenciesReadsLatencypsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOplatenciesWritesLatency(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"opLatencies", "writes", "latency"}
	metricName := "mongodb.oplatencies.writes.latency"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOplatenciesWritesLatencyDataPoint(now, val, database)
	s.mb.RecordMongodbOplatenciesWritesLatencypsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOplogLogsizemb(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"logSizeMb"}
	metricName := "mongodb.oplog.logsizemb"
	value := doc[metricPath[0]]
	val, ok := value.(float64)
	if !ok {
		err := fmt.Errorf("could not parse value as float")
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOplogLogsizembDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOplogTimediff(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"timeDiff"}
	metricName := "mongodb.oplog.timediff"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOplogTimediffDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbOplogUsedsizemb(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"usedSizeMb"}
	metricName := "mongodb.oplog.usedsizemb"
	value := doc[metricPath[0]]
	val, ok := value.(float64)
	if !ok {
		err := fmt.Errorf("could not parse value as float")
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbOplogUsedsizembDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbReplsetHealth(now pcommon.Timestamp, doc bson.M, database string, replset string, member_name string, member_id string, member_state string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"health"}
	metricName := "mongodb.replset.health"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, replset, member_name, member_id, member_state)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbReplsetHealthDataPoint(now, val, database, replset, member_name, member_id, member_state)
}

func (s *mongodbScraper) recordMongodbReplsetOptimeLag(now pcommon.Timestamp, doc bson.M, database string, replset string, member_name string, member_id string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"optimeLag"}
	metricName := "mongodb.replset.optime_lag"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, replset, member_name, member_id)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbReplsetOptimeLagDataPoint(now, val, database, replset, member_name, member_id)
}

func (s *mongodbScraper) recordMongodbReplsetReplicationlag(now pcommon.Timestamp, doc bson.M, database string, replset string, member_name string, member_id string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"replicationLag"}
	metricName := "mongodb.replset.replicationlag"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, replset, member_name, member_id)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbReplsetReplicationlagDataPoint(now, val, database, replset, member_name, member_id)
}

func (s *mongodbScraper) recordMongodbReplsetState(now pcommon.Timestamp, doc bson.M, database string, replset string, member_name string, member_id string, member_state string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"state"}
	metricName := "mongodb.replset.state"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, replset, member_name, member_id, member_state)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbReplsetStateDataPoint(now, val, database, replset, member_name, member_id, member_state)
}

func (s *mongodbScraper) recordMongodbReplsetVotefraction(now pcommon.Timestamp, doc bson.M, database string, replset string, member_name string, member_id string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"voteFraction"}
	metricName := "mongodb.replset.votefraction"
	value := doc[metricPath[0]]
	val, ok := value.(float64)
	if !ok {
		err := fmt.Errorf("could not parse value as float")
		attributes := fmt.Sprint(database, replset, member_name, member_id)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbReplsetVotefractionDataPoint(now, val, database, replset, member_name, member_id)
}

func (s *mongodbScraper) recordMongodbReplsetVotes(now pcommon.Timestamp, doc bson.M, database string, replset string, member_name string, member_id string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"votes"}
	metricName := "mongodb.replset.votes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, replset, member_name, member_id)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbReplsetVotesDataPoint(now, val, database, replset, member_name, member_id)
}

func (s *mongodbScraper) recordMongodbStatsAvgobjsize(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"avgObjSize"}
	metricName := "mongodb.stats.avgobjsize"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbStatsAvgobjsizeDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbStatsCollections(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"collections"}
	metricName := "mongodb.stats.collections"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbStatsCollectionsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbStatsDatasize(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dataSize"}
	metricName := "mongodb.stats.datasize"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbStatsDatasizeDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbStatsFilesize(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	// Mongo version 4.4+ no longer returns filesize since it is part of the obsolete MMAPv1
	mongo44, _ := version.NewVersion("4.4")
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo44) {
		metricPath := []string{"fileSize"}
		metricName := "mongodb.stats.filesize"
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
			return
		}
		s.mb.RecordMongodbStatsFilesizeDataPoint(now, val, database)
	}
}

func (s *mongodbScraper) recordMongodbStatsIndexes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexes"}
	metricName := "mongodb.stats.indexes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbStatsIndexesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbStatsIndexsize(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexSize"}
	metricName := "mongodb.stats.indexsize"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbStatsIndexsizeDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbStatsNumextents(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	// Mongo version 4.4+ no longer returns numExtents since it is part of the obsolete MMAPv1
	// https://www.mongodb.com/docs/manual/release-notes/4.4-compatibility/#mmapv1-cleanup
	mongo44, _ := version.NewVersion("4.4")
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo44) {
		metricPath := []string{"numExtents"}
		metricName := "mongodb.stats.numextents"
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
			return
		}
		s.mb.RecordMongodbStatsNumextentsDataPoint(now, val, database)
	}
}

func (s *mongodbScraper) recordMongodbStatsObjects(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"objects"}
	metricName := "mongodb.stats.objects"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbStatsObjectsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbStatsStoragesize(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageSize"}
	metricName := "mongodb.stats.storagesize"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbStatsStoragesizeDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocGenericCurrentAllocatedBytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "generic", "current_allocated_bytes"}
	metricName := "mongodb.tcmalloc.generic.current_allocated_bytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocGenericCurrentAllocatedBytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocGenericHeapSize(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "generic", "heap_size"}
	metricName := "mongodb.tcmalloc.generic.heap_size"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocGenericHeapSizeDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocTcmallocAggressiveMemoryDecommit(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "tcmalloc", "aggressive_memory_decommit"}
	metricName := "mongodb.tcmalloc.tcmalloc.aggressive_memory_decommit"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocTcmallocAggressiveMemoryDecommitDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocTcmallocCentralCacheFreeBytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "tcmalloc", "central_cache_free_bytes"}
	metricName := "mongodb.tcmalloc.tcmalloc.central_cache_free_bytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocTcmallocCentralCacheFreeBytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocTcmallocCurrentTotalThreadCacheBytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "tcmalloc", "current_total_thread_cache_bytes"}
	metricName := "mongodb.tcmalloc.tcmalloc.current_total_thread_cache_bytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocTcmallocCurrentTotalThreadCacheBytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocTcmallocMaxTotalThreadCacheBytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "tcmalloc", "max_total_thread_cache_bytes"}
	metricName := "mongodb.tcmalloc.tcmalloc.max_total_thread_cache_bytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocTcmallocMaxTotalThreadCacheBytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocTcmallocPageheapFreeBytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "tcmalloc", "pageheap_free_bytes"}
	metricName := "mongodb.tcmalloc.tcmalloc.pageheap_free_bytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocTcmallocPageheapFreeBytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocTcmallocPageheapUnmappedBytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "tcmalloc", "pageheap_unmapped_bytes"}
	metricName := "mongodb.tcmalloc.tcmalloc.pageheap_unmapped_bytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocTcmallocPageheapUnmappedBytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocTcmallocSpinlockTotalDelayNs(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "tcmalloc", "spinlock_total_delay_ns"}
	metricName := "mongodb.tcmalloc.tcmalloc.spinlock_total_delay_ns"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocTcmallocSpinlockTotalDelayNsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocTcmallocThreadCacheFreeBytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "tcmalloc", "thread_cache_free_bytes"}
	metricName := "mongodb.tcmalloc.tcmalloc.thread_cache_free_bytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocTcmallocThreadCacheFreeBytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbTcmallocTcmallocTransferCacheFreeBytes(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"tcmalloc", "tcmalloc", "transfer_cache_free_bytes"}
	metricName := "mongodb.tcmalloc.tcmalloc.transfer_cache_free_bytes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbTcmallocTcmallocTransferCacheFreeBytesDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbUsageCommandsCount(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"commands", "count"}
	metricName := "mongodb.usage.commands.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageCommandsCountDataPoint(now, val, database, collection)
	s.mb.RecordMongodbUsageCommandsCountpsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageCommandsTime(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"commands", "time"}
	metricName := "mongodb.usage.commands.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageCommandsTimeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageGetmoreCount(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"getmore", "count"}
	metricName := "mongodb.usage.getmore.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageGetmoreCountDataPoint(now, val, database, collection)
	s.mb.RecordMongodbUsageGetmoreCountpsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageGetmoreTime(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"getmore", "time"}
	metricName := "mongodb.usage.getmore.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageGetmoreTimeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageInsertCount(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"insert", "count"}
	metricName := "mongodb.usage.insert.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageInsertCountDataPoint(now, val, database, collection)
	s.mb.RecordMongodbUsageInsertCountpsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageInsertTime(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"insert", "time"}
	metricName := "mongodb.usage.insert.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageInsertTimeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageQueriesCount(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"queries", "count"}
	metricName := "mongodb.usage.queries.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageQueriesCountDataPoint(now, val, database, collection)
	s.mb.RecordMongodbUsageQueriesCountpsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageQueriesTime(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"queries", "time"}
	metricName := "mongodb.usage.queries.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageQueriesTimeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageReadlockCount(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"readLock", "count"}
	metricName := "mongodb.usage.readlock.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageReadlockCountDataPoint(now, val, database, collection)
	s.mb.RecordMongodbUsageReadlockCountpsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageReadlockTime(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"readLock", "time"}
	metricName := "mongodb.usage.readlock.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageReadlockTimeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageRemoveCount(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"remove", "count"}
	metricName := "mongodb.usage.remove.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageRemoveCountDataPoint(now, val, database, collection)
	s.mb.RecordMongodbUsageRemoveCountpsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageRemoveTime(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"remove", "time"}
	metricName := "mongodb.usage.remove.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageRemoveTimeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageTotalCount(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"total", "count"}
	metricName := "mongodb.usage.total.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageTotalCountDataPoint(now, val, database, collection)
	s.mb.RecordMongodbUsageTotalCountpsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageTotalTime(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"total", "time"}
	metricName := "mongodb.usage.total.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageTotalTimeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageUpdateCount(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"update", "count"}
	metricName := "mongodb.usage.update.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageUpdateCountDataPoint(now, val, database, collection)
	s.mb.RecordMongodbUsageUpdateCountpsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageUpdateTime(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"update", "time"}
	metricName := "mongodb.usage.update.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageUpdateTimeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageWritelockCount(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"writeLock", "count"}
	metricName := "mongodb.usage.writelock.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageWritelockCountDataPoint(now, val, database, collection)
	s.mb.RecordMongodbUsageWritelockCountpsDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbUsageWritelockTime(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"writeLock", "time"}
	metricName := "mongodb.usage.writelock.time"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		attributes := fmt.Sprint(database, collection)
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, attributes, err))
		return
	}
	s.mb.RecordMongodbUsageWritelockTimeDataPoint(now, val, database, collection)
}

func (s *mongodbScraper) recordMongodbWiredtigerCacheBytesCurrentlyInCache(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "bytes currently in the cache"}
	metricName := "mongodb.wiredtiger.cache.bytes_currently_in_cache"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCacheBytesCurrentlyInCacheDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCacheFailedEvictionOfPagesExceedingTheInMemoryMaximumps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "failed eviction of pages that exceeded the in-memory maximum"}
	metricName := "mongodb.wiredtiger.cache.failed_eviction_of_pages_exceeding_the_in_memory_maximumps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCacheFailedEvictionOfPagesExceedingTheInMemoryMaximumpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCacheInMemoryPageSplits(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "in-memory page splits"}
	metricName := "mongodb.wiredtiger.cache.in_memory_page_splits"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCacheInMemoryPageSplitsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCacheMaximumBytesConfigured(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "maximum bytes configured"}
	metricName := "mongodb.wiredtiger.cache.maximum_bytes_configured"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCacheMaximumBytesConfiguredDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCacheMaximumPageSizeAtEviction(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "maximum page size seen at eviction"}
	metricName := "mongodb.wiredtiger.cache.maximum_page_size_at_eviction"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCacheMaximumPageSizeAtEvictionDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCacheModifiedPagesEvicted(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "modified pages evicted"}
	metricName := "mongodb.wiredtiger.cache.modified_pages_evicted"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCacheModifiedPagesEvictedDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCachePagesCurrentlyHeldInCache(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "pages currently held in the cache"}
	metricName := "mongodb.wiredtiger.cache.pages_currently_held_in_cache"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCachePagesCurrentlyHeldInCacheDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCachePagesEvictedByApplicationThreadsps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "pages evicted by application threads"}
	metricName := "mongodb.wiredtiger.cache.pages_evicted_by_application_threadsps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCachePagesEvictedByApplicationThreadspsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCachePagesEvictedExceedingTheInMemoryMaximumps(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "pages evicted because they exceeded the in-memory maximum"}
	metricName := "mongodb.wiredtiger.cache.pages_evicted_exceeding_the_in_memory_maximumps"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCachePagesEvictedExceedingTheInMemoryMaximumpsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCachePagesReadIntoCache(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "pages read into cache"}
	metricName := "mongodb.wiredtiger.cache.pages_read_into_cache"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCachePagesReadIntoCacheDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCachePagesWrittenFromCache(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "pages written from cache"}
	metricName := "mongodb.wiredtiger.cache.pages_written_from_cache"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCachePagesWrittenFromCacheDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCacheTrackedDirtyBytesInCache(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "tracked dirty bytes in the cache"}
	metricName := "mongodb.wiredtiger.cache.tracked_dirty_bytes_in_cache"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCacheTrackedDirtyBytesInCacheDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerCacheUnmodifiedPagesEvicted(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "cache", "unmodified pages evicted"}
	metricName := "mongodb.wiredtiger.cache.unmodified_pages_evicted"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerCacheUnmodifiedPagesEvictedDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerConcurrenttransactionsReadAvailable(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "concurrentTransactions", "read", "available"}
	metricName := "mongodb.wiredtiger.concurrenttransactions.read.available"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerConcurrenttransactionsReadAvailableDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerConcurrenttransactionsReadOut(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "concurrentTransactions", "read", "out"}
	metricName := "mongodb.wiredtiger.concurrenttransactions.read.out"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerConcurrenttransactionsReadOutDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerConcurrenttransactionsReadTotaltickets(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "concurrentTransactions", "read", "totalTickets"}
	metricName := "mongodb.wiredtiger.concurrenttransactions.read.totaltickets"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerConcurrenttransactionsReadTotalticketsDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerConcurrenttransactionsWriteAvailable(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "concurrentTransactions", "write", "available"}
	metricName := "mongodb.wiredtiger.concurrenttransactions.write.available"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerConcurrenttransactionsWriteAvailableDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerConcurrenttransactionsWriteOut(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "concurrentTransactions", "write", "out"}
	metricName := "mongodb.wiredtiger.concurrenttransactions.write.out"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerConcurrenttransactionsWriteOutDataPoint(now, val, database)
}

func (s *mongodbScraper) recordMongodbWiredtigerConcurrenttransactionsWriteTotaltickets(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	metricPath := []string{"wiredTiger", "concurrentTransactions", "write", "totalTickets"}
	metricName := "mongodb.wiredtiger.concurrenttransactions.write.totaltickets"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, database, err))
		return
	}
	s.mb.RecordMongodbWiredtigerConcurrenttransactionsWriteTotalticketsDataPoint(now, val, database)
}

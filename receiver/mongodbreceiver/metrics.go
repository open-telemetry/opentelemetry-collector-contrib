// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

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
	s.mb.RecordMongodbCollectionCountDataPoint(now, val)
}

func (s *mongodbScraper) recordDataSize(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"dataSize"}
	metricName := "mongodb.data.size"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	s.mb.RecordMongodbDataSizeDataPoint(now, val)
}

func (s *mongodbScraper) recordStorageSize(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"storageSize"}
	metricName := "mongodb.storage.size"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	s.mb.RecordMongodbStorageSizeDataPoint(now, val)
}

func (s *mongodbScraper) recordObjectCount(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"objects"}
	metricName := "mongodb.object.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	s.mb.RecordMongodbObjectCountDataPoint(now, val)
}

func (s *mongodbScraper) recordIndexCount(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexes"}
	metricName := "mongodb.index.count"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	s.mb.RecordMongodbIndexCountDataPoint(now, val)
}

func (s *mongodbScraper) recordIndexSize(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"indexSize"}
	metricName := "mongodb.index.size"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, dbName, err))
		return
	}
	s.mb.RecordMongodbIndexSizeDataPoint(now, val)
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
		s.mb.RecordMongodbExtentCountDataPoint(now, val)
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
		s.mb.RecordMongodbConnectionCountDataPoint(now, val, ct)
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
		s.mb.RecordMongodbMemoryUsageDataPoint(now, memUsageBytes, mt)
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
		s.mb.RecordMongodbDocumentOperationCountDataPoint(now, val, metadataKey)
	}
}

func (s *mongodbScraper) recordSessionCount(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine for session count"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric cannot be collected
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
	currentCounts := make(map[string]int64)

	for operationVal, operation := range metadata.MapAttributeOperation {
		metricPath := []string{"opcounters", operationVal}
		metricName := "mongodb.operation.count"
		val, err := collectMetric(doc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, operationVal, err))
			continue
		}

		s.mb.RecordMongodbOperationCountDataPoint(now, val, operation)

		currentCounts[operationVal] = val
		s.recordOperationPerSecond(now, operationVal, val)
	}

	// For telegraf metrics to get QPS for opcounters
	// Store current counts for next iteration
	s.prevCounts = currentCounts
	s.prevTimestamp = now
}

func (s *mongodbScraper) recordOperationsRepl(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	replDoc := doc
	var highestInsertCount int64 = -1

	if len(s.secondaryClients) > 0 {
		ctx := context.Background()
		for _, secondaryClient := range s.secondaryClients {
			status, err := secondaryClient.ServerStatus(ctx, "admin")
			if err != nil {
				s.logger.Debug("Failed to get secondary server status", zap.Error(err))
				continue
			}

			if opcountersRepl, ok := status["opcountersRepl"].(bson.M); ok {
				if insertCount, ok := opcountersRepl["insert"].(int64); ok {
					if insertCount > highestInsertCount {
						highestInsertCount = insertCount
						replDoc = status
					}
				}
			}
		}
	}

	currentCounts := make(map[string]int64)
	for operationVal, operation := range metadata.MapAttributeOperation {
		metricPath := []string{"opcountersRepl", operationVal}
		metricName := "mongodb.operation.repl.count"
		val, err := collectMetric(replDoc, metricPath)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, operationVal, err))
			continue
		}
		s.mb.RecordMongodbOperationReplCountDataPoint(now, val, operation)

		currentCounts[operationVal] = val
		s.recordReplOperationPerSecond(now, operationVal, val)
	}

	s.prevReplCounts = currentCounts
	s.prevReplTimestamp = now
}

func (s *mongodbScraper) recordReplOperationPerSecond(now pcommon.Timestamp, operationVal string, currentCount int64) {
	if s.prevReplTimestamp > 0 {
		timeDelta := float64(now-s.prevReplTimestamp) / 1e9
		if timeDelta > 0 {
			if prevReplCount, exists := s.prevReplCounts[operationVal]; exists {
				delta := currentCount - prevReplCount
				queriesPerSec := float64(delta) / timeDelta

				switch operationVal {
				case "query":
					s.mb.RecordMongodbReplQueriesPerSecDataPoint(now, queriesPerSec)
				case "insert":
					s.mb.RecordMongodbReplInsertsPerSecDataPoint(now, queriesPerSec)
				case "command":
					s.mb.RecordMongodbReplCommandsPerSecDataPoint(now, queriesPerSec)
				case "getmore":
					s.mb.RecordMongodbReplGetmoresPerSecDataPoint(now, queriesPerSec)
				case "delete":
					s.mb.RecordMongodbReplDeletesPerSecDataPoint(now, queriesPerSec)
				case "update":
					s.mb.RecordMongodbReplUpdatesPerSecDataPoint(now, queriesPerSec)
				default:
					fmt.Printf("Unhandled repl operation: %s\n", operationVal)
				}
			}
		}
	}
}

func (s *mongodbScraper) recordFlushesPerSecond(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"wiredTiger", "checkpoint", "total succeed number of checkpoints"}
	metricName := "mongodb.flushes.rate"
	currentFlushes, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}

	if s.prevFlushTimestamp > 0 {
		timeDelta := float64(now-s.prevFlushTimestamp) / 1e9
		if timeDelta > 0 {
			if prevFlushCount := s.prevFlushCount; true {
				delta := currentFlushes - prevFlushCount
				flushesPerSec := float64(delta) / timeDelta
				s.mb.RecordMongodbFlushesRateDataPoint(now, flushesPerSec)
			}
		}
	}

	s.prevFlushCount = currentFlushes
	s.prevFlushTimestamp = now
}

func (s *mongodbScraper) recordOperationPerSecond(now pcommon.Timestamp, operationVal string, currentCount int64) {
	if s.prevTimestamp > 0 {
		timeDelta := float64(now-s.prevTimestamp) / 1e9
		if timeDelta > 0 {
			if prevCount, exists := s.prevCounts[operationVal]; exists {
				delta := currentCount - prevCount
				queriesPerSec := float64(delta) / timeDelta

				switch operationVal {
				case "query":
					s.mb.RecordMongodbQueriesRateDataPoint(now, queriesPerSec)
				case "insert":
					s.mb.RecordMongodbInsertsRateDataPoint(now, queriesPerSec)
				case "command":
					s.mb.RecordMongodbCommandsRateDataPoint(now, queriesPerSec)
				case "getmore":
					s.mb.RecordMongodbGetmoresRateDataPoint(now, queriesPerSec)
				case "delete":
					s.mb.RecordMongodbDeletesRateDataPoint(now, queriesPerSec)
				case "update":
					s.mb.RecordMongodbUpdatesRateDataPoint(now, queriesPerSec)
				default:
					fmt.Printf("Unhandled operation: %s\n", operationVal)
				}
			}
		}
	}
}

func (s *mongodbScraper) recordActiveWrites(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "activeClients", "writers"}
	metricName := "mongodb.active.writes"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	s.mb.RecordMongodbActiveWritesDataPoint(now, val)
}

func (s *mongodbScraper) recordActiveReads(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"globalLock", "activeClients", "readers"}
	metricName := "mongodb.active.reads"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	s.mb.RecordMongodbActiveReadsDataPoint(now, val)
}

func (s *mongodbScraper) recordWTCacheBytes(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"wiredTiger", "cache", "bytes read into cache"}
	metricName := "mongodb.wtcache.bytes.read"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	s.mb.RecordMongodbWtcacheBytesReadDataPoint(now, val)
}

func (s *mongodbScraper) recordPageFaults(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	metricPath := []string{"extra_info", "page_faults"}
	metricName := "mongodb.page_faults"
	val, err := collectMetric(doc, metricPath)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf(collectMetricError, metricName, err))
		return
	}
	s.mb.RecordMongodbPageFaultsDataPoint(now, val)
}

func (s *mongodbScraper) recordCacheOperations(now pcommon.Timestamp, doc bson.M, errs *scrapererror.ScrapeErrors) {
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		errs.AddPartial(1, errors.New("failed to find storage engine for cache operations"))
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric cannot be collected
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
			s.mb.RecordMongodbLockAcquireCountDataPoint(now, val, lockTypeAttribute, lockModeAttribute)
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
			s.mb.RecordMongodbLockAcquireWaitCountDataPoint(now, val, lockTypeAttribute, lockModeAttribute)
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
			s.mb.RecordMongodbLockAcquireTimeDataPoint(now, val, lockTypeAttribute, lockModeAttribute)
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
			s.mb.RecordMongodbLockDeadlockCountDataPoint(now, val, lockTypeAttribute, lockModeAttribute)
		}
	}
}

// Index Stats
func (s *mongodbScraper) recordIndexAccess(now pcommon.Timestamp, documents []bson.M, dbName string, collectionName string, errs *scrapererror.ScrapeErrors) {
	metricName := "mongodb.index.access.count"
	var indexAccessTotal int64
	for _, doc := range documents {
		metricAttributes := fmt.Sprintf("%s, %s", dbName, collectionName)
		indexAccess, err := dig(doc, []string{"accesses", "ops"})
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, errors.New("could not find key for index access metric")))
			return
		}
		indexAccessValue, err := parseInt(indexAccess)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf(collectMetricWithAttributes, metricName, metricAttributes, err))
			return
		}
		indexAccessTotal += indexAccessValue
	}
	s.mb.RecordMongodbIndexAccessCountDataPoint(now, indexAccessTotal, collectionName)
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
	docTotals, err := dig(document, []string{"totals"})
	if err != nil {
		return nil, err
	}
	docTotalsMap, ok := docTotals.(bson.D)
	if !ok {
		return nil, fmt.Errorf("expected bson.D, got %T", docTotals)
	}

	var collectionPathNames []string
	for _, v := range docTotalsMap {
		if v.Key != "note" {
			collectionPathNames = append(collectionPathNames, v.Key)
		}
	}
	return collectionPathNames, nil
}

func collectMetric(document bson.M, path []string) (int64, error) {
	metric, err := dig(document, path)
	if err != nil {
		return 0, err
	}
	return parseInt(metric)
}

func dig(document bson.M, path []string) (any, error) {
	if len(path) == 0 {
		return nil, errKeyNotFound
	}
	curItem, remainingPath := path[0], path[1:]
	value := document[curItem]
	if value == nil {
		return 0, errKeyNotFound
	}
	if len(remainingPath) == 0 {
		return value, nil
	}
	if value, ok := value.(bson.M); ok {
		return dig(value, remainingPath)
	}
	if value, ok := value.(bson.D); ok {
		return digBsonD(value, remainingPath)
	}
	return nil, fmt.Errorf("expected bson.M, got %T", value)
}

func digBsonD(value bson.D, remainingPath []string) (any, error) {
	if len(remainingPath) == 0 {
		return value, nil
	}
	curItem, remainingPath := remainingPath[0], remainingPath[1:]
	for _, v := range value {
		if v.Key == curItem {
			if len(remainingPath) == 0 {
				return v.Value, nil
			}
			if value, ok := v.Value.(bson.D); ok {
				return digBsonD(value, remainingPath)
			}
			if value, ok := v.Value.(bson.M); ok {
				return dig(value, remainingPath)
			}
			return nil, fmt.Errorf("expected bson.M or bson.D, got %T", v.Value)
		}
	}
	return nil, errKeyNotFound
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
	default:
		return 0, fmt.Errorf("could not parse value as int: %v", reflect.TypeOf(val))
	}
}

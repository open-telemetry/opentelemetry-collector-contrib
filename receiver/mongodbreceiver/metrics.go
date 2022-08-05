// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

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

// DBStats
func (s *mongodbScraper) recordCollections(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	collectionsPath := []string{"collections"}
	collections, err := dig(doc, collectionsPath)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	collectionsVal, err := parseInt(collections)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	s.mb.RecordMongodbCollectionCountDataPoint(now, collectionsVal, dbName)
}

func (s *mongodbScraper) recordDataSize(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	dataSizePath := []string{"dataSize"}
	dataSize, err := dig(doc, dataSizePath)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	dataSizeVal, err := parseInt(dataSize)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	s.mb.RecordMongodbDataSizeDataPoint(now, dataSizeVal, dbName)
}

func (s *mongodbScraper) recordStorageSize(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	storageSizePath := []string{"storageSize"}
	storageSize, err := dig(doc, storageSizePath)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	storageSizeValue, err := parseInt(storageSize)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	s.mb.RecordMongodbStorageSizeDataPoint(now, storageSizeValue, dbName)
}

func (s *mongodbScraper) recordObjectCount(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	objectsPath := []string{"objects"}
	objects, err := dig(doc, objectsPath)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	objectsVal, err := parseInt(objects)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	s.mb.RecordMongodbObjectCountDataPoint(now, objectsVal, dbName)
}

func (s *mongodbScraper) recordIndexCount(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	indexesPath := []string{"indexes"}
	indexes, err := dig(doc, indexesPath)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	indexesVal, err := parseInt(indexes)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	s.mb.RecordMongodbIndexCountDataPoint(now, indexesVal, dbName)
}

func (s *mongodbScraper) recordIndexSize(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	indexSizePath := []string{"indexSize"}
	indexSize, err := dig(doc, indexSizePath)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	indexSizeVal, err := parseInt(indexSize)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	s.mb.RecordMongodbIndexSizeDataPoint(now, indexSizeVal, dbName)
}

func (s *mongodbScraper) recordExtentCount(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	extentsPath := []string{"numExtents"}
	extents, err := dig(doc, extentsPath)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	extentsVal, err := parseInt(extents)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	s.mb.RecordMongodbExtentCountDataPoint(now, extentsVal, dbName)
}

// ServerStatus
func (s *mongodbScraper) recordConnections(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	for ctVal, ct := range metadata.MapAttributeConnectionType {
		connKey := []string{"connections", ctVal}
		conn, err := dig(doc, connKey)
		if err != nil {
			errors.AddPartial(1, err)
			continue
		}

		connVal, err := parseInt(conn)
		if err != nil {
			errors.AddPartial(1, err)
			continue
		}
		s.mb.RecordMongodbConnectionCountDataPoint(now, connVal, dbName, ct)
	}
}

func (s *mongodbScraper) recordMemoryUsage(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	for mtVal, mt := range metadata.MapAttributeMemoryType {
		memKey := []string{"mem", mtVal}
		mem, err := dig(doc, memKey)
		if err != nil {
			errors.AddPartial(1, err)
			continue
		}

		memUsageMebi, err := parseInt(mem)
		if err != nil {
			errors.AddPartial(1, err)
			continue
		}
		// convert from mebibytes to bytes
		memUsageBytes := memUsageMebi * int64(1048576)
		s.mb.RecordMongodbMemoryUsageDataPoint(now, memUsageBytes, dbName, mt)
	}
}

func (s *mongodbScraper) recordDocumentOperations(now pcommon.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	// Collect document insert, delete, update
	for operationKey, metadataKey := range documentMap {
		docOperation, err := dig(doc, []string{"metrics", "document", operationKey})
		if err != nil {
			errors.AddPartial(1, err)
			s.logger.Error("failed to find operation", zap.Error(err), zap.String("operation", operationKey))
		}
		docOperationValue, err := parseInt(docOperation)
		if err != nil {
			errors.AddPartial(1, err)
		} else {
			s.mb.RecordMongodbDocumentOperationCountDataPoint(now, docOperationValue, dbName, metadataKey)
		}
	}
}

func (s *mongodbScraper) recordSessionCount(now pcommon.Timestamp, doc bson.M, errors scrapererror.ScrapeErrors) {
	// Collect session count
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		s.logger.Error("failed to find storage engine for session count", zap.Error(err))
		errors.AddPartial(1, err)
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	sessionCount, err := dig(doc, []string{"wiredTiger", "session", "open session count"})
	if err != nil {
		errors.AddPartial(1, err)
		return
	}

	sessionCountValue, err := parseInt(sessionCount)
	if err != nil {
		errors.AddPartial(1, err)
	} else {
		s.mb.RecordMongodbSessionCountDataPoint(now, sessionCountValue)
	}
}

// Admin Stats
func (s *mongodbScraper) recordOperations(now pcommon.Timestamp, doc bson.M, errors scrapererror.ScrapeErrors) {
	// Collect Operations
	for operationVal, operation := range metadata.MapAttributeOperation {
		count, err := dig(doc, []string{"opcounters", operationVal})
		if err != nil {
			errors.AddPartial(1, err)
			s.logger.Error("failed to find operation", zap.Error(err), zap.String("operation", operationVal))
			continue
		}
		countVal, err := parseInt(count)
		if err != nil {
			errors.AddPartial(1, err)
			continue
		}

		s.mb.RecordMongodbOperationCountDataPoint(now, countVal, operation)
	}
}

func (s *mongodbScraper) recordCacheOperations(now pcommon.Timestamp, doc bson.M, errors scrapererror.ScrapeErrors) {
	// Collect Cache Hits & Misses if wiredTiger storage engine is used
	storageEngine, err := dig(doc, []string{"storageEngine", "name"})
	if err != nil {
		s.logger.Error("failed to find storage engine for cache operation", zap.Error(err))
		errors.AddPartial(1, err)
		return
	}
	if storageEngine != "wiredTiger" {
		// mongodb is using a different storage engine and this metric can not be collected
		return
	}

	canCalculateCacheHits := true

	cacheMisses, err := dig(doc, []string{"wiredTiger", "cache", "pages read into cache"})
	if err != nil {
		s.logger.Error("failed to find cache misses", zap.Error(err))
		errors.AddPartial(1, err)
		return
	}

	cacheMissesValue, err := parseInt(cacheMisses)
	if err != nil {
		errors.AddPartial(1, err)
	} else {
		s.mb.RecordMongodbCacheOperationsDataPoint(now, cacheMissesValue, metadata.AttributeTypeMiss)
	}

	tcr, err := dig(doc, []string{"wiredTiger", "cache", "pages requested from the cache"})
	if err != nil {
		errors.AddPartial(1, err)
		s.logger.Debug("failed to parse total cache requests unable to calculate cache hits", zap.Error(err))
		canCalculateCacheHits = false
	}

	totalCacheReqs, err := parseInt(tcr)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}

	if canCalculateCacheHits && totalCacheReqs > cacheMissesValue {
		cacheHits := totalCacheReqs - cacheMissesValue
		s.mb.RecordMongodbCacheOperationsDataPoint(now, cacheHits, metadata.AttributeTypeHit)
	}
}

func (s *mongodbScraper) recordGlobalLockTime(now pcommon.Timestamp, doc bson.M, errors scrapererror.ScrapeErrors) {
	var heldTimeUs int64

	// Mongo version greater than or equal to 4.0 have it in the serverStats at "globalLock", "totalTime"
	// reference: https://docs.mongodb.com/v4.0/reference/command/serverStatus/#server-status-global-lock
	mongo40, _ := version.NewVersion("4.0")
	if s.mongoVersion.GreaterThanOrEqual(mongo40) {
		val, err := dig(doc, []string{"globalLock", "totalTime"})
		if err != nil {
			errors.AddPartial(1, err)
			return
		}
		parsedVal, err := parseInt(val)
		if err != nil {
			errors.AddPartial(1, err)
			return
		}
		heldTimeUs = parsedVal
	} else {
		for _, lockType := range []string{"W", "R", "r", "w"} {
			waitTime, err := dig(doc, []string{"locks", ".", "timeAcquiringMicros", lockType})
			if err != nil {
				continue
			}
			waitTimeVal, err := parseInt(waitTime)
			if err != nil {
				errors.AddPartial(1, err)
			}
			heldTimeUs += waitTimeVal
		}
	}
	if heldTimeUs != 0 {
		htMilliseconds := heldTimeUs / 1000
		s.mb.RecordMongodbGlobalLockTimeDataPoint(now, htMilliseconds)
	}

	errors.AddPartial(1, fmt.Errorf("was unable to calculate global lock time"))
}

func (s *mongodbScraper) recordCursorCount(now pcommon.Timestamp, doc bson.M, errors scrapererror.ScrapeErrors) {
	// Collect cursor count
	cursorCount, err := dig(doc, []string{"metrics", "cursor", "open", "total"})
	if err != nil {
		errors.AddPartial(1, err)
		return
	}

	cursorCountValue, err := parseInt(cursorCount)
	if err != nil {
		errors.AddPartial(1, err)
	}
	s.mb.RecordMongodbCursorCountDataPoint(now, cursorCountValue)
}

func (s *mongodbScraper) recordCursorTimeoutCount(now pcommon.Timestamp, doc bson.M, errors scrapererror.ScrapeErrors) {
	// Collect cursor timeout count
	cursorTimeoutCount, err := dig(doc, []string{"metrics", "cursor", "timedOut"})
	if err != nil {
		errors.AddPartial(1, err)
		return
	}

	cursorTimeoutCountValue, err := parseInt(cursorTimeoutCount)
	if err != nil {
		errors.AddPartial(1, err)
	}
	s.mb.RecordMongodbCursorTimeoutCountDataPoint(now, cursorTimeoutCountValue)
}

func (s *mongodbScraper) recordNetworkCount(now pcommon.Timestamp, doc bson.M, errors scrapererror.ScrapeErrors) {
	// Collect network bytes receive, transmit, and request count
	networkRecorderMap := map[string]func(pcommon.Timestamp, int64){
		"bytesIn":     s.mb.RecordMongodbNetworkIoReceiveDataPoint,
		"bytesOut":    s.mb.RecordMongodbNetworkIoTransmitDataPoint,
		"numRequests": s.mb.RecordMongodbNetworkRequestCountDataPoint,
	}
	for networkKey, recorder := range networkRecorderMap {
		network, err := dig(doc, []string{"network", networkKey})
		if err != nil {
			errors.AddPartial(1, err)
			s.logger.Error("failed to find network", zap.Error(err), zap.String("network", networkKey))
		}
		networkValue, err := parseInt(network)
		if err != nil {
			errors.AddPartial(1, err)
		} else {
			recorder(now, networkValue)
		}
	}
}

// Index Stats
func (s *mongodbScraper) recordIndexAccess(now pcommon.Timestamp, documents []bson.M, dbName string, collectionName string, scraperErrors scrapererror.ScrapeErrors) {
	// Collect the index access given a collection and database if version is >= 3.2
	// https://www.mongodb.com/docs/v3.2/reference/operator/aggregation/indexStats/
	mongo40, _ := version.NewVersion("3.2")
	if s.mongoVersion.GreaterThanOrEqual(mongo40) {
		var indexAccessTotal int64
		for _, doc := range documents {
			indexAccess, ok := doc["accesses"].(bson.M)["ops"]
			if !ok {
				scraperErrors.AddPartial(1, errors.New("could not find key for metric"))
				return
			}
			indexAccessValue, err := parseInt(indexAccess)
			if err != nil {
				scraperErrors.AddPartial(1, err)
				return
			}
			indexAccessTotal += indexAccessValue
		}
		s.mb.RecordMongodbIndexAccessCountDataPoint(now, indexAccessTotal, dbName, collectionName)
	}
}

// Top Stats
func (s *mongodbScraper) recordOperationTime(now pcommon.Timestamp, doc bson.M, scraperErrors scrapererror.ScrapeErrors) {
	// Collect the total operation time
	collectionPathNames, err := digForCollectionPathNames(doc)
	if err != nil {
		scraperErrors.AddPartial(1, err)
		return
	}
	operationTimeValues, err := aggregateOperationTimeValues(doc, collectionPathNames, operationsMap)
	if err != nil {
		scraperErrors.AddPartial(1, err)
		return
	}

	for operationName, metadataOperationName := range operationsMap {
		operationValue, ok := operationTimeValues[operationName]
		if !ok {
			scraperErrors.AddPartial(1, errors.New("could not find key for metric"))
			return
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
		return nil, errors.New("could not find key for metric")
	}
	collectionPathNames := []string{}
	for collectionPathName := range docTotals {
		if collectionPathName != "note" {
			collectionPathNames = append(collectionPathNames, collectionPathName)
		}
	}
	return collectionPathNames, nil
}

func dig(document bson.M, path []string) (interface{}, error) {
	curItem, remainingPath := path[0], path[1:]
	value := document[curItem]
	if value == nil {
		return 0, errors.New("could not find key for metric")
	}
	if len(remainingPath) == 0 {
		return value, nil
	}
	return dig(value.(bson.M), remainingPath)
}

func parseInt(val interface{}) (int64, error) {
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

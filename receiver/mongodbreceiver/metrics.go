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
	types := []string{
		metadata.AttributeConnectionType.Active,
		metadata.AttributeConnectionType.Available,
		metadata.AttributeConnectionType.Current,
	}
	for _, ct := range types {
		connKey := []string{"connections", ct}
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
	types := []string{
		metadata.AttributeMemoryType.Resident,
		metadata.AttributeMemoryType.Virtual,
	}
	for _, mt := range types {
		memKey := []string{"mem", mt}
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

// Admin Stats
func (s *mongodbScraper) recordOperations(now pcommon.Timestamp, doc bson.M, errors scrapererror.ScrapeErrors) {
	// Collect Operations
	for _, operation := range []string{
		metadata.AttributeOperation.Insert,
		metadata.AttributeOperation.Query,
		metadata.AttributeOperation.Update,
		metadata.AttributeOperation.Delete,
		metadata.AttributeOperation.Getmore,
		metadata.AttributeOperation.Command,
	} {
		count, err := dig(doc, []string{"opcounters", operation})
		if err != nil {
			errors.AddPartial(1, err)
			s.logger.Error("failed to find operation", zap.Error(err), zap.String("operation", operation))
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
	// Collect Cache Hits & Misses
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
		s.mb.RecordMongodbCacheOperationsDataPoint(now, cacheMissesValue, metadata.AttributeType.Miss)
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
		s.mb.RecordMongodbCacheOperationsDataPoint(now, cacheHits, metadata.AttributeType.Hit)
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

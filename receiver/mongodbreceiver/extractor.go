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
	"time"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

type extractor struct {
	version *version.Version
	logger  *zap.Logger
}

type extractType int

const (
	adminServerStats extractType = iota
	normalDBStats
	normalServerStats
)

// Mongo26 is a version representation of mongo 2.6
var Mongo26, _ = version.NewVersion("2.6")

// Mongo30 is a version representation of mongo 3.0
var Mongo30, _ = version.NewVersion("3.0")

// Mongo40 is a version representation of mongo 4.0
var Mongo40, _ = version.NewVersion("4.0")

// Mongo42 is a version representation of mongo 4.2
var Mongo42, _ = version.NewVersion("4.2")

// Mongo44 is a version representation of mongo 4.4
var Mongo44, _ = version.NewVersion("4.4")

// Mongo50 is a version representation of mongo 5.0
var Mongo50, _ = version.NewVersion("5.0")

// newExtractor returns a representation for a utility to dig through arbitrary bson
// for metric values
func newExtractor(mongoVersion string, logger *zap.Logger) (*extractor, error) {
	v, err := version.NewVersion(mongoVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to parse version as semantic version %s: %w", mongoVersion, err)
	}
	return &extractor{
		version: v,
		logger:  logger,
	}, nil
}

func (e *extractor) Extract(document bson.M, mb *metadata.MetricsBuilder, dbName string, et extractType) {
	now := pdata.NewTimestampFromTime(time.Now())
	switch et {
	case normalDBStats:
		e.extractDBStats(now, document, mb, dbName)
	case normalServerStats:
		e.extractNormalServerStats(now, document, mb, dbName)
	case adminServerStats:
		e.extractAdminStats(now, document, mb)
	}
}

func (e *extractor) extractDBStats(ts pdata.Timestamp, doc bson.M, mb *metadata.MetricsBuilder, dbName string) {
	collectionsPath := []string{"collections"}
	if value, err := digForIntValue(doc, collectionsPath); err == nil {
		mb.RecordMongodbCollectionCountDataPoint(ts, value, dbName)
	}

	dataSizePath := []string{"dataSize"}
	if value, err := digForIntValue(doc, dataSizePath); err == nil {
		mb.RecordMongodbDataSizeDataPoint(ts, value, dbName)
	}

	numExtentsPath := []string{"numExtents"}
	if value, err := digForIntValue(doc, numExtentsPath); err == nil {
		mb.RecordMongodbExtentCountDataPoint(ts, value, dbName)
	}

	indexSizePath := []string{"indexSize"}
	if value, err := digForIntValue(doc, indexSizePath); err == nil {
		mb.RecordMongodbIndexSizeDataPoint(ts, value, dbName)
	}

	indexCountPath := []string{"indexes"}
	if value, err := digForIntValue(doc, indexCountPath); err == nil {
		mb.RecordMongodbIndexCountDataPoint(ts, value, dbName)
	}

	objectsPath := []string{"objects"}
	if value, err := digForIntValue(doc, objectsPath); err == nil {
		mb.RecordMongodbObjectCountDataPoint(ts, value, dbName)
	}

	storageSizePath := []string{"storageSize"}
	if value, err := digForIntValue(doc, storageSizePath); err == nil {
		mb.RecordMongodbStorageSizeDataPoint(ts, value, dbName)
	}
}

func (e *extractor) extractNormalServerStats(ts pdata.Timestamp, doc bson.M, mb *metadata.MetricsBuilder, dbName string) {
	activeConnectionsPath := []string{"connections", "active"}
	if value, err := digForIntValue(doc, activeConnectionsPath); err == nil {
		mb.RecordMongodbConnectionCountDataPoint(ts, value, dbName, metadata.AttributeConnectionType.Active)
	}

	availableConnectionsPath := []string{"connections", "available"}
	if value, err := digForIntValue(doc, availableConnectionsPath); err == nil {
		mb.RecordMongodbConnectionCountDataPoint(ts, value, dbName, metadata.AttributeConnectionType.Available)
	}

	currentConnectionpath := []string{"connections", "current"}
	if value, err := digForIntValue(doc, currentConnectionpath); err == nil {
		mb.RecordMongodbConnectionCountDataPoint(ts, value, dbName, metadata.AttributeConnectionType.Current)
	}

	memResidentPath := []string{"mem", "resident"}
	if value, err := digForIntValue(doc, memResidentPath); err == nil {
		memoryValAsMb := value * int64(1048576)
		mb.RecordMongodbMemoryUsageDataPoint(ts, memoryValAsMb, dbName, metadata.AttributeMemoryType.Resident)
	}

	memVirtualPath := []string{"mem", "virtual"}
	if value, err := digForIntValue(doc, memVirtualPath); err == nil {
		memoryValAsMb := value * int64(1048576)
		mb.RecordMongodbMemoryUsageDataPoint(ts, memoryValAsMb, dbName, metadata.AttributeMemoryType.Virtual)
	}
}

func (e *extractor) extractAdminStats(ts pdata.Timestamp, document bson.M, mb *metadata.MetricsBuilder) {
	waitTime, err := e.extractGlobalLockWaitTime(document)
	if err == nil {
		mb.RecordMongodbGlobalLockTimeDataPoint(ts, waitTime)
	}

	// Collect Cache Hits & Misses
	canCalculateCacheHits := true

	cacheMisses, err := digForIntValue(document, []string{"wiredTiger", "cache", "pages read into cache"})
	if err != nil {
		e.logger.Error("failed to parse cache misses", zap.Error(err))
		canCalculateCacheHits = false
	} else {
		mb.RecordMongodbCacheOperationsDataPoint(ts, cacheMisses, metadata.AttributeType.Miss)
	}

	totalCacheRequests, err := digForIntValue(document, []string{"wiredTiger", "cache", "pages requested from the cache"})
	if err != nil {
		e.logger.Debug("failed to parse total cache requests unable to calculate cache hits", zap.Error(err))
		canCalculateCacheHits = false
	}

	if canCalculateCacheHits && totalCacheRequests > cacheMisses {
		cacheHits := totalCacheRequests - cacheMisses
		mb.RecordMongodbCacheOperationsDataPoint(ts, cacheHits, metadata.AttributeType.Hit)
	}

	// Collect Operations
	for _, operation := range []string{
		metadata.AttributeOperation.Insert,
		metadata.AttributeOperation.Query,
		metadata.AttributeOperation.Update,
		metadata.AttributeOperation.Delete,
		metadata.AttributeOperation.Getmore,
		metadata.AttributeOperation.Command,
	} {
		count, err := digForIntValue(document, []string{"opcounters", operation})
		if err != nil {
			e.logger.Error("failed to parse operation", zap.Error(err), zap.String("operation", operation))
			continue
		}
		mb.RecordMongodbOperationCountDataPoint(ts, count, operation)
	}
}

func (e *extractor) extractGlobalLockWaitTime(document bson.M) (int64, error) {
	var heldTimeUs int64

	// Mongo version greater than or equal to 4.0 have it in the serverStats at "globalLock", "totalTime"
	// reference: https://docs.mongodb.com/v4.0/reference/command/serverStatus/#server-status-global-lock
	if e.version.GreaterThanOrEqual(Mongo40) {
		heldTimeUs, _ = digForIntValue(document, []string{"globalLock", "totalTime"})
	} else {
		for _, lockType := range []string{"W", "R", "r", "w"} {
			waitTime, err := digForIntValue(document, []string{"locks", ".", "timeAcquiringMicros", lockType})
			if err == nil {
				heldTimeUs += waitTime
			}
		}
	}

	if heldTimeUs != 0 {
		htMilliseconds := heldTimeUs / 1000
		return htMilliseconds, nil
	}

	e.logger.Warn("unable to find global lock time")
	return 0, errors.New("was unable to calculate global lock time")
}

func digForIntValue(document bson.M, path []string) (int64, error) {
	curItem, remainingPath := path[0], path[1:]
	value := document[curItem]
	if value == nil {
		return 0, errors.New("nil found when digging for metric")
	}

	if len(remainingPath) == 0 {
		switch v := value.(type) {
		case int:
			return int64(v), nil
		case int32:
			return int64(v), nil
		case int64:
			return v, nil
		case float64:
			return int64(v), nil
		default:
			return 0, fmt.Errorf("unexpected type found when parsing int: %v", reflect.TypeOf(value))
		}
	}

	return digForIntValue(value.(bson.M), remainingPath)
}

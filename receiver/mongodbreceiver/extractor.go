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
	"strconv"
	"time"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

type numberType int

const (
	integer numberType = iota
	double
)

type mongoMetric struct {
	name             string
	path             []string
	staticAttributes map[string]string
	dataPointType    numberType
	minMongoVersion  *version.Version
	maxMongoVersion  *version.Version
	conversionFactor int
}

// convertToAppropriateValue allows for definitions of metrics to be scaled to appropriate units,
// example Megabytes to Bytes etc.
func (mm *mongoMetric) convertToAppropriateValue(value interface{}) interface{} {
	if mm.conversionFactor == 0 {
		return value
	}

	switch mm.dataPointType {
	case integer:
		v, _ := value.(int64)
		return v * int64(mm.conversionFactor)
	case double:
		v, _ := value.(float64)
		return v * float64(mm.conversionFactor)
	}

	return value
}

const (
	mebibytesToBytes = 1048576
)

type extractor struct {
	version *version.Version
	logger  *zap.Logger
}

const (
	collectionCount = "mongodb.collection.count"
	dataSize        = "mongodb.data.size"
	numExtents      = "mongodb.extent.count"
	indexSize       = "mongodb.index.size"
	indexes         = "mongodb.index.count"
	objects         = "mongodb.object.count"
	storageSize     = "mongodb.storage.size"
	connections     = "mongodb.connection.count"
	memUsage        = "mongodb.memory.usage"

	operationCount = "mongodb.operation.count"
)

var dbStatsMetrics = []mongoMetric{
	{
		name:          collectionCount,
		path:          []string{"collections"},
		dataPointType: integer,
	},
	{
		name:          dataSize,
		path:          []string{"dataSize"},
		dataPointType: integer,
	},
	{
		name:            numExtents,
		path:            []string{"numExtents"},
		dataPointType:   integer,
		maxMongoVersion: Mongo44,
	},
	{
		name:          indexSize,
		path:          []string{"indexSize"},
		dataPointType: integer,
	},
	{
		name:          indexes,
		path:          []string{"indexes"},
		dataPointType: integer,
	},
	{
		name:          objects,
		path:          []string{"objects"},
		dataPointType: integer,
	},
	{
		name:          storageSize,
		path:          []string{"storageSize"},
		dataPointType: integer,
	},
}

var serverStatusMetrics = []mongoMetric{
	{
		name:             connections,
		path:             []string{"connections", "active"},
		staticAttributes: map[string]string{metadata.A.ConnectionType: metadata.AttributeConnectionType.Active},
		dataPointType:    integer,
	},
	{
		name:             connections,
		path:             []string{"connections", "available"},
		staticAttributes: map[string]string{metadata.A.ConnectionType: metadata.AttributeConnectionType.Available},
		dataPointType:    integer,
	},
	{
		name:             connections,
		path:             []string{"connections", "current"},
		staticAttributes: map[string]string{metadata.A.ConnectionType: metadata.AttributeConnectionType.Current},
		dataPointType:    integer,
	},
	{
		name:             memUsage,
		path:             []string{"mem", "resident"},
		staticAttributes: map[string]string{metadata.A.MemoryType: metadata.AttributeMemoryType.Resident},
		dataPointType:    integer,
		// reported value is Mebibytes, want to convert to bytes
		// https://docs.mongodb.com/manual/reference/command/serverStatus/#mongodb-serverstatus-serverstatus.mem.resident
		conversionFactor: mebibytesToBytes,
	},
	{
		name:             memUsage,
		path:             []string{"mem", "virtual"},
		staticAttributes: map[string]string{metadata.A.MemoryType: metadata.AttributeMemoryType.Virtual},
		dataPointType:    integer,
		// https://docs.mongodb.com/manual/reference/command/serverStatus/#mongodb-serverstatus-serverstatus.mem.virtual
		conversionFactor: mebibytesToBytes,
	},
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

func (e *extractor) Extract(document bson.M, mm *metadata.MetricsBuilder, dbName string, et extractType) {
	now := pdata.NewTimestampFromTime(time.Now())
	switch et {
	case normalDBStats:
		e.extractStats(now, document, mm, dbName, dbStatsMetrics)
	case normalServerStats:
		e.extractStats(now, document, mm, dbName, serverStatusMetrics)
	case adminServerStats:
		e.extractAdminStats(document, mm)
	}
}

func (e *extractor) extractStats(ts pdata.Timestamp, document bson.M, mb *metadata.MetricsBuilder, dbName string, metrics []mongoMetric) {
	for _, metric := range metrics {
		if !e.shouldDig(metric) {
			continue
		}
		e.addMetric(ts, mb, document, metric, dbName)
	}
}

func (e *extractor) extractAdminStats(document bson.M, mb *metadata.MetricsBuilder) {
	now := pdata.NewTimestampFromTime(time.Now())
	waitTime, err := e.extractGlobalLockWaitTime(document)
	if err == nil {
		mb.RecordMongodbGlobalLockTimeDataPoint(now, waitTime)
	}

	// Collect Cache Hits & Misses
	canCalculateCacheHits := true

	cacheMisses, err := digForIntValue(document, []string{"wiredTiger", "cache", "pages read into cache"})
	if err != nil {
		e.logger.Error("Failed to Parse", zap.Error(err), zap.String("metric", operationCount))
		canCalculateCacheHits = false
	} else {
		mb.RecordMongodbCacheOperationsDataPoint(now, cacheMisses, metadata.AttributeType.Miss)
	}

	totalCacheRequests, err := digForIntValue(document, []string{"wiredTiger", "cache", "pages requested from the cache"})
	if err != nil {
		e.logger.Error("Failed to Parse", zap.Error(err), zap.String("metric", operationCount))
		canCalculateCacheHits = false
	}

	if canCalculateCacheHits && totalCacheRequests > cacheMisses {
		cacheHits := totalCacheRequests - cacheMisses
		mb.RecordMongodbCacheOperationsDataPoint(now, cacheHits, metadata.AttributeType.Hit)
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
			e.logger.Error("Failed to Parse", zap.Error(err), zap.String("metric", operationCount))
			continue
		}
		mb.RecordMongodbOperationCountDataPoint(now, count, operation)
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

func (e *extractor) addMetric(ts pdata.Timestamp, mb *metadata.MetricsBuilder, document bson.M, m mongoMetric, dbName string) {
	switch m.dataPointType {
	case integer:
		e.addIntMetric(ts, mb, document, m, dbName)
	case double:
		e.addDoubleMetric(ts, mb, document, m, dbName)
	default:
		e.logger.Debug("unknown data point type", zap.String("name", m.name))
	}
}

func (e *extractor) addIntMetric(ts pdata.Timestamp, mb *metadata.MetricsBuilder, document bson.M, metric mongoMetric, dbName string) {
	value, err := digForIntValue(document, metric.path)
	if err != nil {
		e.logger.Debug("unable to find metric", zap.String("name", metric.name))
		return
	}

	if metric.conversionFactor != 0 {
		value = metric.convertToAppropriateValue(value).(int64)
	}

	switch metric.name {
	// mongodb.collection.count
	case collectionCount:
		mb.RecordMongodbCollectionCountDataPoint(ts, value, dbName)
	// mongodb.connection.count
	case connections:
		mb.RecordMongodbConnectionCountDataPoint(ts, value, dbName, metric.staticAttributes[metadata.A.ConnectionType])
	// mongodb.data.size
	case dataSize:
		mb.RecordMongodbDataSizeDataPoint(ts, value, dbName)
	// mongodb.index.count
	case indexes:
		mb.RecordMongodbIndexCountDataPoint(ts, value, dbName)
	// mongodb.index.size
	case indexSize:
		mb.RecordMongodbIndexSizeDataPoint(ts, value, dbName)
	// mongodb.memory.usage
	case memUsage:
		mb.RecordMongodbMemoryUsageDataPoint(ts, value, dbName, metric.staticAttributes[metadata.A.MemoryType])
	// mongodb.extent.count
	case numExtents:
		mb.RecordMongodbExtentCountDataPoint(ts, value, dbName)
	// mongodb.object.count
	case objects:
		mb.RecordMongodbObjectCountDataPoint(ts, value, dbName)
	// mongodb.storage.size
	case storageSize:
		mb.RecordMongodbStorageSizeDataPoint(ts, value, dbName)
	default:
		e.logger.Warn("attempted to add an unknown metric", zap.String("metric", metric.name))
	}
}

func (e *extractor) addDoubleMetric(_ pdata.Timestamp, _ *metadata.MetricsBuilder, document bson.M, metric mongoMetric, _ string) {
	_, err := digForDoubleValue(document, metric.path)
	if err != nil {
		e.logger.Debug("unable to find metric", zap.String("name", metric.name))
		return
	}

	switch metric.name {
	default:
		e.logger.Warn("attempted to add metric datapoint was collected.", zap.String("name", metric.name))
	}
}

func (e *extractor) shouldDig(metric mongoMetric) bool {
	if metric.maxMongoVersion == nil && metric.minMongoVersion == nil {
		return true
	}

	satisfyLowerBound, satisfyUpperBound := true, true
	if metric.minMongoVersion != nil {
		satisfyLowerBound = e.version.GreaterThanOrEqual(metric.minMongoVersion)
	}

	if metric.maxMongoVersion != nil {
		satisfyUpperBound = e.version.LessThanOrEqual(metric.maxMongoVersion)
	}

	return satisfyLowerBound && satisfyUpperBound
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
		case float32:
			return int64(v), nil
		case string:
			return strconv.ParseInt(v, 10, 64)
		default:
			return 0, fmt.Errorf("unexpected type found when parsing int: %v", reflect.TypeOf(value))
		}
	}

	return digForIntValue(value.(bson.M), remainingPath)
}

func digForDoubleValue(document bson.M, path []string) (float64, error) {
	curItem, remainingPath := path[0], path[1:]
	value := document[curItem]
	if value == nil {
		return 0, errors.New("nil found when digging for metric")
	}

	if len(remainingPath) == 0 {
		switch v := value.(type) {
		case int:
			return float64(v), nil
		case float32:
			return float64(v), nil
		case float64:
			return v, nil
		case string:
			return strconv.ParseFloat(v, 64)
		default:
			return 0, fmt.Errorf("unexpected type found when parsing double: %v", reflect.TypeOf(value))
		}
	}

	return digForDoubleValue(value.(bson.M), remainingPath)
}

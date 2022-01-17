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
	metricDef        metadata.MetricIntf
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
	mebibytesToBytes = 1.049e+6
)

type extractor struct {
	version *version.Version
	logger  *zap.Logger
}

var dbStatsMetrics = []mongoMetric{
	{
		metricDef:     metadata.M.MongodbCollectionCount,
		path:          []string{"collections"},
		dataPointType: integer,
	},
	{
		metricDef:     metadata.M.MongodbDataSize,
		path:          []string{"dataSize"},
		dataPointType: double,
	},
	{
		metricDef:       metadata.M.MongodbExtentCount,
		path:            []string{"numExtents"},
		dataPointType:   integer,
		maxMongoVersion: Mongo44,
	},
	{
		metricDef:     metadata.M.MongodbIndexSize,
		path:          []string{"indexSize"},
		dataPointType: double,
	},
	{
		metricDef:     metadata.M.MongodbIndexCount,
		path:          []string{"indexes"},
		dataPointType: integer,
	},
	{
		metricDef:     metadata.M.MongodbObjectCount,
		path:          []string{"objects"},
		dataPointType: integer,
	},
	{
		metricDef:     metadata.M.MongodbStorageSize,
		path:          []string{"storageSize"},
		dataPointType: double,
	},
}

var serverStatusMetrics = []mongoMetric{
	{
		metricDef:        metadata.M.MongodbConnectionCount,
		path:             []string{"connections", "active"},
		staticAttributes: map[string]string{metadata.A.ConnectionType: metadata.AttributeConnectionType.Active},
		dataPointType:    integer,
	},
	{
		metricDef:        metadata.M.MongodbConnectionCount,
		path:             []string{"connections", "available"},
		staticAttributes: map[string]string{metadata.A.ConnectionType: metadata.AttributeConnectionType.Available},
		dataPointType:    integer,
	},
	{
		metricDef:        metadata.M.MongodbConnectionCount,
		path:             []string{"connections", "current"},
		staticAttributes: map[string]string{metadata.A.ConnectionType: metadata.AttributeConnectionType.Current},
		dataPointType:    integer,
	},
	{
		metricDef:        metadata.M.MongodbMemoryUsage,
		path:             []string{"mem", "resident"},
		staticAttributes: map[string]string{metadata.A.MemoryType: metadata.AttributeMemoryType.Resident},
		dataPointType:    integer,
		// reported value is Mebibytes, want to convert to bytes
		// https://docs.mongodb.com/manual/reference/command/serverStatus/#mongodb-serverstatus-serverstatus.mem.resident
		conversionFactor: mebibytesToBytes,
	},
	{
		metricDef:        metadata.M.MongodbMemoryUsage,
		path:             []string{"mem", "virtual"},
		staticAttributes: map[string]string{metadata.A.MemoryType: metadata.AttributeMemoryType.Virtual},
		dataPointType:    integer,
		// https://docs.mongodb.com/manual/reference/command/serverStatus/#mongodb-serverstatus-serverstatus.mem.virtual
		conversionFactor: mebibytesToBytes,
	},
}

type extractType = int

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

func (e *extractor) Extract(document bson.M, mm *metricManager, dbName string, et extractType) {
	switch et {
	case normalDBStats:
		e.extractStats(document, mm, dbName, dbStatsMetrics)
	case normalServerStats:
		e.extractStats(document, mm, dbName, serverStatusMetrics)
	case adminServerStats:
		e.extractAdminStats(document, mm)
	}
}

func (e *extractor) extractStats(document bson.M, mm *metricManager, dbName string, metrics []mongoMetric) {
	for _, metric := range metrics {
		if e.shouldDig(metric) {
			attributes := pdata.NewAttributeMap()
			attributes.Insert(metadata.A.Database, pdata.NewAttributeValueString(dbName))
			for k, v := range metric.staticAttributes {
				attributes.Insert(k, pdata.NewAttributeValueString(v))
			}
			value, err := e.extractMetric(document, metric)
			if err != nil {
				e.logger.Warn("Failed to extract metric",
					zap.String("database", dbName),
					zap.String("mongo-version", e.version.String()),
					zap.String("metric", metric.metricDef.Name()),
					zap.Strings("path", metric.path),
					zap.Error(err))
				continue
			}
			value = metric.convertToAppropriateValue(value)

			mm.addDataPoint(metric.metricDef, value, attributes)
		}
	}
}

func (e *extractor) extractAdminStats(document bson.M, mm *metricManager) {
	waitTime, err := e.extractGlobalLockWaitTime(document)
	if err == nil {
		mm.addDataPoint(metadata.M.MongodbGlobalLockTime, waitTime, pdata.NewAttributeMap())
	}

	// Collect Cache Hits & Misses
	canCalculateCacheHits := true

	cacheMisses, err := digForIntValue(document, []string{"wiredTiger", "cache", "pages read into cache"})
	if err != nil {
		e.logger.Error("Failed to Parse", zap.Error(err), zap.String("metric", metadata.M.MongodbCacheOperations.Name()))
		canCalculateCacheHits = false
	} else {
		attributes := pdata.NewAttributeMap()
		attributes.Insert(metadata.A.Type, pdata.NewAttributeValueString("misses"))
		mm.addDataPoint(metadata.M.MongodbCacheOperations, cacheMisses, attributes)
	}

	totalCacheRequests, err := digForIntValue(document, []string{"wiredTiger", "cache", "pages requested from the cache"})
	if err != nil {
		e.logger.Error("Failed to Parse", zap.Error(err), zap.String("metric", metadata.M.MongodbCacheOperations.Name()))
		canCalculateCacheHits = false
	}

	if canCalculateCacheHits && totalCacheRequests > cacheMisses {
		cacheHits := totalCacheRequests - cacheMisses
		attributes := pdata.NewAttributeMap()
		attributes.Insert(metadata.A.Type, pdata.NewAttributeValueString("hits"))
		mm.addDataPoint(metadata.M.MongodbCacheOperations, cacheHits, attributes)
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
			e.logger.Error("Failed to Parse", zap.Error(err), zap.String("metric", metadata.M.MongodbOperationCount.Name()))
		} else {
			attributes := pdata.NewAttributeMap()
			attributes.Insert(metadata.A.Operation, pdata.NewAttributeValueString(operation))
			mm.addDataPoint(metadata.M.MongodbOperationCount, count, attributes)
		}
	}
}

func (e *extractor) extractGlobalLockWaitTime(document bson.M) (int64, error) {
	var lockKey string
	if e.version.GreaterThanOrEqual(Mongo30) {
		lockKey = "Global"
	} else {
		lockKey = "."
	}
	totalWaitTime := int64(0)
	foundLocks := false
	for _, lockType := range []string{"W", "R", "r", "w"} {
		waitTimeMicroSeconds, err := digForIntValue(
			document,
			[]string{"locks", lockKey, "timeAcquiringMicros", lockType},
		)
		if err == nil {
			// reported value is in microseconds, prefer to report in milliseconds
			totalWaitTime += waitTimeMicroSeconds
			foundLocks = true
		}
	}
	if foundLocks {
		waitTimeMilliseconds := totalWaitTime / 1000
		return waitTimeMilliseconds, nil
	}

	e.logger.Warn("unable to calculate global lock wait time")
	return 0, errors.New("was unable to calculate global lock time")
}

func (e *extractor) extractMetric(document bson.M, m mongoMetric) (interface{}, error) {
	switch m.dataPointType {
	case integer:
		return digForIntValue(document, m.path)
	case double:
		return digForDoubleValue(document, m.path)
	default:
		return nil, fmt.Errorf("unsupported datapoint type: %#v", m.dataPointType)
	}
}

func (e *extractor) shouldDig(metric mongoMetric) bool {
	if metric.maxMongoVersion == nil && metric.minMongoVersion == nil {
		return true
	}
	return (metric.minMongoVersion != nil && e.version.GreaterThanOrEqual(metric.minMongoVersion)) || (metric.maxMongoVersion != nil && e.version.LessThanOrEqual(metric.maxMongoVersion))
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

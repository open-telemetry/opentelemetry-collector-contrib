package mongodbreceiver

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

type extractor struct {
	version *version.Version
	logger  *zap.Logger
}

var dbStatsMetrics = []mongoMetric{
	{
		metricDef:     metadata.M.MongodbCollections,
		path:          []string{"collections"},
		dataPointType: integer,
	},
	{
		metricDef:     metadata.M.MongodbDataSize,
		path:          []string{"dataSize"},
		dataPointType: double,
	},
	{
		metricDef:       metadata.M.MongodbExtents,
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
		metricDef:     metadata.M.MongodbObjects,
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
		metricDef:        metadata.M.MongodbConnections,
		path:             []string{"connections", "active"},
		staticAttributes: map[string]string{metadata.A.ConnectionType: metadata.AttributeConnectionType.Active},
		dataPointType:    integer,
	},
	{
		metricDef:        metadata.M.MongodbConnections,
		path:             []string{"connections", "available"},
		staticAttributes: map[string]string{metadata.A.ConnectionType: metadata.AttributeConnectionType.Available},
		dataPointType:    integer,
	},
	{
		metricDef:        metadata.M.MongodbConnections,
		path:             []string{"connections", "current"},
		staticAttributes: map[string]string{metadata.A.ConnectionType: metadata.AttributeConnectionType.Current},
		dataPointType:    integer,
	},
	{
		metricDef:        metadata.M.MongodbMemoryUsage,
		path:             []string{"mem", "resident"},
		staticAttributes: map[string]string{metadata.A.MemoryType: metadata.AttributeMemoryType.Resident},
		dataPointType:    integer,
	},
	{
		metricDef:        metadata.M.MongodbMemoryUsage,
		path:             []string{"mem", "virtual"},
		staticAttributes: map[string]string{metadata.A.MemoryType: metadata.AttributeMemoryType.Virtual},
		dataPointType:    integer,
	},
	{
		metricDef:        metadata.M.MongodbMemoryUsage,
		path:             []string{"mem", "mapped"},
		staticAttributes: map[string]string{metadata.A.MemoryType: metadata.AttributeMemoryType.Mapped},
		dataPointType:    integer,
		// removed in 4.4 https://docs.mongodb.com/v4.4/reference/command/serverStatus/
		maxMongoVersion: Mongo42,
	},
	{
		metricDef:        metadata.M.MongodbMemoryUsage,
		path:             []string{"mem", "mappedWithJournal"},
		staticAttributes: map[string]string{metadata.A.MemoryType: metadata.AttributeMemoryType.MappedWithJournal},
		dataPointType:    integer,
		// removed in 4.4 https://docs.mongodb.com/v4.4/reference/command/serverStatus/
		maxMongoVersion: Mongo42,
	},
}

type extractType = int

const (
	AdminServerStats extractType = iota
	NormalDBStats
	NormalServerStats
)

// Mongo30 is a version representation of mongo 2.6
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
		return nil, fmt.Errorf("unable to create extractor due to invalid semver %s: %w", mongoVersion, err)
	}
	return &extractor{
		version: v,
		logger:  logger,
	}, nil
}

func (e *extractor) Extract(document bson.M, mm *metricManager, dbName string, et extractType) {
	switch et {
	case NormalDBStats:
		e.extractStats(document, mm, dbName, dbStatsMetrics)
	case NormalServerStats:
		e.extractStats(document, mm, dbName, serverStatusMetrics)
	case AdminServerStats:
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
			mm.addDataPoint(metric.metricDef, value, attributes)
		}
	}
}

func (e *extractor) extractAdminStats(document bson.M, mm *metricManager) {
	waitTime, err := e.extractGlobalLockWaitTime(document)
	if err == nil {
		mm.addDataPoint(metadata.M.MongodbGlobalLockHold, waitTime, pdata.NewAttributeMap())
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
			e.logger.Error("Failed to Parse", zap.Error(err), zap.String("metric", metadata.M.MongodbOperations.Name()))
		} else {
			attributes := pdata.NewAttributeMap()
			attributes.Insert(metadata.A.Operation, pdata.NewAttributeValueString(operation))
			mm.addDataPoint(metadata.M.MongodbOperations, count, attributes)
		}
	}
}

func (e *extractor) extractGlobalLockWaitTime(document bson.M) (int64, error) {
	switch {
	// Mongo version greater than or equal to 4.0 have it in the serverStats at "globalLock", "totalTime"
	// reference: https://docs.mongodb.com/v4.0/reference/command/serverStatus/#server-status-global-lock
	case e.version.GreaterThanOrEqual(Mongo40):
		return digForIntValue(document, []string{"globalLock", "totalTime"})
	default:
		totalWaitTime := int64(0)
		foundLocks := false
		for _, lockType := range []string{"W", "R", "r", "w"} {
			waitTime, err := digForIntValue(document, []string{"locks", "Global", "timeAcquiringMicros", lockType})
			if err == nil {
				totalWaitTime += waitTime / 1000
				foundLocks = true
			}
		}
		if foundLocks {
			return totalWaitTime, nil
		}
		e.logger.Warn("unable to find global lock time")
		return 0, errors.New("was unable to calculate global lock time")
	}
}

func (e *extractor) extractMetric(document bson.M, m mongoMetric) (interface{}, error) {
	switch m.dataPointType {
	case integer:
		return digForIntValue(document, m.path)
	case double:
		return digForDoubleValue(document, m.path)
	default:
		return nil, fmt.Errorf("unsupported dataPointType: %#v", m.dataPointType)
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
	} else if len(remainingPath) == 0 {
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
	} else {
		return digForIntValue(value.(bson.M), remainingPath)
	}
}

func digForDoubleValue(document bson.M, path []string) (float64, error) {
	curItem, remainingPath := path[0], path[1:]
	value := document[curItem]
	if value == nil {
		return 0, errors.New("nil found when digging for metric")
	} else if len(remainingPath) == 0 {
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
	} else {
		return digForDoubleValue(value.(bson.M), remainingPath)
	}
}

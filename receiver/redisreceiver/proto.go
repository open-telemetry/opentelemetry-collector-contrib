package redisreceiver

import (
	"fmt"
	"strconv"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
)

// Builds proto metrics from the combination of a metrics map and a redisMetricCollection list. These are the
// fixed, non keyspace metrics.
func buildFixedProtoMetrics(redisInfo map[string]string, metrics []*redisMetric, t time.Time) ([]*metricspb.Metric, []error) {
	var warnings []error
	var protoMetrics []*metricspb.Metric
	for _, redisMetric := range metrics {
		strVal, ok := redisInfo[redisMetric.key]
		if !ok {
			warnings = append(warnings, fmt.Errorf("info key not found: %v", redisMetric.key))
			continue
		}
		if len(strVal) == 0 {
			continue
		}
		protoMetric, parsingError := buildSingleProtoMetric(redisMetric, strVal, t)
		if parsingError != nil {
			warnings = append(warnings, parsingError)
			continue
		}
		protoMetrics = append(protoMetrics, protoMetric)
	}
	return protoMetrics, warnings
}

// Builds proto metrics from any 'keyspace' metrics in Redis INFO:
// e.g. "db0:keys=1,expires=2,avg_ttl=3"
func buildKeyspaceProtoMetrics(redisInfo map[string]string, t time.Time) ([]*metricspb.Metric, []error) {
	var warnings []error
	var out []*metricspb.Metric
	const RedisMaxDbs = 16
	for i := 0; i < RedisMaxDbs; i++ {
		key := "db" + strconv.Itoa(i)
		str, ok := redisInfo[key]
		if !ok {
			break
		}
		keyspace, parsingError := parseKeyspaceString(i, str)
		if parsingError != nil {
			warnings = append(warnings, parsingError)
			continue
		}
		keyspaceMetrics := buildKeyspaceTriplet(keyspace, t)
		out = append(out, keyspaceMetrics...)
	}
	return out, warnings
}

func newMetricsData(protoMetrics []*metricspb.Metric) *consumerdata.MetricsData {
	return &consumerdata.MetricsData{
		Resource: &resourcepb.Resource{
			Type:   typeStr,
			Labels: map[string]string{"type": typeStr},
		},
		Metrics: protoMetrics,
	}
}

func buildKeyspaceTriplet(k *keyspace, t time.Time) []*metricspb.Metric {
	return []*metricspb.Metric{
		buildKeyspaceKeysMetric(k, t),
		buildKeyspaceExpiresMetric(k, t),
		buildKeyspaceTtlMetric(k, t),
	}
}

func buildKeyspaceKeysMetric(k *keyspace, t time.Time) *metricspb.Metric {
	ttlRedisMetric := &redisMetric{
		name:   "redis/db/keys",
		labels: map[string]string{"db": k.db},
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
	ttlPt := &metricspb.Point{Value: &metricspb.Point_Int64Value{Int64Value: int64(k.keys)}}
	return newProtoMetric(ttlRedisMetric, ttlPt, t)
}

func buildKeyspaceExpiresMetric(k *keyspace, t time.Time) *metricspb.Metric {
	ttlRedisMetric := &redisMetric{
		name:   "redis/db/expires",
		labels: map[string]string{"db": k.db},
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
	ttlPt := &metricspb.Point{Value: &metricspb.Point_Int64Value{Int64Value: int64(k.expires)}}
	return newProtoMetric(ttlRedisMetric, ttlPt, t)
}

func buildKeyspaceTtlMetric(k *keyspace, t time.Time) *metricspb.Metric {
	ttlRedisMetric := &redisMetric{
		name:   "redis/db/avg_ttl",
		units:  "ms",
		labels: map[string]string{"db": k.db},
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
	ttlPt := &metricspb.Point{Value: &metricspb.Point_Int64Value{Int64Value: int64(k.avgTtl)}}
	return newProtoMetric(ttlRedisMetric, ttlPt, t)
}

// Create new protobuf Metric.
// Arguments:
//   * redisMetric -- the fixed metadata to build the pb metric
//   * pt -- the Point value: e.g. a Point_Int64Value
//   * currTime -- the timestamp to be put on the Point (not on the timeseries)
func newProtoMetric(redisMetric *redisMetric, pt *metricspb.Point, currTime time.Time) *metricspb.Metric {
	// for cumulative types, set the start time to a non-nil value
	var startTime *timestamp.Timestamp = nil
	switch redisMetric.mdType {
	case metricspb.MetricDescriptor_CUMULATIVE_INT64, metricspb.MetricDescriptor_CUMULATIVE_DOUBLE:
		startTime = &timestamp.Timestamp{} // todo: not sure about this
	}

	pt.Timestamp = timeToTimestamp(&currTime)
	labelKeys, labelVals := buildLabels(redisMetric.labels, redisMetric.labelDescriptions)
	pbMetric := &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:        redisMetric.name,
			Description: redisMetric.desc,
			Unit:        redisMetric.units,
			Type:        redisMetric.mdType,
			LabelKeys:   labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{{
			LabelValues:    labelVals,
			Points:         []*metricspb.Point{pt},
			StartTimestamp: startTime,
		}},
	}
	return pbMetric
}

// TODO: Maybe this should be moved to a general purpose utility if we don't have one already
func timeToTimestamp(t *time.Time) *timestamp.Timestamp {
	if t == nil {
		return nil
	}
	return &timestamp.Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

func buildLabels(
	labels map[string]string,
	descriptions map[string]string,
) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	var keys []*metricspb.LabelKey
	var values []*metricspb.LabelValue
	for key, val := range labels {
		labelKey := &metricspb.LabelKey{Key: key}
		desc, hasDesc := descriptions[key]
		if hasDesc {
			labelKey.Description = desc
		}
		keys = append(keys, labelKey)
		values = append(values, &metricspb.LabelValue{Value: val})
	}
	return keys, values
}

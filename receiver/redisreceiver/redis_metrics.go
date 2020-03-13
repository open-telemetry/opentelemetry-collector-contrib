package redisreceiver

import (
	"errors"

	metricsProto "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

type redisMetric struct {
	key               string
	units             string
	desc              string
	labels            map[string]string
	labelDescriptions map[string]string
	metricType        metricType
}

func getDefaultRedisMetrics() []*redisMetric {
	return []*redisMetric{
		uptimeInSeconds(),
		uptimeInDays(),

		usedCpuSys(),
		usedCpuSysChildren(),
		usedCpuSysUserChildren(),

		lruClock(),

		connectedClients(),

		clientRecentMaxInputBuffer(),
		clientRecentMaxOutputBuffer(),

		blockedClients(),

		expiredKeys(),
		evictedKeys(),

		rejectedConnections(),

		usedMemory(),
		usedMemoryRss(),
		usedMemoryPeak(),
		usedMemoryLua(),

		memFragmentationRatio(),

		changesSinceLastSave(),

		instantaneousOpsPerSec(),

		rdbBgsaveInProgress(),

		totalConnectionsReceived(),
		totalCommandsProcessed(),

		totalNetInputBytes(),
		totalNetOutputBytes(),

		keyspaceHits(),
		keyspaceMisses(),

		latestForkUsec(),

		connectedSlaves(),

		replBacklogFirstByteOffset(),

		masterReplOffset(),
	}
}

type metricType int

const (
	unspecified = iota
	gaugeInt
	gaugeDouble
	gaugeDistribution
	cumulativeInt
	cumulativeDouble
)

func buildProtoMetrics(
	redisInfo map[string]string,
	redisMetrics []*redisMetric,
) ([]*metricsProto.Metric, []error) {
	var parsingErrors []error
	var protoMetrics []*metricsProto.Metric
	for _, redisMetric := range redisMetrics {
		strVal := redisInfo[redisMetric.key]
		if len(strVal) == 0 {
			continue
		}
		protoMetric, parsingError := buildSingleProtoMetric(strVal, redisMetric)
		if parsingError != nil {
			parsingErrors = append(parsingErrors, parsingError)
			continue
		}
		protoMetrics = append(protoMetrics, protoMetric)
	}
	return protoMetrics, parsingErrors
}

func buildSingleProtoMetric(strVal string, redisMetric *redisMetric) (*metricsProto.Metric, error) {
	f := getPointConverter(redisMetric)
	if f == nil {
		return nil, errors.New("oops") // todo constant?
	}
	pt, err := f(strVal)
	if err != nil {
		return nil, err
	}
	labelKeys, labelVals := convertLabels(redisMetric.labels, redisMetric.labelDescriptions)
	metric := &metricsProto.Metric{
		MetricDescriptor: &metricsProto.MetricDescriptor{
			Name:        redisMetric.key,
			Description: redisMetric.desc,
			Unit:        redisMetric.units,
			Type:        metricsProto.MetricDescriptor_Type(redisMetric.metricType),
			LabelKeys:   labelKeys,
		},
		Timeseries: []*metricsProto.TimeSeries{{
			LabelValues: labelVals,
			Points:      []*metricsProto.Point{pt},
		}},
	}
	return metric, nil
}

func getPointConverter(metric *redisMetric) pointConverter {
	switch metric.metricType {
	case cumulativeInt, gaugeInt:
		return strToInt64Point
	case cumulativeDouble, gaugeDouble:
		return strToDoublePoint
	}
	return nil
}

func convertLabels(
	labels map[string]string,
	descriptions map[string]string,
) ([]*metricsProto.LabelKey, []*metricsProto.LabelValue) {
	var keys []*metricsProto.LabelKey
	var values []*metricsProto.LabelValue
	for key, val := range labels {
		labelKey := &metricsProto.LabelKey{Key: key}
		desc, hasDesc := descriptions[key]
		if hasDesc {
			labelKey.Description = desc
		}
		keys = append(keys, labelKey)
		values = append(values, &metricsProto.LabelValue{Value: val})
	}
	return keys, values
}

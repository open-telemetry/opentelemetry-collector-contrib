// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"

import (
	"bufio"
	"bytes"
	"errors"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
)

const (
	TypeStr = "cwmetrics"

	attributeAWSCloudWatchMetricStreamName = "aws.cloudwatch.metric_stream_name"
	dimensionInstanceID                    = "InstanceId"
	namespaceDelimiter                     = "/"
)

var errInvalidRecords = errors.New("record format invalid")

// Unmarshaler for the CloudWatch Metric Stream JSON record format.
//
// More details can be found at:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-json.html
type Unmarshaler struct {
	logger *zap.Logger

	buildInfo component.BuildInfo
}

var _ pmetric.Unmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger, buildInfo component.BuildInfo) *Unmarshaler {
	return &Unmarshaler{logger, buildInfo}
}

// UnmarshalMetrics deserializes the record in CloudWatch Metric Stream JSON
// format into a pmetric.Metrics, grouping metrics by resource and metric
// name and unit.
func (u Unmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	type metricKey struct {
		name string
		unit string
	}
	byResource := make(map[resourceKey]map[metricKey]pmetric.Metric)

	// Multiple metrics in each record separated by newline character
	scanner := bufio.NewScanner(bytes.NewReader(record))
	for datumIndex := 0; scanner.Scan(); datumIndex++ {
		var cwMetric cWMetric
		if err := jsoniter.ConfigFastest.Unmarshal(scanner.Bytes(), &cwMetric); err != nil {
			u.logger.Error(
				"Unable to unmarshal input",
				zap.Error(err),
				zap.Int("datum_index", datumIndex),
			)
			continue
		}
		if !u.isValid(cwMetric) {
			u.logger.Error(
				"Invalid metric",
				zap.Int("datum_index", datumIndex),
			)
			continue
		}

		rkey := resourceKey{
			metricStreamName: cwMetric.MetricStreamName,
			namespace:        cwMetric.Namespace,
			accountID:        cwMetric.AccountID,
			region:           cwMetric.Region,
		}
		metrics, ok := byResource[rkey]
		if !ok {
			metrics = make(map[metricKey]pmetric.Metric)
			byResource[rkey] = metrics
		}

		mkey := metricKey{
			name: cwMetric.MetricName,
			unit: cwMetric.Unit,
		}
		metric, ok := metrics[mkey]
		if !ok {
			metric = pmetric.NewMetric()
			metric.SetName(mkey.name)
			metric.SetUnit(mkey.unit)
			metric.SetEmptySummary()
			metrics[mkey] = metric
		}

		dp := metric.Summary().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(cwMetric.Timestamp)))
		setDataPointAttributes(cwMetric, dp)
		dp.SetCount(uint64(cwMetric.Value.Count))
		dp.SetSum(cwMetric.Value.Sum)
		minQ := dp.QuantileValues().AppendEmpty()
		minQ.SetQuantile(0)
		minQ.SetValue(cwMetric.Value.Min)
		maxQ := dp.QuantileValues().AppendEmpty()
		maxQ.SetQuantile(1)
		maxQ.SetValue(cwMetric.Value.Max)

		for key, value := range cwMetric.Value.Percentiles {
			// Only process percentile fields (those starting with 'p')
			if len(key) < 2 || key[0] != 'p' {
				continue
			}

			// Extract the percentile value from the field name (e.g., "p95" -> 0.95)
			percentileStr := key[1:]
			percentileInt, err := strconv.ParseFloat(percentileStr, 64)
			if err != nil {
				// Skip if we can't parse the percentile value
				u.logger.Debug(
					"Unable to parse percentile",
					zap.String("percentile", percentileStr),
					zap.Error(err),
				)
				continue
			}

			// Calculate the quantile value (divide by 100 to get a value between 0 and 1)
			quantile := percentileInt / 100.0

			q := dp.QuantileValues().AppendEmpty()
			q.SetQuantile(quantile)
			q.SetValue(value)
		}
	}
	if err := scanner.Err(); err != nil {
		// Treat this as a non-fatal error, and handle the data below.
		u.logger.Error("Error scanning for newline-delimited JSON", zap.Error(err))
	}
	if len(byResource) == 0 {
		return pmetric.Metrics{}, errInvalidRecords
	}

	metrics := pmetric.NewMetrics()
	for resourceKey, metricsMap := range byResource {
		rm := metrics.ResourceMetrics().AppendEmpty()
		setResourceAttributes(resourceKey, rm.Resource())
		scopeMetrics := rm.ScopeMetrics().AppendEmpty()
		scopeMetrics.Scope().SetName(metadata.ScopeName)
		scopeMetrics.Scope().SetVersion(u.buildInfo.Version)
		for _, metric := range metricsMap {
			metric.MoveTo(scopeMetrics.Metrics().AppendEmpty())
		}
	}
	return metrics, nil
}

// isValid validates that the cWMetric has been unmarshalled correctly.
func (Unmarshaler) isValid(metric cWMetric) bool {
	return metric.MetricName != "" && metric.Namespace != "" && metric.Unit != "" && metric.Value.isSet
}

// Type of the serialized messages.
func (Unmarshaler) Type() string {
	return TypeStr
}

type resourceKey struct {
	metricStreamName string
	namespace        string
	accountID        string
	region           string
}

// setResourceAttributes sets attributes on a pcommon.Resource from a cwMetric.
func setResourceAttributes(key resourceKey, resource pcommon.Resource) {
	attributes := resource.Attributes()
	attributes.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attributes.PutStr(string(conventions.CloudAccountIDKey), key.accountID)
	attributes.PutStr(string(conventions.CloudRegionKey), key.region)
	serviceNamespace, serviceName := toServiceAttributes(key.namespace)
	if serviceNamespace != "" {
		attributes.PutStr(string(conventions.ServiceNamespaceKey), serviceNamespace)
	}
	attributes.PutStr(string(conventions.ServiceNameKey), serviceName)
	attributes.PutStr(attributeAWSCloudWatchMetricStreamName, key.metricStreamName)
}

// toServiceAttributes splits the CloudWatch namespace into service namespace/name
// if prepended by AWS/. Otherwise, it returns the CloudWatch namespace as the
// service name with an empty service namespace
func toServiceAttributes(namespace string) (serviceNamespace, serviceName string) {
	before, after, ok := strings.Cut(namespace, namespaceDelimiter)
	if ok && strings.EqualFold(before, conventions.CloudProviderAWS.Value.AsString()) {
		return before, after
	}
	return "", namespace
}

// setResourceAttributes sets attributes on a metric data point from a cwMetric.
func setDataPointAttributes(m cWMetric, dp pmetric.SummaryDataPoint) {
	attrs := dp.Attributes()
	for k, v := range m.Dimensions {
		switch k {
		case dimensionInstanceID:
			attrs.PutStr(string(conventions.ServiceInstanceIDKey), v)
		default:
			attrs.PutStr(k, v)
		}
	}
}

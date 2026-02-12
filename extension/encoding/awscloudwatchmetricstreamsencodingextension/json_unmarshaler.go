// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension/internal/metadata"
)

const (
	attributeAWSCloudWatchMetricStreamName = "aws.cloudwatch.metric_stream_name"
	dimensionInstanceID                    = "InstanceId"
	namespaceDelimiter                     = "/"
)

var (
	errNoMetricName      = errors.New("cloudwatch metric is missing metric name field")
	errNoMetricNamespace = errors.New("cloudwatch metric is missing namespace field")
	errNoMetricUnit      = errors.New("cloudwatch metric is missing unit field")
	errNoMetricValue     = errors.New("cloudwatch metric is missing value")
)

type formatJSONUnmarshaler struct {
	buildInfo component.BuildInfo
}

var _ pmetric.Unmarshaler = (*formatJSONUnmarshaler)(nil)

// The cloudwatchMetric is the format for the CloudWatch metric stream records.
//
// More details can be found at:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-json.html
type cloudwatchMetric struct {
	// MetricStreamName is the name of the CloudWatch metric stream.
	MetricStreamName string `json:"metric_stream_name"`
	// AccountID is the AWS account ID associated with the metric.
	AccountID string `json:"account_id"`
	// Region is the AWS region for the metric.
	Region string `json:"region"`
	// Namespace is the CloudWatch namespace the metric is in.
	Namespace string `json:"namespace"`
	// MetricName is the name of the metric.
	MetricName string `json:"metric_name"`
	// Dimensions is a map of name/value pairs that help to
	// differentiate a metric.
	Dimensions map[string]string `json:"dimensions"`
	// Timestamp is the milliseconds since epoch for
	// the metric.
	Timestamp int64 `json:"timestamp"`
	// Value is the cloudwatchMetricValue, which has the min, max,
	// sum, and count.
	Value cloudwatchMetricValue `json:"value"`
	// Unit is the unit for the metric.
	//
	// More details can be found at:
	// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html
	Unit string `json:"unit"`
}

// The cloudwatchMetricValue is the actual values of the CloudWatch metric.
type cloudwatchMetricValue struct {
	isSet bool

	// Max is the highest value observed.
	Max float64
	// Min is the lowest value observed.
	Min float64
	// Sum is the sum of data points collected.
	Sum float64
	// Count is the number of data points.
	Count float64
	// Percentiles contains percentile fields (e.g., p50, p99, p99.9).
	Percentiles map[string]float64
}

// validateMetric validates that the cloudwatch metric has been unmarshalled correctly
func validateMetric(metric cloudwatchMetric) error {
	if metric.MetricName == "" {
		return errNoMetricName
	}
	if metric.Namespace == "" {
		return errNoMetricNamespace
	}
	if metric.Unit == "" {
		return errNoMetricUnit
	}
	if !metric.Value.isSet {
		return errNoMetricValue
	}
	return nil
}

func (v *cloudwatchMetricValue) UnmarshalJSON(data []byte) error {
	// All CloudWatch metric values are float64, so use a typed map
	rawFields := make(map[string]float64)
	if err := gojson.Unmarshal(data, &rawFields); err != nil {
		return err
	}

	v.Max = rawFields["max"]
	v.Min = rawFields["min"]
	v.Sum = rawFields["sum"]
	v.Count = rawFields["count"]

	// Other statistics (TM, WM, TC, TS, PR, IQM) are silently ignored.
	v.Percentiles = make(map[string]float64)
	for key, value := range rawFields {
		if len(key) > 1 && key[0] == 'p' {
			if _, err := strconv.ParseFloat(key[1:], 64); err == nil {
				v.Percentiles[key] = value
			}
		}
	}

	v.isSet = true
	return nil
}

// resourceKey stores the metric attributes
// that make a cloudwatchMetric unique to
// a resource
type resourceKey struct {
	metricStreamName string
	namespace        string
	accountID        string
	region           string
}

// metricKey stores the metric attributes
// that make a metric unique within
// a resource
type metricKey struct {
	name string
	unit string
}

func (c *formatJSONUnmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	var errs []error
	byResource := make(map[resourceKey]map[metricKey]pmetric.Metric)

	// Multiple metrics in each record separated by newline character
	scanner := bufio.NewScanner(bytes.NewReader(record))
	for datumIndex := 0; scanner.Scan(); datumIndex++ {
		var cwMetric cloudwatchMetric
		if err := gojson.Unmarshal(scanner.Bytes(), &cwMetric); err != nil {
			errs = append(errs, fmt.Errorf("error unmarshaling datum at index %d: %w", datumIndex, err))
			byResource = map[resourceKey]map[metricKey]pmetric.Metric{} // free the memory
			continue
		}
		if err := validateMetric(cwMetric); err != nil {
			errs = append(errs, fmt.Errorf("invalid cloudwatch metric at index %d: %w", datumIndex, err))
			byResource = map[resourceKey]map[metricKey]pmetric.Metric{} // free the memory
			continue
		}

		if len(errs) == 0 {
			// only add the metric if there are
			// no errors so far
			c.addMetricToResource(byResource, cwMetric)
		}
	}

	if err := scanner.Err(); err != nil {
		errs = append(errs, fmt.Errorf("error scanning for newline-delimited JSON: %w", err))
	}

	if len(errs) > 0 {
		return pmetric.Metrics{}, errors.Join(errs...)
	}

	return c.createMetrics(byResource), nil
}

// addMetricToResource adds a new cloudwatchMetric to the
// resource it belongs to according to resourceKey. It then
// sets the data point for the cloudwatchMetric.
func (*formatJSONUnmarshaler) addMetricToResource(
	byResource map[resourceKey]map[metricKey]pmetric.Metric,
	cwMetric cloudwatchMetric,
) {
	rKey := resourceKey{
		metricStreamName: cwMetric.MetricStreamName,
		namespace:        cwMetric.Namespace,
		accountID:        cwMetric.AccountID,
		region:           cwMetric.Region,
	}
	metrics, ok := byResource[rKey]
	if !ok {
		metrics = make(map[metricKey]pmetric.Metric)
		byResource[rKey] = metrics
	}

	mKey := metricKey{
		name: cwMetric.MetricName,
		unit: cwMetric.Unit,
	}
	metric, ok := metrics[mKey]
	if !ok {
		metric = pmetric.NewMetric()
		metric.SetName(mKey.name)
		metric.SetUnit(mKey.unit)
		metric.SetEmptySummary()
		metrics[mKey] = metric
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
		percentileFloat, _ := strconv.ParseFloat(key[1:], 64)
		q := dp.QuantileValues().AppendEmpty()
		q.SetQuantile(percentileFloat / 100) // Convert percentile to quantile
		q.SetValue(value)
	}
}

// createMetrics creates pmetric.Metrics based on
// on the extracted metrics of each resource
func (c *formatJSONUnmarshaler) createMetrics(
	byResource map[resourceKey]map[metricKey]pmetric.Metric,
) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for rKey, metricsMap := range byResource {
		rm := metrics.ResourceMetrics().AppendEmpty()
		setResourceAttributes(rKey, rm.Resource())
		scopeMetrics := rm.ScopeMetrics().AppendEmpty()
		scopeMetrics.Scope().SetName(metadata.ScopeName)
		scopeMetrics.Scope().SetVersion(c.buildInfo.Version)
		for _, metric := range metricsMap {
			metric.MoveTo(scopeMetrics.Metrics().AppendEmpty())
		}
	}
	return metrics
}

// setResourceAttributes sets attributes on a pcommon.Resource from a cloudwatchMetric.
func setResourceAttributes(rKey resourceKey, resource pcommon.Resource) {
	attributes := resource.Attributes()
	attributes.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attributes.PutStr(string(conventions.CloudAccountIDKey), rKey.accountID)
	attributes.PutStr(string(conventions.CloudRegionKey), rKey.region)
	serviceNamespace, serviceName := toServiceAttributes(rKey.namespace)
	if serviceNamespace != "" {
		attributes.PutStr(string(conventions.ServiceNamespaceKey), serviceNamespace)
	}
	attributes.PutStr(string(conventions.ServiceNameKey), serviceName)
	attributes.PutStr(attributeAWSCloudWatchMetricStreamName, rKey.metricStreamName)
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

// setDataPointAttributes sets attributes on a metric data point from a cloudwatchMetric.
func setDataPointAttributes(metric cloudwatchMetric, dp pmetric.SummaryDataPoint) {
	attrs := dp.Attributes()
	for k, v := range metric.Dimensions {
		switch k {
		case dimensionInstanceID:
			attrs.PutStr(string(conventions.ServiceInstanceIDKey), v)
		default:
			attrs.PutStr(k, v)
		}
	}
}

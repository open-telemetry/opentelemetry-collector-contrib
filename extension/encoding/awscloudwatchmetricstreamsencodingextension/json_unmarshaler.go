// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
)

const (
	attributeAWSCloudWatchMetricStreamName = "aws.cloudwatch.metric_stream_name"
	dimensionInstanceID                    = "InstanceId"
	namespaceDelimiter                     = "/"
)

var (
	_ pmetric.Unmarshaler = (*formatJSONUnmarshaler)(nil)
	_ streamUnmarshal     = (*formatJSONUnmarshaler)(nil)
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

func (r *formatJSONUnmarshaler) UnmarshalMetrics(record []byte) (pmetric.Metrics, error) {
	// Decode as a stream but flush all at once using flush options
	decoder, err := r.NewMetricsDecoder(bytes.NewReader(record), encoding.WithOffset(0), encoding.WithFlushBytes(0))
	if err != nil {
		return pmetric.Metrics{}, err
	}

	metrics, err := decoder.DecodeMetrics()
	if err != nil {
		// we must check for EOF with direct comparison and avoid wrapped EOF that can come from stream itself
		//nolint:errorlint
		if err == io.EOF {
			// EOF indicates no metrics were found, return any metrics that's available
			return metrics, nil
		}

		return pmetric.Metrics{}, err
	}

	return metrics, nil
}

func (r *formatJSONUnmarshaler) NewMetricsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.MetricsDecoder, error) {
	scanner, err := xstreamencoding.NewScannerHelper(reader, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create scanner helper: %w", err)
	}

	offsetF := func() int64 {
		return scanner.Offset()
	}

	decoderF := func() (pmetric.Metrics, error) {
		byResource := make(map[resourceKey]map[metricKey]pmetric.Metric)

		for {
			line, flush, err := scanner.ScanBytes()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return pmetric.Metrics{}, fmt.Errorf("error reading metric from stream: %w", err)
				}

				if len(line) == 0 {
					break
				}
			}

			var cwMetric cloudwatchMetric
			if err := gojson.Unmarshal(line, &cwMetric); err != nil {
				return pmetric.Metrics{}, fmt.Errorf("error unmarshaling cloudwatch metric: %w", err)
			}
			if err := validateMetric(cwMetric); err != nil {
				return pmetric.Metrics{}, fmt.Errorf("error validating cloudwatch metric: %w", err)
			}

			r.addMetricToResource(byResource, cwMetric)

			if flush {
				return r.createMetrics(byResource), nil
			}
		}

		if len(byResource) == 0 {
			return pmetric.NewMetrics(), io.EOF
		}

		return r.createMetrics(byResource), nil
	}

	return xstreamencoding.NewMetricsDecoderAdapter(decoderF, offsetF), nil
}

// addMetricToResource adds a new cloudwatchMetric to the resource it belongs to according to resourceKey.
// It then sets the data point for the cloudwatchMetric.
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

// createMetrics creates pmetric.Metrics based on the extracted metrics of each resource.
func (r *formatJSONUnmarshaler) createMetrics(
	byResource map[resourceKey]map[metricKey]pmetric.Metric,
) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for rKey, metricsMap := range byResource {
		rm := metrics.ResourceMetrics().AppendEmpty()
		setResourceAttributes(rKey, rm.Resource())
		scopeMetrics := rm.ScopeMetrics().AppendEmpty()
		scopeMetrics.Scope().SetName(metadata.ScopeName)
		scopeMetrics.Scope().SetVersion(r.buildInfo.Version)
		for _, metric := range metricsMap {
			metric.MoveTo(scopeMetrics.Metrics().AppendEmpty())
		}
	}
	return metrics
}

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

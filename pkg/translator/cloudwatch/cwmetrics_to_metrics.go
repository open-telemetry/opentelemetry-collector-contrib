// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatch // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/cloudwatch"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"

	expmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
)

// The cloudwatchMetric is the format for the CloudWatch metric stream records.
//
// More details can be found at:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-json.html
type cloudwatchMetric struct {
	MetricStreamName string                 `json:"metric_stream_name"`
	AccountID        string                 `json:"account_id"`
	Region           string                 `json:"region"`
	Namespace        string                 `json:"namespace"`
	MetricName       string                 `json:"metric_name"`
	Dimensions       map[string]string      `json:"dimensions"`
	Timestamp        int64                  `json:"timestamp"`
	Value            *cloudwatchMetricValue `json:"value"`
	Unit             string                 `json:"unit"`
}

// The cloudwatchMetricValue is the actual values of the CloudWatch metric.
type cloudwatchMetricValue struct {
	Max   float64 `json:"max"`
	Min   float64 `json:"min"`
	Sum   float64 `json:"sum"`
	Count float64 `json:"count"`
	Q1    float64 `json:"p99"`
	Q2    float64 `json:"p99.9"`
}

const (
	attributeAWSCloudWatchMetricStreamName = "aws.cloudwatch.metric_stream_name"
	dimensionInstanceID                    = "InstanceId"
	namespaceDelimiter                     = "/"
)

// find the valid unit values for cloudwatch metric in unit section of
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html#API_MetricDatum_Contents
var validUnitValues = map[string]bool{
	"Seconds":          true,
	"Microseconds":     true,
	"Milliseconds":     true,
	"Bytes":            true,
	"Kilobytes":        true,
	"Megabytes":        true,
	"Gigabytes":        true,
	"Terabytes":        true,
	"Bits":             true,
	"Kilobits":         true,
	"Megabits":         true,
	"Gigabits":         true,
	"Terabits":         true,
	"Percent":          true,
	"Count":            true,
	"Bytes/Second":     true,
	"Kilobytes/Second": true,
	"Megabytes/Second": true,
	"Gigabytes/Second": true,
	"Terabytes/Second": true,
	"Bits/Second":      true,
	"Kilobits/Second":  true,
	"Megabits/Second":  true,
	"Gigabits/Second":  true,
	"Terabits/Second":  true,
	"Count/Second":     true,
	"None":             true,
}

// isMetricValid validates that the cloudwatch metric has been unmarshalled correctly
func isMetricValid(metric cloudwatchMetric) (bool, error) {
	if metric.MetricName == "" {
		return false, errors.New("cloudwatch metric is missing metric name field")
	}
	if metric.Namespace == "" {
		return false, errors.New("cloudwatch metric is missing namespace field")
	}
	if _, exists := validUnitValues[metric.Unit]; !exists {
		return false, fmt.Errorf("cloudwatch metric unit '%s' value is not valid", metric.Unit)
	}
	if metric.Value == nil {
		return false, errors.New("cloudwatch metric is missing value")
	}
	return true, nil
}

// setResourceAttributes sets attributes on a pcommon.Resource from a cwMetric.
func setResourceAttributes(m cloudwatchMetric, resource pcommon.Resource) {
	attributes := resource.Attributes()
	attributes.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attributes.PutStr(conventions.AttributeCloudAccountID, m.AccountID)
	attributes.PutStr(conventions.AttributeCloudRegion, m.Region)
	serviceNamespace, serviceName := toServiceAttributes(m.Namespace)
	if serviceNamespace != "" {
		attributes.PutStr(conventions.AttributeServiceNamespace, serviceNamespace)
	}
	attributes.PutStr(conventions.AttributeServiceName, serviceName)
	attributes.PutStr(attributeAWSCloudWatchMetricStreamName, m.MetricStreamName)
}

// toServiceAttributes splits the CloudWatch namespace into service namespace/name
// if prepended by AWS/. Otherwise, it returns the CloudWatch namespace as the
// service name with an empty service namespace
func toServiceAttributes(namespace string) (serviceNamespace, serviceName string) {
	if strings.HasPrefix(namespace, strings.ToUpper(conventions.AttributeCloudProviderAWS)+"/") {
		parts := strings.SplitN(namespace, namespaceDelimiter, 2)
		return parts[0], parts[1]
	}
	return "", namespace
}

// setResourceAttributes sets attributes on a metric data point from
// a cloudwatchMetric dimensions
func setDataPointAttributes(m cloudwatchMetric, dp pmetric.SummaryDataPoint) {
	attrs := dp.Attributes()
	for k, v := range m.Dimensions {
		switch k {
		case dimensionInstanceID:
			attrs.PutStr(conventions.AttributeServiceInstanceID, v)
		default:
			attrs.PutStr(k, v)
		}
	}
}

// addJSONMetric transforms the cloudwatch metric to pmetric.Metric
// and appends it to pmetric.Metrics
func addJSONMetric(cwMetric cloudwatchMetric, metrics pmetric.Metrics) {
	rm := metrics.ResourceMetrics().AppendEmpty()
	setResourceAttributes(cwMetric, rm.Resource())

	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName(cwMetric.MetricName)
	metric.SetUnit(cwMetric.Unit)

	dp := metric.SetEmptySummary().DataPoints().AppendEmpty()
	dp.SetCount(uint64(cwMetric.Value.Count))
	dp.SetSum(cwMetric.Value.Sum)
	qv := dp.QuantileValues()
	qvs := qv.AppendEmpty()
	qvs.SetQuantile(99)
	qvs.SetValue(cwMetric.Value.Q1)
	qvs = qv.AppendEmpty()
	qvs.SetQuantile(99.9)
	qvs.SetValue(cwMetric.Value.Q2)
	qvs = qv.AppendEmpty()
	qvs.SetQuantile(0)
	qvs.SetValue(cwMetric.Value.Min)
	qvs = qv.AppendEmpty()
	qvs.SetQuantile(100)
	qvs.SetValue(cwMetric.Value.Max)

	// all dimensions are metric attributes
	setDataPointAttributes(cwMetric, dp)

	// cloudwatch timestamp is in ms, but the metric
	// expects it in ns
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(cwMetric.Timestamp)))
}

func addMetric(format Format, record []byte, metrics pmetric.Metrics) error {
	var cwMetric cloudwatchMetric
	switch format {
	case JSONFormat:
		decoder := json.NewDecoder(bytes.NewReader(record))
		if err := decoder.Decode(&cwMetric); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if valid, err := isMetricValid(cwMetric); !valid {
			return err
		}
		addJSONMetric(cwMetric, metrics)
		return nil
	case OTelFormat:
		// TODO Still needs to be implemented
		return errors.New("implement me")
	default:
		return fmt.Errorf("error decoding cloudwatch metric using format %s", format)
	}
}

func UnmarshalMetrics(format Format, record []byte) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	// Split metrics by delimiter
	cwMetrics := bytes.Split(record, []byte("\n"))
	for datumIndex, datum := range cwMetrics {
		err := addMetric(format, datum, metrics)
		if err != nil {
			return pmetric.Metrics{},
				fmt.Errorf("unable to unmarshal datum [%d] into cloudwatch metric: %w", datumIndex, err)
		}
	}

	if metrics.MetricCount() == 0 {
		return metrics, errors.New("no resource metrics could be obtained from the record")
	}

	metrics = expmetrics.Merge(pmetric.NewMetrics(), metrics)
	return metrics, nil
}

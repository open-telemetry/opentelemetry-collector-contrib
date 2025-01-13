package cloudwatch // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/cloudwatch"

import (
	"bytes"
	"encoding/json"
	"errors"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"io"
	"strings"
	"time"

	expmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
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
}

const (
	attributeAWSCloudWatchMetricStreamName = "aws.cloudwatch.metric_stream_name"
	dimensionInstanceID                    = "InstanceId"
	namespaceDelimiter                     = "/"
)

var (
	errInvalidMetricRecord = errors.New("no resource metrics were obtained from the record")
)

// isMetricValid validates that the cloudwatch metric has been unmarshalled correctly
func isMetricValid(metric cloudwatchMetric) bool {
	return metric.MetricName != "" && metric.Namespace != "" && metric.Unit != "" && metric.Value != nil
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
	index := strings.Index(namespace, namespaceDelimiter)
	if index != -1 && strings.EqualFold(namespace[:index], conventions.AttributeCloudProviderAWS) {
		return namespace[:index], namespace[index+1:]
	}
	return "", namespace
}

// setResourceAttributes sets attributes on a metric data point from a cloudwatchMetric.
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

func UnmarshalMetrics(record []byte, logger *zap.Logger) (pmetric.Metrics, error) {
	decoder := json.NewDecoder(bytes.NewReader(record))
	metrics := pmetric.NewMetrics()
	for datumIndex := 0; ; datumIndex++ {
		var cwMetric cloudwatchMetric
		if err := decoder.Decode(&cwMetric); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			logger.Error(
				"Unable to unmarshal input",
				zap.Int("datum_index", datumIndex),
				zap.Error(err),
			)
			continue
		}
		if !isMetricValid(cwMetric) {
			logger.Error(
				"Invalid cloudwatch cwMetric",
				zap.Int("datum_index", datumIndex),
			)
			continue
		}

		rm := metrics.ResourceMetrics().AppendEmpty()
		setResourceAttributes(cwMetric, rm.Resource())

		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName(cwMetric.MetricName)
		metric.SetUnit(cwMetric.Unit)

		dp := metric.SetEmptySummary().DataPoints().AppendEmpty()
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
	}

	if metrics.MetricCount() == 0 {
		return metrics, errInvalidMetricRecord
	}

	metrics = expmetrics.Merge(pmetric.NewMetrics(), metrics)
	return metrics, nil
}

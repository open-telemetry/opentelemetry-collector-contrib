// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"

import (
	"bytes"
	"encoding/json"
	"errors"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
)

const (
	TypeStr         = "cwmetrics"
	recordDelimiter = "\n"
)

var (
	errInvalidRecords = errors.New("record format invalid")
)

// Unmarshaler for the CloudWatch Metric Stream JSON record format.
//
// More details can be found at:
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-json.html
type Unmarshaler struct {
	logger *zap.Logger
}

var _ unmarshaler.MetricsUnmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger) *Unmarshaler {
	return &Unmarshaler{logger}
}

// Unmarshal deserializes the records into cWMetrics and uses the
// resourceMetricsBuilder to group them into a single pmetric.Metrics.
// Skips invalid cWMetrics received in the record and
func (u Unmarshaler) Unmarshal(records [][]byte) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	builders := make(map[resourceAttributes]*resourceMetricsBuilder)
	for recordIndex, record := range records {
		// Multiple metrics in each record separated by newline character
		for datumIndex, datum := range bytes.Split(record, []byte(recordDelimiter)) {
			if len(datum) > 0 {
				var metric cWMetric
				err := json.Unmarshal(datum, &metric)
				if err != nil {
					u.logger.Error(
						"Unable to unmarshal input",
						zap.Error(err),
						zap.Int("datum_index", datumIndex),
						zap.Int("record_index", recordIndex),
					)
					continue
				}
				if !u.isValid(metric) {
					u.logger.Error(
						"Invalid metric",
						zap.Int("datum_index", datumIndex),
						zap.Int("record_index", recordIndex),
					)
					continue
				}
				attrs := resourceAttributes{
					metricStreamName: metric.MetricStreamName,
					namespace:        metric.Namespace,
					accountID:        metric.AccountID,
					region:           metric.Region,
				}
				mb, ok := builders[attrs]
				if !ok {
					mb = newResourceMetricsBuilder(md, attrs)
					builders[attrs] = mb
				}
				mb.AddMetric(metric)
			}
		}
	}

	if len(builders) == 0 {
		return pmetric.NewMetrics(), errInvalidRecords
	}

	return md, nil
}

// isValid validates that the cWMetric has been unmarshalled correctly.
func (u Unmarshaler) isValid(metric cWMetric) bool {
	return metric.MetricName != "" && metric.Namespace != "" && metric.Unit != "" && metric.Value != nil
}

// Type of the serialized messages.
func (u Unmarshaler) Type() string {
	return TypeStr
}

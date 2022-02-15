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

package cwmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"
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
// See https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-metric-streams-formats-json.html
type Unmarshaler struct {
	logger *zap.Logger
}

var _ unmarshaler.MetricsUnmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger) *Unmarshaler {
	return &Unmarshaler{logger}
}

// Unmarshal deserializes the records into cWMetrics and uses the
// resourceMetricsBuilder to group them into a single pdata.Metrics.
// Skips invalid cWMetrics received in the record and
func (u Unmarshaler) Unmarshal(records [][]byte) (pdata.Metrics, error) {
	builders := make(map[string]*resourceMetricsBuilder)
	for recordIndex, record := range records {
		// Multiple metrics in each record separated by newline character
		for datumIndex, datum := range bytes.Split(record, []byte(recordDelimiter)) {
			if len(datum) > 0 {
				var metric cWMetric
				err := json.Unmarshal(datum, &metric)
				if err != nil || !u.isValid(metric) {
					u.logger.Error(
						"Invalid metric",
						zap.Error(err),
						zap.Int("datum_index", datumIndex),
						zap.Int("record_index", recordIndex),
					)
					continue
				}
				resourceKey := u.toResourceKey(metric)
				mb, ok := builders[resourceKey]
				if !ok {
					mb = newResourceMetricsBuilder(
						metric.MetricStreamName,
						metric.AccountID,
						metric.Region,
						metric.Namespace,
					)
					builders[resourceKey] = mb
				}
				mb.AddMetric(metric)
			}
		}
	}

	if len(builders) == 0 {
		return pdata.NewMetrics(), errInvalidRecords
	}

	md := pdata.NewMetrics()
	for _, builder := range builders {
		builder.Build(md.ResourceMetrics().AppendEmpty())
	}

	return md, nil
}

// isValid validates that the cWMetric has been unmarshalled correctly.
func (u Unmarshaler) isValid(metric cWMetric) bool {
	return metric.MetricName != "" && metric.Namespace != "" && metric.Unit != "" && metric.Value != nil
}

// toResourceKey combines the metric stream name, namespace, account id, and region into a string key
func (u Unmarshaler) toResourceKey(metric cWMetric) string {
	return fmt.Sprintf("%s::%s::%s::%s", metric.MetricStreamName, metric.Namespace, metric.AccountID, metric.Region)
}

// Type of the serialized messages.
func (u Unmarshaler) Type() string {
	return TypeStr
}

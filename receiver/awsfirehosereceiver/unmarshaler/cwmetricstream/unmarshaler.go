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

package cwmetricstream // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/unmarshaler/cwmetricstream"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/unmarshaler"
)

const (
	Encoding        = "cwmetrics"
	recordDelimiter = "\n"
)

var (
	errInvalidRecords = errors.New("record format invalid")
)

type Unmarshaler struct {
}

var _ unmarshaler.MetricsUnmarshaler = (*Unmarshaler)(nil)

func NewUnmarshaler() *Unmarshaler {
	return &Unmarshaler{}
}

func (u Unmarshaler) Unmarshal(records [][]byte) (pdata.Metrics, error) {
	builders := make(map[string]*resourceMetricsBuilder)
	for _, record := range records {
		// multiple metrics in each record separated by newline character
		for _, datum := range bytes.Split(record, []byte(recordDelimiter)) {
			if len(datum) > 0 {
				var metric cWMetric
				err := json.Unmarshal(datum, &metric)
				if err != nil || !u.isValid(metric) {
					continue
				}
				resourceKey := u.toResourceKey(metric)
				mb, ok := builders[resourceKey]
				if !ok {
					mb = newResourceMetricsBuilder(
						metric.MetricStreamName,
						metric.AccountId,
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
		builder.Build().CopyTo(md.ResourceMetrics().AppendEmpty())
	}

	return md, nil
}

// isValid validates that the cWMetric has been unmarshalled correctly.
func (u Unmarshaler) isValid(metric cWMetric) bool {
	return metric.MetricName != "" && metric.Namespace != "" && metric.Unit != "" && metric.Value != nil
}

// toResourceKey combines the metric stream name, namespace, account id, and region into a string key
func (u Unmarshaler) toResourceKey(metric cWMetric) string {
	return fmt.Sprintf("%s::%s::%s::%s", metric.MetricStreamName, metric.Namespace, metric.AccountId, metric.Region)
}

func (u Unmarshaler) Encoding() string {
	return Encoding
}

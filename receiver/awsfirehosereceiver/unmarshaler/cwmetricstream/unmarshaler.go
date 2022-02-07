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
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/unmarshaler"
)

const (
	Encoding = "cwmetrics"
)

var (
	errInvalidRecords = fmt.Errorf("record format invalid")
)

type Unmarshaler struct {
}

var _ unmarshaler.MetricsUnmarshaler = (*Unmarshaler)(nil)

func (u Unmarshaler) Unmarshal(record []byte, logger *zap.Logger) (pdata.Metrics, error) {
	metricsBuilders := make(map[string]*namespacedMetricsBuilder)
	// multiple metrics in each record separated by newline character
	for _, datum := range bytes.Split(record, []byte("\n")) {
		if len(datum) > 0 {
			var metric cWMetric
			// TODO: use sonic unmarshaler
			err := json.Unmarshal(datum, &metric)
			if err != nil || !u.isValid(metric) {
				continue
			}
			logger.Debug("successfully unmarshalled", zap.Reflect("cwmetric", metric))
			mb, ok := metricsBuilders[metric.Namespace]
			if !ok {
				mb = newNamespacedMetricsBuilder()
				metricsBuilders[metric.Namespace] = mb
			}
			mb.AddMetric(metric)
		}
	}

	if len(metricsBuilders) == 0 {
		return pdata.NewMetrics(), errInvalidRecords
	}

	md := pdata.NewMetrics()
	for _, mb := range metricsBuilders {
		mb.Build().CopyTo(md.ResourceMetrics().AppendEmpty())
	}

	return md, nil
}

func (u Unmarshaler) isValid(metric cWMetric) bool {
	return metric.MetricName != "" && metric.Namespace != "" && metric.Unit != "" && metric.Value != nil
}

func (u Unmarshaler) Encoding() string {
	return Encoding
}

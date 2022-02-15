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

type cWMetric struct {
	MetricStreamName string            `json:"metric_stream_name"`
	AccountID        string            `json:"account_id"`
	Region           string            `json:"region"`
	Namespace        string            `json:"namespace"`
	MetricName       string            `json:"metric_name"`
	Dimensions       map[string]string `json:"dimensions"`
	Timestamp        int64             `json:"timestamp"` // in epoch milliseconds
	Value            *cWMetricValue    `json:"value"`
	Unit             string            `json:"unit"`
}

type cWMetricValue struct {
	Max   float64 `json:"max"`
	Min   float64 `json:"min"`
	Sum   float64 `json:"sum"`
	Count float64 `json:"count"`
}

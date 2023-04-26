// Copyright The OpenTelemetry Authors
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

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/models"

// ClusterProperties represents the top level json returned by the /metrics/json endpoint
type ClusterProperties struct {
	Version    string               `json:"version"`
	Gauges     map[string]Gauge     `json:"gauges"`
	Counters   map[string]Counter   `json:"counters"`
	Histograms map[string]Histogram `json:"histograms"`
	Timers     map[string]Timer     `json:"timers"`
}

type Gauge struct {
	Value float64 `json:"value"`
}

type Counter struct {
	Count int `json:"count"`
}

type Histogram struct {
	Count  int     `json:"count"`
	Max    int     `json:"max"`
	Mean   float64 `json:"mean"`
	Min    int     `json:"min"`
	P50    float64 `json:"p50"`
	P75    float64 `json:"p75"`
	P95    float64 `json:"p95"`
	P98    float64 `json:"p98"`
	P99    float64 `json:"p99"`
	P999   float64 `json:"p999"`
	Stddev float64 `json:"stddev"`
}

type Timer struct {
	Count int     `json:"count"`
	Mean  float64 `json:"mean"`
}

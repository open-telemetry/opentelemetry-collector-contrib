// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	Count int64 `json:"count"`
}

type Histogram struct {
	Count  int64   `json:"count"`
	Max    int64   `json:"max"`
	Mean   float64 `json:"mean"`
	Min    int64   `json:"min"`
	P50    float64 `json:"p50"`
	P75    float64 `json:"p75"`
	P95    float64 `json:"p95"`
	P98    float64 `json:"p98"`
	P99    float64 `json:"p99"`
	P999   float64 `json:"p999"`
	Stddev float64 `json:"stddev"`
}

type Timer struct {
	Count int64   `json:"count"`
	Mean  float64 `json:"mean"`
}

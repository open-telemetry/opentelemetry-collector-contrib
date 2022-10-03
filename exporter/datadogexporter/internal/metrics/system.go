// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"strings"

	"gopkg.in/zorkian/go-datadog-api.v2"
)

// copyMetric copies the metric from src by giving it a new name. If div differs from 1, it scales all
// data points. If interval is non-zero, it sets it as the metric's interval.
//
// Warning: this is not a deep copy. Only some fields are fully copied, others remain shared. This is intentional.
// Do not alter the returned metric (or the source one) after copying.
func copyMetric(src datadog.Metric, name string, div float64, interval int) datadog.Metric {
	cp := src
	cp.Metric = &name
	if interval != 0 {
		cp.Interval = &interval
	}
	t := "gauge"
	cp.Type = &t
	if div == 0 || div == 1 || len(src.Points) == 0 {
		// division by 0 or 1 should not have an impact
		return cp
	}
	cp.Points = make([]datadog.DataPoint, len(src.Points))
	for i, dp := range src.Points {
		cp.Points[i][0] = dp[0]
		if dp[1] != nil {
			newdp := *dp[1] / div
			cp.Points[i][1] = &newdp
		}
	}
	return cp
}

const (
	// divMebibytes specifies the number of bytes in a mebibyte.
	divMebibytes = 1024 * 1024
	// divPercentage specifies the division necessary for converting fractions to percentages.
	divPercentage = 0.01
)

// extractSystemMetrics takes an OpenTelemetry metric m and extracts Datadog system metrics from it,
// if m is a valid system metric. The boolean argument reports whether any system metrics were extractd.
func extractSystemMetrics(m datadog.Metric) (ms []datadog.Metric, ok bool) {
	var series []datadog.Metric
	switch *m.Metric {
	case "system.cpu.load_average.1m":
		series = append(series, copyMetric(m, "system.load.1", 1, 1))
	case "system.cpu.load_average.5m":
		series = append(series, copyMetric(m, "system.load.5", 1, 1))
	case "system.cpu.load_average.15m":
		series = append(series, copyMetric(m, "system.load.15", 1, 1))
	case "system.cpu.utilization":
		for _, tag := range m.Tags {
			switch tag {
			case "state:idle":
				series = append(series, copyMetric(m, "system.cpu.idle", divPercentage, 1))
			case "state:user":
				series = append(series, copyMetric(m, "system.cpu.user", divPercentage, 1))
			case "state:system":
				series = append(series, copyMetric(m, "system.cpu.system", divPercentage, 1))
			case "state:wait":
				series = append(series, copyMetric(m, "system.cpu.iowait", divPercentage, 1))
			case "state:steal":
				series = append(series, copyMetric(m, "system.cpu.stolen", divPercentage, 0))
			}
		}
	case "system.memory.usage":
		series = append(series, copyMetric(m, "system.mem.total", divMebibytes, 1))
		for _, tag := range m.Tags {
			switch tag {
			case "state:free", "state:cached", "state:buffered":
				series = append(series, copyMetric(m, "system.mem.usable", divMebibytes, 1))
			}
		}
	case "system.network.io":
		for _, tag := range m.Tags {
			switch tag {
			case "direction:receive":
				series = append(series, copyMetric(m, "system.net.bytes_rcvd", 1, 1))
			case "direction:transmit":
				series = append(series, copyMetric(m, "system.net.bytes_sent", 1, 1))
			}
		}
	case "system.paging.usage":
		for _, tag := range m.Tags {
			switch tag {
			case "state:free":
				series = append(series, copyMetric(m, "system.swap.free", divMebibytes, 1))
			case "state:used":
				series = append(series, copyMetric(m, "system.swap.used", divMebibytes, 1))
			}
		}
	case "system.filesystem.utilization":
		series = append(series, copyMetric(m, "system.disk.in_use", 1, 0))
	}
	return series, len(series) > 0
}

// otelNamespacePrefix specifies the namespace used for OpenTelemetry host metrics.
const otelNamespacePrefix = "otel."

// PrepareSystemMetrics prepends system hosts metrics with the otel.* prefix to identify
// them as part of the Datadog OpenTelemetry Integration. It also extracts Datadog compatible
// system metrics and returns the full set of metrics to be used.
func PrepareSystemMetrics(ms []datadog.Metric) []datadog.Metric {
	series := ms
	for i, m := range ms {
		if !strings.HasPrefix(*m.Metric, "system.") &&
			!strings.HasPrefix(*m.Metric, "process.") {
			// not a system metric
			continue
		}
		if newms, ok := extractSystemMetrics(m); ok {
			series = append(series, newms...)
		}
		// all existing system metrics need to be prepended
		newname := otelNamespacePrefix + *m.Metric
		series[i].Metric = &newname
	}
	return series
}

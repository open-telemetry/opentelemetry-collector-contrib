// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"strings"

	zorkian "gopkg.in/zorkian/go-datadog-api.v2"
)

// copyZorkianSystemMetric copies the metric from src by giving it a new name. If div differs from 1, it scales all
// data points.
//
// Warning: this is not a deep copy. Only some fields are fully copied, others remain shared. This is intentional.
// Do not alter the returned metric (or the source one) after copying.
func copyZorkianSystemMetric(src zorkian.Metric, name string, div float64) zorkian.Metric {
	return copyZorkianSystemMetricWithUnit(src, name, div, "")
}

func copyZorkianSystemMetricWithUnit(src zorkian.Metric, name string, div float64, unit string) zorkian.Metric {
	cp := src
	cp.Metric = &name
	i := 1
	cp.Interval = &i
	t := "gauge"
	cp.Type = &t
	if div == 0 || div == 1 || len(src.Points) == 0 {
		// division by 0 or 1 should not have an impact
		return cp
	}
	if unit != "" {
		cp.Unit = &unit
	}
	cp.Points = make([]zorkian.DataPoint, len(src.Points))
	for i, dp := range src.Points {
		cp.Points[i][0] = dp[0]
		if dp[1] != nil {
			newdp := *dp[1] / div
			cp.Points[i][1] = &newdp
		}
	}
	return cp
}

// extractZorkianSystemMetric takes an OpenTelemetry metric m and extracts Datadog system metrics from it,
// if m is a valid system metric. The boolean argument reports whether any system metrics were extractd.
func extractZorkianSystemMetric(m zorkian.Metric) []zorkian.Metric {
	var series []zorkian.Metric
	switch *m.Metric {
	case "system.cpu.load_average.1m":
		series = append(series, copyZorkianSystemMetric(m, "system.load.1", 1))
	case "system.cpu.load_average.5m":
		series = append(series, copyZorkianSystemMetric(m, "system.load.5", 1))
	case "system.cpu.load_average.15m":
		series = append(series, copyZorkianSystemMetric(m, "system.load.15", 1))
	case "system.cpu.utilization":
		for _, tag := range m.Tags {
			switch tag {
			case "state:idle":
				series = append(series, copyZorkianSystemMetric(m, "system.cpu.idle", divPercentage))
			case "state:user":
				series = append(series, copyZorkianSystemMetric(m, "system.cpu.user", divPercentage))
			case "state:system":
				series = append(series, copyZorkianSystemMetric(m, "system.cpu.system", divPercentage))
			case "state:wait":
				series = append(series, copyZorkianSystemMetric(m, "system.cpu.iowait", divPercentage))
			case "state:steal":
				series = append(series, copyZorkianSystemMetric(m, "system.cpu.stolen", divPercentage))
			}
		}
	case "system.memory.usage":
		series = append(series, copyZorkianSystemMetric(m, "system.mem.total", divMebibytes))
		for _, tag := range m.Tags {
			switch tag {
			case "state:free", "state:cached", "state:buffered":
				series = append(series, copyZorkianSystemMetric(m, "system.mem.usable", divMebibytes))
			}
		}
	case "system.network.io":
		for _, tag := range m.Tags {
			switch tag {
			case "direction:receive":
				series = append(series, copyZorkianSystemMetric(m, "system.net.bytes_rcvd", 1))
			case "direction:transmit":
				series = append(series, copyZorkianSystemMetric(m, "system.net.bytes_sent", 1))
			}
		}
	case "system.paging.usage":
		for _, tag := range m.Tags {
			switch tag {
			case "state:free":
				series = append(series, copyZorkianSystemMetric(m, "system.swap.free", divMebibytes))
			case "state:used":
				series = append(series, copyZorkianSystemMetric(m, "system.swap.used", divMebibytes))
			}
		}
	case "system.filesystem.utilization":
		series = append(series, copyZorkianSystemMetric(m, "system.disk.in_use", 1))
	}
	return series
}

// PrepareZorkianSystemMetrics prepends system hosts metrics with the otel.* prefix to identify
// them as part of the Datadog OpenTelemetry Integration. It also extracts Datadog compatible
// system metrics and returns the full set of metrics to be used.
func PrepareZorkianSystemMetrics(ms []zorkian.Metric) []zorkian.Metric {
	series := ms
	for i, m := range ms {
		if !strings.HasPrefix(*m.Metric, "system.") &&
			!strings.HasPrefix(*m.Metric, "process.") {
			// not a system metric
			continue
		}
		series = append(series, extractZorkianSystemMetric(m)...)
		// all existing system metrics need to be prepended
		newname := otelNamespacePrefix + *m.Metric
		series[i].Metric = &newname
	}
	return series
}

// PrepareZorkianContainerMetrics converts OTEL container.* metrics to Datadog container
// metrics.
func PrepareZorkianContainerMetrics(ms []zorkian.Metric) []zorkian.Metric {
	series := ms
	for _, m := range ms {
		if !strings.HasPrefix(*m.Metric, "container.") {
			// not what we're looking for
			continue
		}
		switch *m.Metric {
		case "container.cpu.usage.total":
			series = append(series, copyZorkianSystemMetricWithUnit(m, "container.cpu.usage", 1, "nanocore"))
		case "container.cpu.usage.usermode":
			series = append(series, copyZorkianSystemMetricWithUnit(m, "container.cpu.user", 1, "nanocore"))
		case "container.cpu.usage.system":
			series = append(series, copyZorkianSystemMetricWithUnit(m, "container.cpu.system", 1, "nanocore"))
		case "container.cpu.throttling_data.throttled_time":
			series = append(series, copyZorkianSystemMetric(m, "container.cpu.throttled", 1))
		case "container.cpu.throttling_data.throttled_periods":
			series = append(series, copyZorkianSystemMetric(m, "container.cpu.throttled.periods", 1))
		case "container.memory.usage.total":
			series = append(series, copyZorkianSystemMetric(m, "container.memory.usage", 1))
		case "container.memory.active_anon":
			series = append(series, copyZorkianSystemMetric(m, "container.memory.kernel", 1))
		case "container.memory.hierarchical_memory_limit":
			series = append(series, copyZorkianSystemMetric(m, "container.memory.limit", 1))
		case "container.memory.usage.limit":
			series = append(series, copyZorkianSystemMetric(m, "container.memory.soft_limit", 1))
		case "container.memory.total_cache":
			series = append(series, copyZorkianSystemMetric(m, "container.memory.cache", 1))
		case "container.memory.total_swap":
			series = append(series, copyZorkianSystemMetric(m, "container.memory.swap", 1))
		case "container.blockio.io_service_bytes_recursive":
			for _, tag := range m.Tags {
				switch tag {
				case "operation:write":
					series = append(series, copyZorkianSystemMetric(m, "container.io.write", 1))
				case "operation:read":
					series = append(series, copyZorkianSystemMetric(m, "container.io.read", 1))
				}
			}
		case "container.blockio.io_serviced_recursive":
			for _, tag := range m.Tags {
				switch tag {
				case "operation:write":
					series = append(series, copyZorkianSystemMetric(m, "container.io.write.operations", 1))
				case "operation:read":
					series = append(series, copyZorkianSystemMetric(m, "container.io.read.operations", 1))
				}
			}
		case "container.network.io.usage.tx_bytes":
			series = append(series, copyZorkianSystemMetric(m, "container.net.sent", 1))
		case "container.network.io.usage.tx_packets":
			series = append(series, copyZorkianSystemMetric(m, "container.net.sent.packets", 1))
		case "container.network.io.usage.rx_bytes":
			series = append(series, copyZorkianSystemMetric(m, "container.net.rcvd", 1))
		case "container.network.io.usage.rx_packets":
			series = append(series, copyZorkianSystemMetric(m, "container.net.rcvd.packets", 1))
		}
	}
	return series
}

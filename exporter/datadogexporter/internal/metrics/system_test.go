// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"fmt"
	"math"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/stretchr/testify/require"
)

func TestCopyMetric(t *testing.T) {
	sptr := func(s string) *string { return &s }
	dp := func(a int64, b float64) datadogV2.MetricPoint {
		return datadogV2.MetricPoint{
			Timestamp: datadog.PtrInt64(a),
			Value:     datadog.PtrFloat64(b),
		}
	}

	var unspecified = datadogV2.METRICINTAKETYPE_UNSPECIFIED
	var gauge = datadogV2.METRICINTAKETYPE_GAUGE

	t.Run("renaming", func(t *testing.T) {
		require.EqualValues(t, copySystemMetric(datadogV2.MetricSeries{
			Metric:    "oldname",
			Points:    []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
			Type:      &unspecified,
			Resources: []datadogV2.MetricResource{{Name: sptr("oldhost"), Type: sptr("host")}},
			Tags:      []string{"x", "y", "z"},
			Unit:      sptr("oldunit"),
		}, "newname", 1), datadogV2.MetricSeries{
			Metric:    "newname",
			Points:    []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
			Type:      &gauge,
			Resources: []datadogV2.MetricResource{{Name: sptr("oldhost"), Type: sptr("host")}},
			Tags:      []string{"x", "y", "z"},
			Unit:      sptr("oldunit"),
		})

		require.EqualValues(t, copySystemMetric(datadogV2.MetricSeries{
			Metric:    "oldname",
			Points:    []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
			Type:      &unspecified,
			Resources: []datadogV2.MetricResource{{Name: sptr("oldhost"), Type: sptr("host")}},
			Tags:      []string{"x", "y", "z"},
			Unit:      sptr("oldunit"),
		}, "", 1), datadogV2.MetricSeries{
			Metric:    "",
			Points:    []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
			Type:      &gauge,
			Resources: []datadogV2.MetricResource{{Name: sptr("oldhost"), Type: sptr("host")}},
			Tags:      []string{"x", "y", "z"},
			Unit:      sptr("oldunit"),
		})

		require.EqualValues(t, copySystemMetric(datadogV2.MetricSeries{
			Metric:    "",
			Points:    []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
			Type:      &unspecified,
			Resources: []datadogV2.MetricResource{{Name: sptr("oldhost"), Type: sptr("host")}},
			Tags:      []string{"x", "y", "z"},
			Unit:      sptr("oldunit"),
		}, "", 1), datadogV2.MetricSeries{
			Metric:    "",
			Points:    []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
			Type:      &gauge,
			Resources: []datadogV2.MetricResource{{Name: sptr("oldhost"), Type: sptr("host")}},
			Tags:      []string{"x", "y", "z"},
			Unit:      sptr("oldunit"),
		})
	})

	t.Run("division", func(t *testing.T) {
		for _, tt := range []struct {
			in, out []datadogV2.MetricPoint
			div     float64
		}{
			{
				in:  []datadogV2.MetricPoint{dp(0, 0), dp(1, 20)},
				div: 0,
				out: []datadogV2.MetricPoint{dp(0, 0), dp(1, 20)},
			},
			{
				in:  []datadogV2.MetricPoint{dp(0, 0), dp(1, 20)},
				div: 1,
				out: []datadogV2.MetricPoint{dp(0, 0), dp(1, 20)},
			},
			{
				in:  []datadogV2.MetricPoint{dp(1, 0), {}},
				div: 2,
				out: []datadogV2.MetricPoint{dp(1, 0), {}},
			},
			{
				in:  []datadogV2.MetricPoint{dp(0, 0), dp(1, 20)},
				div: 10,
				out: []datadogV2.MetricPoint{dp(0, 0), dp(1, 2)},
			},
			{
				in:  []datadogV2.MetricPoint{dp(1, 0), dp(55, math.MaxFloat64)},
				div: 1024 * 1024.5,
				out: []datadogV2.MetricPoint{dp(1, 0), dp(55, 1.713577063947272e+302)},
			},
			{
				in:  []datadogV2.MetricPoint{dp(1, 0), dp(55, 20)},
				div: math.MaxFloat64,
				out: []datadogV2.MetricPoint{dp(1, 0), dp(55, 1.1125369292536009e-307)},
			},
		} {
			t.Run(fmt.Sprintf("%.0f", tt.div), func(t *testing.T) {
				require.EqualValues(t,
					copySystemMetric(datadogV2.MetricSeries{Points: tt.in}, "", tt.div),
					datadogV2.MetricSeries{Metric: "", Type: &gauge, Points: tt.out},
				)
			})
		}
	})
}

func TestExtractSystemMetrics(t *testing.T) {
	dp := func(a int64, b float64) datadogV2.MetricPoint {
		return datadogV2.MetricPoint{
			Timestamp: datadog.PtrInt64(a),
			Value:     datadog.PtrFloat64(b),
		}
	}

	var gauge = datadogV2.METRICINTAKETYPE_GAUGE

	for _, tt := range []struct {
		in  datadogV2.MetricSeries
		out []datadogV2.MetricSeries
	}{
		{
			in: datadogV2.MetricSeries{
				Metric: "system.cpu.load_average.1m",
				Points: []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.load.1",
				Points: []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.cpu.load_average.5m",
				Points: []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.load.5",
				Points: []datadogV2.MetricPoint{dp(1, 2), dp(3, 4)},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.cpu.load_average.15m",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.load.15",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.cpu.utilization",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:idle"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.cpu.idle",
				Points: []datadogV2.MetricPoint{dp(2, 200), dp(3, 400)},
				Tags:   []string{"state:idle"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.cpu.utilization",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:user"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.cpu.user",
				Points: []datadogV2.MetricPoint{dp(2, 200), dp(3, 400)},
				Tags:   []string{"state:user"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.cpu.utilization",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:system"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.cpu.system",
				Points: []datadogV2.MetricPoint{dp(2, 200), dp(3, 400)},
				Tags:   []string{"state:system"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.cpu.utilization",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:wait"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.cpu.iowait",
				Points: []datadogV2.MetricPoint{dp(2, 200), dp(3, 400)},
				Tags:   []string{"state:wait"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.cpu.utilization",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:steal"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.cpu.stolen",
				Points: []datadogV2.MetricPoint{dp(2, 200), dp(3, 400)},
				Tags:   []string{"state:steal"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.memory.usage",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:other"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.mem.total",
				Points: []datadogV2.MetricPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:   []string{"state:other"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.memory.usage",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:free"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.mem.total",
				Points: []datadogV2.MetricPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:   []string{"state:free"},
				Type:   &gauge,
			}, {
				Metric: "system.mem.usable",
				Points: []datadogV2.MetricPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:   []string{"state:free"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.memory.usage",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:cached"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.mem.total",
				Points: []datadogV2.MetricPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:   []string{"state:cached"},
				Type:   &gauge,
			}, {
				Metric: "system.mem.usable",
				Points: []datadogV2.MetricPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:   []string{"state:cached"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.memory.usage",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:buffered"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.mem.total",
				Points: []datadogV2.MetricPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:   []string{"state:buffered"},
				Type:   &gauge,
			}, {
				Metric: "system.mem.usable",
				Points: []datadogV2.MetricPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:   []string{"state:buffered"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.network.io",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"direction:receive"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.net.bytes_rcvd",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"direction:receive"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.network.io",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"direction:transmit"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.net.bytes_sent",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"direction:transmit"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.paging.usage",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:free"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.swap.free",
				Points: []datadogV2.MetricPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:   []string{"state:free"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.paging.usage",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:used"},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.swap.used",
				Points: []datadogV2.MetricPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:   []string{"state:used"},
				Type:   &gauge,
			}},
		},
		{
			in: datadogV2.MetricSeries{
				Metric: "system.filesystem.utilization",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
			},
			out: []datadogV2.MetricSeries{{
				Metric: "system.disk.in_use",
				Points: []datadogV2.MetricPoint{dp(2, 2), dp(3, 4)},
				Type:   &gauge,
			}},
		},
	} {
		t.Run("", func(t *testing.T) {
			out := extractSystemMetrics(tt.in)
			require.EqualValues(t, tt.out, out, fmt.Sprintf("%s[%#v]", tt.in.Metric, tt.in.Tags))
		})
	}
}

func TestPrepareSystemMetrics(t *testing.T) {
	var gauge = datadogV2.METRICINTAKETYPE_GAUGE

	m := func(name string, tags []string, points ...float64) datadogV2.MetricSeries {
		met := datadogV2.MetricSeries{
			Metric: name,
			Tags:   tags,
		}
		if len(points)%2 != 0 {
			t.Fatal("Number of data point arguments passed to function must be even.")
		}
		met.Points = make([]datadogV2.MetricPoint, 0, len(points)/2)
		for i := 0; i < len(points); i += 2 {
			ts := int64(points[i])
			val := points[i+1]
			met.Points = append(met.Points, datadogV2.MetricPoint{Timestamp: datadog.PtrInt64(ts), Value: datadog.PtrFloat64(val)})
		}
		return met
	}

	dp := func(a int64, b float64) datadogV2.MetricPoint {
		return datadogV2.MetricPoint{
			Timestamp: datadog.PtrInt64(a),
			Value:     datadog.PtrFloat64(b),
		}
	}

	require.EqualValues(t, PrepareSystemMetrics([]datadogV2.MetricSeries{
		m("system.metric.1", nil, 0.1, 0.2),
		m("system.metric.2", nil, 0.3, 0.4),
		m("process.metric.1", nil, 0.5, 0.6),
		m("process.metric.2", nil, 0.7, 0.8),
		m("system.cpu.load_average.1m", nil, 1, 2),
		m("system.cpu.load_average.5m", nil, 3, 4),
		m("system.cpu.load_average.15m", nil, 5, 6),
		m("system.cpu.utilization", []string{"state:idle"}, 0.15, 0.17),
		m("system.cpu.utilization", []string{"state:user"}, 0.18, 0.19),
		m("system.cpu.utilization", []string{"state:system"}, 0.20, 0.21),
		m("system.cpu.utilization", []string{"state:wait"}, 0.22, 0.23),
		m("system.cpu.utilization", []string{"state:steal"}, 0.24, 0.25),
		m("system.cpu.utilization", []string{"state:other"}, 0.26, 0.27),
		m("system.memory.usage", nil, 1, 0.30),
		m("system.memory.usage", []string{"state:other"}, 1, 1.35),
		m("system.memory.usage", []string{"state:free"}, 1, 1.30),
		m("system.memory.usage", []string{"state:cached"}, 1, 1.35),
		m("system.memory.usage", []string{"state:buffered"}, 1, 1.37, 2, 2.22),
		m("system.network.io", []string{"direction:receive"}, 1, 2.37, 2, 3.22),
		m("system.network.io", []string{"direction:transmit"}, 1, 4.37, 2, 5.22),
		m("system.paging.usage", []string{"state:free"}, 1, 4.37, 2, 5.22),
		m("system.paging.usage", []string{"state:used"}, 1, 4.3, 2, 8.22),
		m("system.filesystem.utilization", nil, 1, 4.3, 2, 5.5, 3, 12.1),
	}), []datadogV2.MetricSeries{
		{
			Metric: "otel.system.metric.1",
			Points: []datadogV2.MetricPoint{dp(0, 0.2)},
		},
		{
			Metric: "otel.system.metric.2",
			Points: []datadogV2.MetricPoint{dp(0, 0.4)},
		},
		{
			Metric: "otel.process.metric.1",
			Points: []datadogV2.MetricPoint{dp(0, 0.6)},
		},
		{
			Metric: "otel.process.metric.2",
			Points: []datadogV2.MetricPoint{dp(0, 0.8)},
		},
		{
			Metric: "otel.system.cpu.load_average.1m",
			Points: []datadogV2.MetricPoint{dp(1, 2)},
		},
		{
			Metric: "otel.system.cpu.load_average.5m",
			Points: []datadogV2.MetricPoint{dp(3, 4)},
		},
		{
			Metric: "otel.system.cpu.load_average.15m",
			Points: []datadogV2.MetricPoint{dp(5, 6)},
		},
		{
			Metric: "otel.system.cpu.utilization",
			Points: []datadogV2.MetricPoint{dp(0, 0.17)},
			Tags:   []string{"state:idle"},
		},
		{
			Metric: "otel.system.cpu.utilization",
			Points: []datadogV2.MetricPoint{dp(0, 0.19)},
			Tags:   []string{"state:user"},
		},
		{
			Metric: "otel.system.cpu.utilization",
			Points: []datadogV2.MetricPoint{dp(0, 0.21)},
			Tags:   []string{"state:system"},
		},
		{
			Metric: "otel.system.cpu.utilization",
			Points: []datadogV2.MetricPoint{dp(0, 0.23)},
			Tags:   []string{"state:wait"}},
		{
			Metric: "otel.system.cpu.utilization",
			Points: []datadogV2.MetricPoint{dp(0, 0.25)},
			Tags:   []string{"state:steal"},
		},
		{
			Metric: "otel.system.cpu.utilization",
			Points: []datadogV2.MetricPoint{dp(0, 0.27)},
			Tags:   []string{"state:other"},
		},
		{
			Metric: "otel.system.memory.usage",
			Points: []datadogV2.MetricPoint{dp(1, 0.3)},
		},
		{
			Metric: "otel.system.memory.usage",
			Points: []datadogV2.MetricPoint{dp(1, 1.35)},
			Tags:   []string{"state:other"},
		},
		{
			Metric: "otel.system.memory.usage",
			Points: []datadogV2.MetricPoint{dp(1, 1.3)},
			Tags:   []string{"state:free"},
		},
		{
			Metric: "otel.system.memory.usage",
			Points: []datadogV2.MetricPoint{dp(1, 1.35)},
			Tags:   []string{"state:cached"},
		},
		{
			Metric: "otel.system.memory.usage",
			Points: []datadogV2.MetricPoint{dp(1, 1.37), dp(2, 2.22)},
			Tags:   []string{"state:buffered"},
		},
		{
			Metric: "otel.system.network.io",
			Points: []datadogV2.MetricPoint{dp(1, 2.37), dp(2, 3.22)},
			Tags:   []string{"direction:receive"},
		},
		{
			Metric: "otel.system.network.io",
			Points: []datadogV2.MetricPoint{dp(1, 4.37), dp(2, 5.22)},
			Tags:   []string{"direction:transmit"},
		},
		{
			Metric: "otel.system.paging.usage",
			Points: []datadogV2.MetricPoint{dp(1, 4.37), dp(2, 5.22)},
			Tags:   []string{"state:free"},
		},
		{
			Metric: "otel.system.paging.usage",
			Points: []datadogV2.MetricPoint{dp(1, 4.3), dp(2, 8.22)},
			Tags:   []string{"state:used"},
		},
		{
			Metric: "otel.system.filesystem.utilization",
			Points: []datadogV2.MetricPoint{dp(1, 4.3), dp(2, 5.5), dp(3, 12.1)},
		},
		{
			Metric: "system.load.1",
			Points: []datadogV2.MetricPoint{dp(1, 2)},
			Type:   &gauge,
		},
		{
			Metric: "system.load.5",
			Points: []datadogV2.MetricPoint{dp(3, 4)},
			Type:   &gauge,
		},
		{
			Metric: "system.load.15",
			Points: []datadogV2.MetricPoint{dp(5, 6)},
			Type:   &gauge,
		},
		{
			Metric: "system.cpu.idle",
			Points: []datadogV2.MetricPoint{dp(0, 17)},
			Type:   &gauge,
			Tags:   []string{"state:idle"},
		},
		{
			Metric: "system.cpu.user",
			Points: []datadogV2.MetricPoint{dp(0, 19)},
			Type:   &gauge,
			Tags:   []string{"state:user"},
		},
		{
			Metric: "system.cpu.system",
			Points: []datadogV2.MetricPoint{dp(0, 21)},
			Type:   &gauge,
			Tags:   []string{"state:system"},
		},
		{
			Metric: "system.cpu.iowait",
			Points: []datadogV2.MetricPoint{dp(0, 23)},
			Type:   &gauge,
			Tags:   []string{"state:wait"},
		},
		{
			Metric: "system.cpu.stolen",
			Points: []datadogV2.MetricPoint{dp(0, 25)},
			Type:   &gauge,
			Tags:   []string{"state:steal"},
		},
		{
			Metric: "system.mem.total",
			Points: []datadogV2.MetricPoint{dp(1, 2.86102294921875e-07)},
			Type:   &gauge,
		},
		{
			Metric: "system.mem.total",
			Points: []datadogV2.MetricPoint{dp(1, 1.2874603271484376e-06)},
			Type:   &gauge,
			Tags:   []string{"state:other"},
		},
		{
			Metric: "system.mem.total",
			Points: []datadogV2.MetricPoint{dp(1, 1.239776611328125e-06)},
			Type:   &gauge,
			Tags:   []string{"state:free"},
		},
		{
			Metric: "system.mem.usable",
			Points: []datadogV2.MetricPoint{dp(1, 1.239776611328125e-06)},
			Type:   &gauge,
			Tags:   []string{"state:free"},
		},
		{
			Metric: "system.mem.total",
			Points: []datadogV2.MetricPoint{dp(1, 1.2874603271484376e-06)},
			Type:   &gauge,
			Tags:   []string{"state:cached"},
		},
		{
			Metric: "system.mem.usable",
			Points: []datadogV2.MetricPoint{dp(1, 1.2874603271484376e-06)},
			Type:   &gauge,
			Tags:   []string{"state:cached"},
		},
		{
			Metric: "system.mem.total",
			Points: []datadogV2.MetricPoint{dp(1, 1.3065338134765626e-06), dp(2, 2.117156982421875e-06)},
			Type:   &gauge,
			Tags:   []string{"state:buffered"},
		},
		{
			Metric: "system.mem.usable",
			Points: []datadogV2.MetricPoint{dp(1, 1.3065338134765626e-06), dp(2, 2.117156982421875e-06)},
			Type:   &gauge,
			Tags:   []string{"state:buffered"},
		},
		{
			Metric: "system.net.bytes_rcvd",
			Points: []datadogV2.MetricPoint{dp(1, 2.37), dp(2, 3.22)},
			Type:   &gauge,
			Tags:   []string{"direction:receive"},
		},
		{
			Metric: "system.net.bytes_sent",
			Points: []datadogV2.MetricPoint{dp(1, 4.37), dp(2, 5.22)},
			Type:   &gauge,
			Tags:   []string{"direction:transmit"},
		},
		{
			Metric: "system.swap.free",
			Points: []datadogV2.MetricPoint{dp(1, 4.37/1024/1024), dp(2, 5.22/1024/1024)},
			Type:   &gauge,
			Tags:   []string{"state:free"},
		},
		{
			Metric: "system.swap.used",
			Points: []datadogV2.MetricPoint{dp(1, 4.3/1024/1024), dp(2, 8.22/1024/1024)},
			Type:   &gauge,
			Tags:   []string{"state:used"},
		},
		{
			Metric: "system.disk.in_use",
			Points: []datadogV2.MetricPoint{dp(1, 4.3), dp(2, 5.5), dp(3, 12.1)},
			Type:   &gauge,
		},
	})
}

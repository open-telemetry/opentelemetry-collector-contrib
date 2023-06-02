// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	zorkian "gopkg.in/zorkian/go-datadog-api.v2"
)

func TestCopyZorkianMetric(t *testing.T) {
	sptr := func(s string) *string { return &s }
	dptr := func(d int) *int { return &d }
	dp := func(a, b float64) zorkian.DataPoint { return zorkian.DataPoint{&a, &b} }

	t.Run("renaming", func(t *testing.T) {
		require.EqualValues(t, copyZorkianSystemMetric(zorkian.Metric{
			Metric:   sptr("oldname"),
			Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			Type:     sptr("oldtype"),
			Host:     sptr("oldhost"),
			Tags:     []string{"x", "y", "z"},
			Unit:     sptr("oldunit"),
			Interval: dptr(3),
		}, "newname", 1), zorkian.Metric{
			Metric:   sptr("newname"),
			Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			Type:     sptr("gauge"),
			Host:     sptr("oldhost"),
			Tags:     []string{"x", "y", "z"},
			Unit:     sptr("oldunit"),
			Interval: dptr(1),
		})

		require.EqualValues(t, copyZorkianSystemMetric(zorkian.Metric{
			Metric:   sptr("oldname"),
			Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			Type:     sptr("oldtype"),
			Host:     sptr("oldhost"),
			Tags:     []string{"x", "y", "z"},
			Unit:     sptr("oldunit"),
			Interval: dptr(3),
		}, "", 1), zorkian.Metric{
			Metric:   sptr(""),
			Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			Type:     sptr("gauge"),
			Host:     sptr("oldhost"),
			Tags:     []string{"x", "y", "z"},
			Unit:     sptr("oldunit"),
			Interval: dptr(1),
		})

		require.EqualValues(t, copyZorkianSystemMetric(zorkian.Metric{
			Metric:   nil,
			Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			Type:     sptr("oldtype"),
			Host:     sptr("oldhost"),
			Tags:     []string{"x", "y", "z"},
			Unit:     sptr("oldunit"),
			Interval: dptr(3),
		}, "", 1), zorkian.Metric{
			Metric:   sptr(""),
			Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			Type:     sptr("gauge"),
			Host:     sptr("oldhost"),
			Tags:     []string{"x", "y", "z"},
			Unit:     sptr("oldunit"),
			Interval: dptr(1),
		})
	})

	t.Run("interval", func(t *testing.T) {
		require.EqualValues(t, copyZorkianSystemMetric(zorkian.Metric{
			Metric:   nil,
			Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			Type:     sptr("oldtype"),
			Host:     sptr("oldhost"),
			Tags:     []string{"x", "y", "z"},
			Unit:     sptr("oldunit"),
			Interval: dptr(3),
		}, "", 1), zorkian.Metric{
			Metric:   sptr(""),
			Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			Type:     sptr("gauge"),
			Host:     sptr("oldhost"),
			Tags:     []string{"x", "y", "z"},
			Unit:     sptr("oldunit"),
			Interval: dptr(1),
		})
	})

	t.Run("division", func(t *testing.T) {
		for _, tt := range []struct {
			in, out []zorkian.DataPoint
			div     float64
		}{
			{
				in:  []zorkian.DataPoint{dp(0, 0), dp(1, 20)},
				div: 0,
				out: []zorkian.DataPoint{dp(0, 0), dp(1, 20)},
			},
			{
				in:  []zorkian.DataPoint{dp(0, 0), dp(1, 20)},
				div: 1,
				out: []zorkian.DataPoint{dp(0, 0), dp(1, 20)},
			},
			{
				in:  []zorkian.DataPoint{dp(1.1, 0), {}},
				div: 2,
				out: []zorkian.DataPoint{dp(1.1, 0), {}},
			},
			{
				in:  []zorkian.DataPoint{dp(0, 0), dp(1, 20)},
				div: 10,
				out: []zorkian.DataPoint{dp(0, 0), dp(1, 2)},
			},
			{
				in:  []zorkian.DataPoint{dp(1.1, 0), dp(55.5, math.MaxFloat64)},
				div: 1024 * 1024.5,
				out: []zorkian.DataPoint{dp(1.1, 0), dp(55.5, 1.713577063947272e+302)},
			},
			{
				in:  []zorkian.DataPoint{dp(1.1, 0), dp(55.5, 20)},
				div: math.MaxFloat64,
				out: []zorkian.DataPoint{dp(1.1, 0), dp(55.5, 1.1125369292536009e-307)},
			},
		} {
			t.Run(fmt.Sprintf("%.0f", tt.div), func(t *testing.T) {
				require.EqualValues(t,
					copyZorkianSystemMetric(zorkian.Metric{Points: tt.in}, "", tt.div),
					zorkian.Metric{Metric: sptr(""), Type: sptr("gauge"), Points: tt.out, Interval: dptr(1)},
				)
			})
		}
	})
}

func TestExtractZorkianSystemMetrics(t *testing.T) {
	sptr := func(s string) *string { return &s }
	dptr := func(d int) *int { return &d }
	dp := func(a, b float64) zorkian.DataPoint { return zorkian.DataPoint{&a, &b} }

	for _, tt := range []struct {
		in  zorkian.Metric
		out []zorkian.Metric
	}{
		{
			in: zorkian.Metric{
				Metric: sptr("system.cpu.load_average.1m"),
				Points: []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.load.1"),
				Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.cpu.load_average.5m"),
				Points: []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.load.5"),
				Points:   []zorkian.DataPoint{dp(1, 2), dp(3, 4)},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.cpu.load_average.15m"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.load.15"),
				Points:   []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.cpu.utilization"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:idle"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.cpu.idle"),
				Points:   []zorkian.DataPoint{dp(2, 200), dp(3, 400)},
				Interval: dptr(1),
				Tags:     []string{"state:idle"},
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.cpu.utilization"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:user"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.cpu.user"),
				Points:   []zorkian.DataPoint{dp(2, 200), dp(3, 400)},
				Interval: dptr(1),
				Tags:     []string{"state:user"},
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.cpu.utilization"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:system"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.cpu.system"),
				Points:   []zorkian.DataPoint{dp(2, 200), dp(3, 400)},
				Interval: dptr(1),
				Tags:     []string{"state:system"},
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.cpu.utilization"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:wait"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.cpu.iowait"),
				Points:   []zorkian.DataPoint{dp(2, 200), dp(3, 400)},
				Interval: dptr(1),
				Tags:     []string{"state:wait"},
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.cpu.utilization"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:steal"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.cpu.stolen"),
				Points:   []zorkian.DataPoint{dp(2, 200), dp(3, 400)},
				Tags:     []string{"state:steal"},
				Type:     sptr("gauge"),
				Interval: dptr(1),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.memory.usage"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:other"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.mem.total"),
				Points:   []zorkian.DataPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:     []string{"state:other"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.memory.usage"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:free"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.mem.total"),
				Points:   []zorkian.DataPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:     []string{"state:free"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}, {
				Metric:   sptr("system.mem.usable"),
				Points:   []zorkian.DataPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:     []string{"state:free"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.memory.usage"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:cached"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.mem.total"),
				Points:   []zorkian.DataPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:     []string{"state:cached"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}, {
				Metric:   sptr("system.mem.usable"),
				Points:   []zorkian.DataPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:     []string{"state:cached"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.memory.usage"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:buffered"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.mem.total"),
				Points:   []zorkian.DataPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:     []string{"state:buffered"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}, {
				Metric:   sptr("system.mem.usable"),
				Points:   []zorkian.DataPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:     []string{"state:buffered"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.network.io"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"direction:receive"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.net.bytes_rcvd"),
				Points:   []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:     []string{"direction:receive"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.network.io"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"direction:transmit"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.net.bytes_sent"),
				Points:   []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:     []string{"direction:transmit"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.paging.usage"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:free"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.swap.free"),
				Points:   []zorkian.DataPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:     []string{"state:free"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.paging.usage"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Tags:   []string{"state:used"},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.swap.used"),
				Points:   []zorkian.DataPoint{dp(2, 1.9073486328125e-06), dp(3, 3.814697265625e-06)},
				Tags:     []string{"state:used"},
				Interval: dptr(1),
				Type:     sptr("gauge"),
			}},
		},
		{
			in: zorkian.Metric{
				Metric: sptr("system.filesystem.utilization"),
				Points: []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
			},
			out: []zorkian.Metric{{
				Metric:   sptr("system.disk.in_use"),
				Points:   []zorkian.DataPoint{dp(2, 2), dp(3, 4)},
				Type:     sptr("gauge"),
				Interval: dptr(1),
			}},
		},
	} {
		t.Run("", func(t *testing.T) {
			out := extractZorkianSystemMetric(tt.in)
			require.EqualValues(t, tt.out, out, fmt.Sprintf("%s[%#v]", *tt.in.Metric, tt.in.Tags))
		})
	}
}

func TestPrepareZorkianSystemMetrics(t *testing.T) {
	m := func(name string, tags []string, points ...float64) zorkian.Metric {
		met := zorkian.Metric{
			Metric: &name,
			Tags:   tags,
		}
		if len(points)%2 != 0 {
			t.Fatal("Number of data point arguments passed to function must be even.")
		}
		met.Points = make([]zorkian.DataPoint, 0, len(points)/2)
		for i := 0; i < len(points); i += 2 {
			met.Points = append(met.Points, zorkian.DataPoint{&points[i], &points[i+1]})
		}
		return met
	}
	fptr := func(d float64) *float64 { return &d }
	dptr := func(d int) *int { return &d }
	sptr := func(s string) *string { return &s }
	require.EqualValues(t, PrepareZorkianSystemMetrics([]zorkian.Metric{
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
	}), []zorkian.Metric{
		{
			Metric: sptr("otel.system.metric.1"),
			Points: []zorkian.DataPoint{{fptr(0.1), fptr(0.2)}},
		},
		{
			Metric: sptr("otel.system.metric.2"),
			Points: []zorkian.DataPoint{{fptr(0.3), fptr(0.4)}},
		},
		{
			Metric: sptr("otel.process.metric.1"),
			Points: []zorkian.DataPoint{{fptr(0.5), fptr(0.6)}},
		},
		{
			Metric: sptr("otel.process.metric.2"),
			Points: []zorkian.DataPoint{{fptr(0.7), fptr(0.8)}},
		},
		{
			Metric: sptr("otel.system.cpu.load_average.1m"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(2)}},
		},
		{
			Metric: sptr("otel.system.cpu.load_average.5m"),
			Points: []zorkian.DataPoint{{fptr(3), fptr(4)}},
		},
		{
			Metric: sptr("otel.system.cpu.load_average.15m"),
			Points: []zorkian.DataPoint{{fptr(5), fptr(6)}},
		},
		{
			Metric: sptr("otel.system.cpu.utilization"),
			Points: []zorkian.DataPoint{{fptr(0.15), fptr(0.17)}},
			Tags:   []string{"state:idle"},
		},
		{
			Metric: sptr("otel.system.cpu.utilization"),
			Points: []zorkian.DataPoint{{fptr(0.18), fptr(0.19)}},
			Tags:   []string{"state:user"},
		},
		{
			Metric: sptr("otel.system.cpu.utilization"),
			Points: []zorkian.DataPoint{{fptr(0.2), fptr(0.21)}},
			Tags:   []string{"state:system"},
		},
		{
			Metric: sptr("otel.system.cpu.utilization"),
			Points: []zorkian.DataPoint{{fptr(0.22), fptr(0.23)}},
			Tags:   []string{"state:wait"}},
		{
			Metric: sptr("otel.system.cpu.utilization"),
			Points: []zorkian.DataPoint{{fptr(0.24), fptr(0.25)}},
			Tags:   []string{"state:steal"},
		},
		{
			Metric: sptr("otel.system.cpu.utilization"),
			Points: []zorkian.DataPoint{{fptr(0.26), fptr(0.27)}},
			Tags:   []string{"state:other"}},
		{
			Metric: sptr("otel.system.memory.usage"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(0.3)}},
		},
		{
			Metric: sptr("otel.system.memory.usage"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(1.35)}},
			Tags:   []string{"state:other"},
		},
		{
			Metric: sptr("otel.system.memory.usage"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(1.3)}},
			Tags:   []string{"state:free"},
		},
		{
			Metric: sptr("otel.system.memory.usage"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(1.35)}},
			Tags:   []string{"state:cached"},
		},
		{
			Metric: sptr("otel.system.memory.usage"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(1.37)}, {fptr(2), fptr(2.22)}},
			Tags:   []string{"state:buffered"},
		},
		{
			Metric: sptr("otel.system.network.io"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(2.37)}, {fptr(2), fptr(3.22)}},
			Tags:   []string{"direction:receive"},
		},
		{
			Metric: sptr("otel.system.network.io"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(4.37)}, {fptr(2), fptr(5.22)}},
			Tags:   []string{"direction:transmit"},
		},
		{
			Metric: sptr("otel.system.paging.usage"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(4.37)}, {fptr(2), fptr(5.22)}},
			Tags:   []string{"state:free"},
		},
		{
			Metric: sptr("otel.system.paging.usage"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(4.3)}, {fptr(2), fptr(8.22)}},
			Tags:   []string{"state:used"},
		},
		{
			Metric: sptr("otel.system.filesystem.utilization"),
			Points: []zorkian.DataPoint{{fptr(1), fptr(4.3)}, {fptr(2), fptr(5.5)}, {fptr(3), fptr(12.1)}},
		},
		{
			Metric:   sptr("system.load.1"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(2)}},
			Type:     sptr("gauge"),
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.load.5"),
			Points:   []zorkian.DataPoint{{fptr(3), fptr(4)}},
			Type:     sptr("gauge"),
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.load.15"),
			Points:   []zorkian.DataPoint{{fptr(5), fptr(6)}},
			Type:     sptr("gauge"),
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.cpu.idle"),
			Points:   []zorkian.DataPoint{{fptr(0.15), fptr(17)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:idle"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.cpu.user"),
			Points:   []zorkian.DataPoint{{fptr(0.18), fptr(19)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:user"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.cpu.system"),
			Points:   []zorkian.DataPoint{{fptr(0.2), fptr(21)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:system"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.cpu.iowait"),
			Points:   []zorkian.DataPoint{{fptr(0.22), fptr(23)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:wait"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.cpu.stolen"),
			Points:   []zorkian.DataPoint{{fptr(0.24), fptr(25)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:steal"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.mem.total"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(2.86102294921875e-07)}},
			Type:     sptr("gauge"),
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.mem.total"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(1.2874603271484376e-06)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:other"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.mem.total"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(1.239776611328125e-06)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:free"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.mem.usable"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(1.239776611328125e-06)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:free"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.mem.total"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(1.2874603271484376e-06)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:cached"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.mem.usable"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(1.2874603271484376e-06)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:cached"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.mem.total"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(1.3065338134765626e-06)}, {fptr(2), fptr(2.117156982421875e-06)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:buffered"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.mem.usable"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(1.3065338134765626e-06)}, {fptr(2), fptr(2.117156982421875e-06)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:buffered"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.net.bytes_rcvd"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(2.37)}, {fptr(2), fptr(3.22)}},
			Type:     sptr("gauge"),
			Tags:     []string{"direction:receive"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.net.bytes_sent"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(4.37)}, {fptr(2), fptr(5.22)}},
			Type:     sptr("gauge"),
			Tags:     []string{"direction:transmit"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.swap.free"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(4.37 / 1024 / 1024)}, {fptr(2), fptr(5.22 / 1024 / 1024)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:free"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.swap.used"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(4.3 / 1024 / 1024)}, {fptr(2), fptr(8.22 / 1024 / 1024)}},
			Type:     sptr("gauge"),
			Tags:     []string{"state:used"},
			Interval: dptr(1),
		},
		{
			Metric:   sptr("system.disk.in_use"),
			Points:   []zorkian.DataPoint{{fptr(1), fptr(4.3)}, {fptr(2), fptr(5.5)}, {fptr(3), fptr(12.1)}},
			Type:     sptr("gauge"),
			Interval: dptr(1),
		},
	})
}

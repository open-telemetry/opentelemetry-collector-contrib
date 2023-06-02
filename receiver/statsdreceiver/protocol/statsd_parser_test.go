// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/lightstep/go-expohisto/mapping/logarithm"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/metricstestutil"
)

func Test_ParseMessageToMetric(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMetric statsDMetric
		err        error
	}{
		{
			name:  "empty input string",
			input: "",
			err:   errors.New("invalid message format: "),
		},
		{
			name:  "missing metric value",
			input: "test.metric|c",
			err:   errors.New("invalid <name>:<value> format: test.metric"),
		},
		{
			name:  "empty metric name",
			input: ":42|c",
			err:   errors.New("empty metric name"),
		},
		{
			name:  "empty metric value",
			input: "test.metric:|c",
			err:   errors.New("empty metric value"),
		},
		{
			name:  "invalid sample rate value",
			input: "test.metric:42|c|@1.0a",
			err:   errors.New("parse sample rate: 1.0a"),
		},
		{
			name:  "invalid tag format",
			input: "test.metric:42|c|#key1",
			err:   errors.New("invalid tag format: [key1]"),
		},
		{
			name:  "unrecognized message part",
			input: "test.metric:42|c|$extra",
			err:   errors.New("unrecognized message part: $extra"),
		},
		{
			name:  "integer counter",
			input: "test.metric:42|c",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"c", 0, nil, nil),
		},
		{
			name:  "invalid  counter metric value",
			input: "test.metric:42.abc|c",
			err:   errors.New("parse metric value string: 42.abc"),
		},
		{
			name:  "unhandled metric type",
			input: "test.metric:42|unhandled_type",
			err:   errors.New("unsupported metric type: unhandled_type"),
		},
		{
			name:  "counter metric with sample rate and tag",
			input: "test.metric:42|c|@0.1|#key:value",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"c",
				0.1,
				[]string{"key"},
				[]string{"value"}),
		},
		{
			name:  "counter metric with sample rate(not divisible) and tag",
			input: "test.metric:42|c|@0.8|#key:value",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"c",
				0.8,
				[]string{"key"},
				[]string{"value"}),
		},
		{
			name:  "counter metric with sample rate(not divisible) and two tags",
			input: "test.metric:42|c|@0.8|#key:value,key2:value2",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"c",
				0.8,
				[]string{"key", "key2"},
				[]string{"value", "value2"}),
		},
		{
			name:  "double gauge",
			input: "test.metric:42.0|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"g", 0, nil, nil),
		},
		{
			name:  "int gauge",
			input: "test.metric:42|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"g", 0, nil, nil),
		},
		{
			name:  "invalid gauge metric value",
			input: "test.metric:42.abc|g",
			err:   errors.New("parse metric value string: 42.abc"),
		},
		{
			name:  "gauge metric with sample rate and tag",
			input: "test.metric:11|g|@0.1|#key:value",
			wantMetric: testStatsDMetric(
				"test.metric",
				11,
				false,
				"g",
				0.1,
				[]string{"key"},
				[]string{"value"}),
		},
		{
			name:  "gauge metric with sample rate and two tags",
			input: "test.metric:11|g|@0.8|#key:value,key2:value2",
			wantMetric: testStatsDMetric(
				"test.metric",
				11,
				false,
				"g",
				0.8,
				[]string{"key", "key2"},
				[]string{"value", "value2"}),
		},
		{
			name:  "double gauge plus",
			input: "test.metric:+42.0|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				true,
				"g", 0, nil, nil),
		},
		{
			name:  "double gauge minus",
			input: "test.metric:-42.0|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				-42,
				true,
				"g", 0, nil, nil),
		},
		{
			name:  "int gauge plus",
			input: "test.metric:+42|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				true,
				"g", 0, nil, nil),
		},
		{
			name:  "int gauge minus",
			input: "test.metric:-42|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				-42,
				true,
				"g", 0, nil, nil),
		},
		{
			name:  "invalid histogram metric value",
			input: "test.metric:42.abc|h",
			err:   errors.New("parse metric value string: 42.abc"),
		},
		{
			name:  "int timer",
			input: "test.metric:-42|ms",
			wantMetric: testStatsDMetric(
				"test.metric",
				-42,
				true,
				"ms", 0, nil, nil),
		},
		{
			name:  "int histogram",
			input: "test.metric:42|h",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"h", 0, nil, nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMessageToMetric(tt.input, false)

			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantMetric, got)
			}
		})
	}
}

func Test_ParseMessageToMetricWithMetricType(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMetric statsDMetric
		err        error
	}{
		{
			name:  "integer counter",
			input: "test.metric:42|c",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"c", 0,
				[]string{"metric_type"},
				[]string{"counter"}),
		},
		{
			name:  "counter metric with sample rate and tag",
			input: "test.metric:42|c|@0.1|#key:value",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"c",
				0.1,
				[]string{"key", "metric_type"},
				[]string{"value", "counter"}),
		},
		{
			name:  "counter metric with sample rate(not divisible) and tag",
			input: "test.metric:42|c|@0.8|#key:value",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"c",
				0.8,
				[]string{"key", "metric_type"},
				[]string{"value", "counter"}),
		},
		{
			name:  "counter metric with sample rate(not divisible) and two tags",
			input: "test.metric:42|c|@0.8|#key:value,key2:value2",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"c",
				0.8,
				[]string{"key", "key2", "metric_type"},
				[]string{"value", "value2", "counter"}),
		},
		{
			name:  "double gauge",
			input: "test.metric:42.0|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"g", 0,
				[]string{"metric_type"},
				[]string{"gauge"}),
		},
		{
			name:  "int gauge",
			input: "test.metric:42|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"g", 0,
				[]string{"metric_type"},
				[]string{"gauge"}),
		},
		{
			name:  "invalid gauge metric value",
			input: "test.metric:42.abc|g",
			err:   errors.New("parse metric value string: 42.abc"),
		},
		{
			name:  "gauge metric with sample rate and tag",
			input: "test.metric:11|g|@0.1|#key:value",
			wantMetric: testStatsDMetric(
				"test.metric",
				11,
				false,
				"g",
				0.1,
				[]string{"key", "metric_type"},
				[]string{"value", "gauge"}),
		},
		{
			name:  "gauge metric with sample rate and two tags",
			input: "test.metric:11|g|@0.8|#key:value,key2:value2",
			wantMetric: testStatsDMetric(
				"test.metric",
				11,
				false,
				"g",
				0.8,
				[]string{"key", "key2", "metric_type"},
				[]string{"value", "value2", "gauge"}),
		},
		{
			name:  "double gauge plus",
			input: "test.metric:+42.0|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				true,
				"g", 0,
				[]string{"metric_type"},
				[]string{"gauge"}),
		},
		{
			name:  "double gauge minus",
			input: "test.metric:-42.0|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				-42,
				true,
				"g", 0,
				[]string{"metric_type"},
				[]string{"gauge"}),
		},
		{
			name:  "int gauge plus",
			input: "test.metric:+42|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				true,
				"g", 0,
				[]string{"metric_type"},
				[]string{"gauge"}),
		},
		{
			name:  "int gauge minus",
			input: "test.metric:-42|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				-42,
				true,
				"g", 0,
				[]string{"metric_type"},
				[]string{"gauge"}),
		},
		{
			name:  "int timer",
			input: "test.metric:-42|ms",
			wantMetric: testStatsDMetric(
				"test.metric",
				-42,
				true,
				"ms", 0,
				[]string{"metric_type"},
				[]string{"timing"}),
		},
		{
			name:  "int histogram",
			input: "test.metric:42|h",
			wantMetric: testStatsDMetric(
				"test.metric",
				42,
				false,
				"h", 0,
				[]string{"metric_type"},
				[]string{"histogram"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMessageToMetric(tt.input, true)

			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantMetric, got)
			}
		})
	}
}

func testStatsDMetric(
	name string, asFloat float64,
	addition bool, metricType MetricType,
	sampleRate float64, labelKeys []string,
	labelValue []string,
) statsDMetric {
	if len(labelKeys) > 0 {
		var kvs []attribute.KeyValue
		var sortable attribute.Sortable
		for n, k := range labelKeys {
			kvs = append(kvs, attribute.String(k, labelValue[n]))
		}
		return statsDMetric{
			description: statsDMetricDescription{
				name:       name,
				metricType: metricType,
				attrs:      attribute.NewSetWithSortable(kvs, &sortable),
			},
			asFloat:    asFloat,
			addition:   addition,
			unit:       "",
			sampleRate: sampleRate,
		}
	}
	return statsDMetric{
		description: statsDMetricDescription{
			name:       name,
			metricType: metricType,
		},
		asFloat:    asFloat,
		addition:   addition,
		unit:       "",
		sampleRate: sampleRate,
	}
}

func testDescription(name string, metricType MetricType, keys []string, values []string) statsDMetricDescription {
	var kvs []attribute.KeyValue
	var sortable attribute.Sortable
	for n, k := range keys {
		kvs = append(kvs, attribute.String(k, values[n]))
	}
	return statsDMetricDescription{
		name:       name,
		metricType: metricType,
		attrs:      attribute.NewSetWithSortable(kvs, &sortable),
	}
}

func TestStatsDParser_Aggregate(t *testing.T) {
	timeNowFunc = func() time.Time {
		return time.Unix(711, 0)
	}

	tests := []struct {
		name             string
		input            []string
		expectedGauges   map[statsDMetricDescription]pmetric.ScopeMetrics
		expectedCounters map[statsDMetricDescription]pmetric.ScopeMetrics
		expectedTimer    []pmetric.ScopeMetrics
		err              error
	}{
		{
			name: "parsedMetric error: empty metric value",
			input: []string{
				"test.metric:|c",
			},
			err: errors.New("empty metric value"),
		},
		{
			name: "parsedMetric error: empty metric name",
			input: []string{
				":42|c",
			},
			err: errors.New("empty metric name"),
		},
		{
			name: "gauge plus",
			input: []string{
				"statsdTestMetric1:1|g|#mykey:myvalue",
				"statsdTestMetric2:2|g|#mykey:myvalue",
				"statsdTestMetric1:+1|g|#mykey:myvalue",
				"statsdTestMetric1:+100|g|#mykey:myvalue",
				"statsdTestMetric1:+10000|g|#mykey:myvalue",
				"statsdTestMetric2:+5|g|#mykey:myvalue",
				"statsdTestMetric2:+500|g|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 10102, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", 507, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{},
		},
		{
			name: "gauge minus",
			input: []string{
				"statsdTestMetric1:5000|g|#mykey:myvalue",
				"statsdTestMetric2:10|g|#mykey:myvalue",
				"statsdTestMetric1:-1|g|#mykey:myvalue",
				"statsdTestMetric2:-5|g|#mykey:myvalue",
				"statsdTestMetric1:-1|g|#mykey:myvalue",
				"statsdTestMetric1:-1|g|#mykey:myvalue",
				"statsdTestMetric1:-10|g|#mykey:myvalue",
				"statsdTestMetric1:-1|g|#mykey:myvalue",
				"statsdTestMetric1:-100|g|#mykey:myvalue",
				"statsdTestMetric1:-1|g|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 4885, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", 5, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{},
		},
		{
			name: "gauge plus and minus",
			input: []string{
				"statsdTestMetric1:5000|g|#mykey:myvalue",
				"statsdTestMetric1:4000|g|#mykey:myvalue",
				"statsdTestMetric1:+500|g|#mykey:myvalue",
				"statsdTestMetric1:-400|g|#mykey:myvalue",
				"statsdTestMetric1:+2|g|#mykey:myvalue",
				"statsdTestMetric1:-1|g|#mykey:myvalue",
				"statsdTestMetric2:365|g|#mykey:myvalue",
				"statsdTestMetric2:+300|g|#mykey:myvalue",
				"statsdTestMetric2:-200|g|#mykey:myvalue",
				"statsdTestMetric2:200|g|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 4101, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", 200, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{},
		},
		{
			name: "counter with increment and sample rate",
			input: []string{
				"statsdTestMetric1:3000|c|#mykey:myvalue",
				"statsdTestMetric1:4000|c|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricDescription]pmetric.ScopeMetrics{},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", 7000, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", 50, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false),
			},
		},
		{
			name: "counter and gauge: one gauge and two counters",
			input: []string{
				"statsdTestMetric1:3000|c|#mykey:myvalue",
				"statsdTestMetric1:500|g|#mykey:myvalue",
				"statsdTestMetric1:400|g|#mykey:myvalue",
				"statsdTestMetric1:+20|g|#mykey:myvalue",
				"statsdTestMetric1:4000|c|#mykey:myvalue",
				"statsdTestMetric1:-1|g|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
				"statsdTestMetric1:+2|g|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 421, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", 7000, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", 50, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false),
			},
		},
		{
			name: "counter and gauge: 2 gauges and 2 counters",
			input: []string{
				"statsdTestMetric1:500|g|#mykey:myvalue",
				"statsdTestMetric1:400|g|#mykey:myvalue1",
				"statsdTestMetric1:300|g|#mykey:myvalue",
				"statsdTestMetric1:-1|g|#mykey:myvalue1",
				"statsdTestMetric1:+20|g|#mykey:myvalue",
				"statsdTestMetric1:-1|g|#mykey:myvalue",
				"statsdTestMetric1:20|c|@0.1|#mykey:myvalue",
				"statsdTestMetric2:50|c|#mykey:myvalue",
				"statsdTestMetric1:15|c|#mykey:myvalue",
				"statsdTestMetric2:5|c|@0.2|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 319, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue1"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 399, false, "g", 0, []string{"mykey"}, []string{"myvalue1"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", 215, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", 75, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false),
			},
		},
		{
			name: "counter and gauge: 2 timings and 2 histograms",
			input: []string{
				"statsdTestMetric1:500|ms|#mykey:myvalue",
				"statsdTestMetric1:400|h|#mykey:myvalue",
				"statsdTestMetric1:300|ms|#mykey:myvalue",
				"statsdTestMetric1:10|h|@0.1|#mykey:myvalue",
			},
			expectedGauges:   map[statsDMetricDescription]pmetric.ScopeMetrics{},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{},
			expectedTimer: []pmetric.ScopeMetrics{
				buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 500, false, "ms", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 400, false, "h", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 300, false, "ms", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 10, false, "h", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			p := &StatsDParser{}
			assert.NoError(t, p.Initialize(false, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}}))
			p.lastIntervalTime = time.Unix(611, 0)
			addr, _ := net.ResolveUDPAddr("udp", "1.2.3.4:5678")
			addrKey := newNetAddr(addr)
			for _, line := range tt.input {
				err = p.Aggregate(line, addr)
			}
			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.Equal(t, tt.expectedGauges, p.instrumentsByAddress[addrKey].gauges)
				assert.Equal(t, tt.expectedCounters, p.instrumentsByAddress[addrKey].counters)
				assert.Equal(t, tt.expectedTimer, p.instrumentsByAddress[addrKey].timersAndDistributions)
			}
		})
	}
}

func TestStatsDParser_AggregateByAddress(t *testing.T) {
	tests := []struct {
		name           string
		addresses      []net.Addr
		input          [][]string
		expectedGauges []map[statsDMetricDescription]pmetric.ScopeMetrics
	}{
		{
			name: "two addresses",
			addresses: []net.Addr{
				&net.UDPAddr{IP: []byte{1, 2, 3, 4}, Port: 5678},
				&net.UDPAddr{IP: []byte{255, 254, 253, 252}, Port: 251},
			},
			input: [][]string{
				{
					"statsdTestMetric1:1|g|#mykey:myvalue",
					"statsdTestMetric2:2|g|#mykey:myvalue",
					"statsdTestMetric1:+1|g|#mykey:myvalue",
					"statsdTestMetric1:+100|g|#mykey:myvalue",
					"statsdTestMetric1:+10000|g|#mykey:myvalue",
					"statsdTestMetric2:+5|g|#mykey:myvalue",
					"statsdTestMetric2:+500|g|#mykey:myvalue",
				}, {
					"statsdTestMetric1:1|g|#mykey:myvalue",
					"statsdTestMetric2:2|g|#mykey:myvalue",
					"statsdTestMetric1:+1|g|#mykey:myvalue",
					"statsdTestMetric1:+100|g|#mykey:myvalue",
					"statsdTestMetric1:+10000|g|#mykey:myvalue",
					"statsdTestMetric2:+5|g|#mykey:myvalue",
					"statsdTestMetric2:+500|g|#mykey:myvalue",
				},
			},
			expectedGauges: []map[statsDMetricDescription]pmetric.ScopeMetrics{
				{
					testDescription("statsdTestMetric1", "g",
						[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 10102, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0)),
					testDescription("statsdTestMetric2", "g",
						[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", 507, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0)),
				},
				{
					testDescription("statsdTestMetric1", "g",
						[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 10102, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0)),
					testDescription("statsdTestMetric2", "g",
						[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", 507, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0)),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &StatsDParser{}
			assert.NoError(t, p.Initialize(true, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}}))
			p.lastIntervalTime = time.Unix(611, 0)
			for i, addr := range tt.addresses {
				for _, line := range tt.input[i] {
					assert.NoError(t, p.Aggregate(line, addr))
				}
			}
			for i, addr := range tt.addresses {
				addrKey := newNetAddr(addr)
				assert.Equal(t, tt.expectedGauges[i], p.instrumentsByAddress[addrKey].gauges)
			}
		})
	}
}

func TestStatsDParser_AggregateWithMetricType(t *testing.T) {
	timeNowFunc = func() time.Time {
		return time.Unix(711, 0)
	}

	tests := []struct {
		name             string
		input            []string
		expectedGauges   map[statsDMetricDescription]pmetric.ScopeMetrics
		expectedCounters map[statsDMetricDescription]pmetric.ScopeMetrics
		err              error
	}{
		{
			name: "gauge plus",
			input: []string{
				"statsdTestMetric1:1|g|#mykey:myvalue",
				"statsdTestMetric2:2|g|#mykey:myvalue",
				"statsdTestMetric1:+1|g|#mykey:myvalue",
				"statsdTestMetric1:+100|g|#mykey:myvalue",
				"statsdTestMetric1:+10000|g|#mykey:myvalue",
				"statsdTestMetric2:+5|g|#mykey:myvalue",
				"statsdTestMetric2:+500|g|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 10102, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "g",
					[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", 507, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{},
		},

		{
			name: "counter with increment and sample rate",
			input: []string{
				"statsdTestMetric1:3000|c|#mykey:myvalue",
				"statsdTestMetric1:4000|c|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricDescription]pmetric.ScopeMetrics{},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey", "metric_type"}, []string{"myvalue", "counter"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", 7000, false, "c", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "counter"}), false),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey", "metric_type"}, []string{"myvalue", "counter"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", 50, false, "c", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "counter"}), false),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			p := &StatsDParser{}
			assert.NoError(t, p.Initialize(true, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}}))
			p.lastIntervalTime = time.Unix(611, 0)
			addr, _ := net.ResolveUDPAddr("udp", "1.2.3.4:5678")
			addrKey := newNetAddr(addr)
			for _, line := range tt.input {
				err = p.Aggregate(line, addr)
			}
			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.Equal(t, tt.expectedGauges, p.instrumentsByAddress[addrKey].gauges)
				assert.Equal(t, tt.expectedCounters, p.instrumentsByAddress[addrKey].counters)
			}
		})
	}
}

func TestStatsDParser_AggregateWithIsMonotonicCounter(t *testing.T) {
	timeNowFunc = func() time.Time {
		return time.Unix(711, 0)
	}

	tests := []struct {
		name             string
		input            []string
		expectedGauges   map[statsDMetricDescription]pmetric.ScopeMetrics
		expectedCounters map[statsDMetricDescription]pmetric.ScopeMetrics
		err              error
	}{
		{
			name: "counter with increment and sample rate",
			input: []string{
				"statsdTestMetric1:3000|c|#mykey:myvalue",
				"statsdTestMetric1:4000|c|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricDescription]pmetric.ScopeMetrics{},
			expectedCounters: map[statsDMetricDescription]pmetric.ScopeMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", 7000, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), true),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", 50, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			p := &StatsDParser{}
			assert.NoError(t, p.Initialize(false, true, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}}))
			p.lastIntervalTime = time.Unix(611, 0)
			addr, _ := net.ResolveUDPAddr("udp", "1.2.3.4:5678")
			addrKey := newNetAddr(addr)
			for _, line := range tt.input {
				err = p.Aggregate(line, addr)
			}
			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.Equal(t, tt.expectedGauges, p.instrumentsByAddress[addrKey].gauges)
				assert.Equal(t, tt.expectedCounters, p.instrumentsByAddress[addrKey].counters)
			}
		})
	}
}

func TestStatsDParser_AggregateTimerWithSummary(t *testing.T) {
	timeNowFunc = func() time.Time {
		return time.Unix(711, 0)
	}

	tests := []struct {
		name              string
		input             []string
		expectedSummaries map[statsDMetricDescription]summaryMetric
		err               error
	}{
		{
			name: "timer",
			input: []string{
				"statsdTestMetric1:1|ms|#mykey:myvalue",
				"statsdTestMetric2:2|ms|#mykey:myvalue",
				"statsdTestMetric1:1|ms|#mykey:myvalue",
				"statsdTestMetric1:10|ms|#mykey:myvalue",
				"statsdTestMetric1:20|ms|#mykey:myvalue",
				"statsdTestMetric2:5|ms|#mykey:myvalue",
				"statsdTestMetric2:10|ms|#mykey:myvalue",
				"statsdTestMetric1:20|ms|#mykey:myvalue",
			},
			expectedSummaries: map[statsDMetricDescription]summaryMetric{
				testDescription("statsdTestMetric1", "ms",
					[]string{"mykey"}, []string{"myvalue"}): {
					points:  []float64{1, 1, 10, 20, 20},
					weights: []float64{1, 1, 1, 1, 1},
				},
				testDescription("statsdTestMetric2", "ms",
					[]string{"mykey"}, []string{"myvalue"}): {
					points:  []float64{2, 5, 10},
					weights: []float64{1, 1, 1},
				},
			},
		},
		{
			name: "histogram",
			input: []string{
				"statsdTestMetric1:1|h|#mykey:myvalue",
				"statsdTestMetric2:2|h|#mykey:myvalue",
				"statsdTestMetric1:1|h|#mykey:myvalue",
				"statsdTestMetric1:10|h|#mykey:myvalue",
				"statsdTestMetric1:20|h|#mykey:myvalue",
				"statsdTestMetric2:5|h|#mykey:myvalue",
				"statsdTestMetric2:10|h|#mykey:myvalue",
			},
			expectedSummaries: map[statsDMetricDescription]summaryMetric{
				testDescription("statsdTestMetric1", "h",
					[]string{"mykey"}, []string{"myvalue"}): {
					points:  []float64{1, 1, 10, 20},
					weights: []float64{1, 1, 1, 1},
				},
				testDescription("statsdTestMetric2", "h",
					[]string{"mykey"}, []string{"myvalue"}): {
					points:  []float64{2, 5, 10},
					weights: []float64{1, 1, 1},
				},
			},
		},
		{
			name: "histogram_sampled",
			input: []string{
				"statsdTestMetric1:300|h|@0.1|#mykey:myvalue",
				"statsdTestMetric1:100|h|@0.05|#mykey:myvalue",
				"statsdTestMetric1:300|h|@0.1|#mykey:myvalue",
				"statsdTestMetric1:200|h|@0.01|#mykey:myvalue",
			},
			expectedSummaries: map[statsDMetricDescription]summaryMetric{
				testDescription("statsdTestMetric1", "h",
					[]string{"mykey"}, []string{"myvalue"}): {
					points:  []float64{300, 100, 300, 200},
					weights: []float64{10, 20, 10, 100},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			p := &StatsDParser{}
			assert.NoError(t, p.Initialize(false, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "summary"}, {StatsdType: "histogram", ObserverType: "summary"}}))
			addr, _ := net.ResolveUDPAddr("udp", "1.2.3.4:5678")
			addrKey := newNetAddr(addr)
			for _, line := range tt.input {
				err = p.Aggregate(line, addr)
			}
			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.EqualValues(t, tt.expectedSummaries, p.instrumentsByAddress[addrKey].summaries)
			}
		})
	}
}

func TestStatsDParser_Initialize(t *testing.T) {
	p := &StatsDParser{}
	assert.NoError(t, p.Initialize(true, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}}))
	teststatsdDMetricdescription := statsDMetricDescription{
		name:       "test",
		metricType: "g",
		attrs:      *attribute.EmptySet(),
	}
	addr, _ := net.ResolveUDPAddr("udp", "1.2.3.4:5678")
	addrKey := newNetAddr(addr)
	instrument := newInstruments(addr)
	instrument.gauges[teststatsdDMetricdescription] = pmetric.ScopeMetrics{}
	p.instrumentsByAddress[addrKey] = instrument
	assert.Equal(t, 1, len(p.instrumentsByAddress))
	assert.Equal(t, 1, len(p.instrumentsByAddress[addrKey].gauges))
	assert.Equal(t, GaugeObserver, p.timerEvents.method)
	assert.Equal(t, GaugeObserver, p.histogramEvents.method)
}

func TestStatsDParser_GetMetricsWithMetricType(t *testing.T) {
	p := &StatsDParser{}
	assert.NoError(t, p.Initialize(true, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}}))
	instrument := newInstruments(nil)
	instrument.gauges[testDescription("statsdTestMetric1", "g",
		[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"})] = buildGaugeMetric(testStatsDMetric("testGauge1", 1, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0))
	instrument.gauges[testDescription("statsdTestMetric1", "g",
		[]string{"mykey2", "metric_type"}, []string{"myvalue2", "gauge"})] = buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 10102, false, "g", 0, []string{"mykey2", "metric_type"}, []string{"myvalue2", "gauge"}), time.Unix(711, 0))
	instrument.counters[testDescription("statsdTestMetric1", "g",
		[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"})] = buildCounterMetric(testStatsDMetric("statsdTestMetric1", 10102, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), false)
	instrument.timersAndDistributions = append(instrument.timersAndDistributions, buildGaugeMetric(testStatsDMetric("statsdTestMetric1", 10102, false, "ms", 0, []string{"mykey2", "metric_type"}, []string{"myvalue2", "gauge"}), time.Unix(711, 0)))
	instrument.summaries = map[statsDMetricDescription]summaryMetric{
		testDescription("statsdTestMetric1", "h",
			[]string{"mykey"}, []string{"myvalue"}): {
			points:  []float64{1, 1, 10, 20},
			weights: []float64{1, 1, 1, 1},
		},
	}
	p.instrumentsByAddress[netAddr{}] = instrument
	metrics := p.GetMetrics()[0].Metrics
	assert.Equal(t, 5, metrics.ResourceMetrics().At(0).ScopeMetrics().Len())
}

func TestStatsDParser_Mappings(t *testing.T) {
	type testCase struct {
		name    string
		mapping []TimerHistogramMapping
		expect  map[string]string
	}

	for _, tc := range []testCase{
		{
			name: "timer-gauge-histo-summary",
			mapping: []TimerHistogramMapping{
				{StatsdType: "timer", ObserverType: "gauge"},
				{StatsdType: "histogram", ObserverType: "summary"},
			},
			expect: map[string]string{
				"Summary": "H",
				"Gauge":   "T",
			},
		},
		{
			name: "histo-to-summary",
			mapping: []TimerHistogramMapping{
				{StatsdType: "histogram", ObserverType: "summary"},
			},
			expect: map[string]string{
				"Summary": "H",
			},
		},
		{
			name: "timer-summary-histo-gauge",
			mapping: []TimerHistogramMapping{
				{StatsdType: "timer", ObserverType: "summary"},
				{StatsdType: "histogram", ObserverType: "gauge"},
			},
			expect: map[string]string{
				"Summary": "T",
				"Gauge":   "H",
			},
		},
		{
			name: "timer-to-gauge",
			mapping: []TimerHistogramMapping{
				{StatsdType: "timer", ObserverType: "gauge"},
			},
			expect: map[string]string{
				"Gauge": "T",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &StatsDParser{}

			assert.NoError(t, p.Initialize(false, false, tc.mapping))

			addr, _ := net.ResolveUDPAddr("udp", "1.2.3.4:5678")
			assert.NoError(t, p.Aggregate("H:10|h", addr))
			assert.NoError(t, p.Aggregate("T:10|ms", addr))

			typeNames := map[string]string{}

			metrics := p.GetMetrics()[0].Metrics
			ilm := metrics.ResourceMetrics().At(0).ScopeMetrics()
			for i := 0; i < ilm.Len(); i++ {
				ilms := ilm.At(i).Metrics()
				for j := 0; j < ilms.Len(); j++ {
					m := ilms.At(j)
					typeNames[m.Type().String()] = m.Name()
				}
			}

			assert.Equal(t, tc.expect, typeNames)
		})
	}
}

func TestTimeNowFunc(t *testing.T) {
	timeNow := timeNowFunc()
	assert.NotNil(t, timeNow)
}

func TestStatsDParser_AggregateTimerWithHistogram(t *testing.T) {
	timeNowFunc = func() time.Time {
		return time.Unix(711, 0)
	}
	// It is easiest to validate data in tests such as this by limiting the
	// histogram size to a small number and then setting the maximum range
	// to test at scale 0, which is easy to reason about.  The tests use
	// max size 10, so tests w/ a range of 2**10 appear below.
	normalMapping := []TimerHistogramMapping{
		{
			StatsdType:   "timer",
			ObserverType: "histogram",
			Histogram: HistogramConfig{
				MaxSize: 10,
			},
		},
		{
			StatsdType:   "histogram",
			ObserverType: "histogram",
			Histogram: HistogramConfig{
				MaxSize: 10,
			},
		},
	}

	newPoint := func() (pmetric.Metrics, pmetric.ExponentialHistogramDataPoint) {
		data := pmetric.NewMetrics()
		ilm := data.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
		m := ilm.Metrics().AppendEmpty()
		m.SetName("expohisto")
		ep := m.SetEmptyExponentialHistogram()
		ep.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := ep.DataPoints().AppendEmpty()

		dp.Attributes().PutStr("mykey", "myvalue")
		return data, dp
	}

	tests := []struct {
		name     string
		input    []string
		expected pmetric.Metrics
		mapping  []TimerHistogramMapping
	}{
		{
			name: "basic",
			input: []string{
				"expohisto:0|ms|#mykey:myvalue",
				"expohisto:1.5|ms|#mykey:myvalue",
				"expohisto:2.5|ms|#mykey:myvalue",
				"expohisto:4.5|ms|#mykey:myvalue",
				"expohisto:8.5|ms|#mykey:myvalue",
				"expohisto:16.5|ms|#mykey:myvalue",
				"expohisto:32.5|ms|#mykey:myvalue",
				"expohisto:64.5|ms|#mykey:myvalue",
				"expohisto:128.5|ms|#mykey:myvalue",
				"expohisto:256.5|ms|#mykey:myvalue",
				"expohisto:512.5|ms|#mykey:myvalue",
			},
			expected: func() pmetric.Metrics {
				data, dp := newPoint()
				dp.SetCount(11)
				dp.SetSum(1028)
				dp.SetMin(0)
				dp.SetMax(512.5)
				dp.SetZeroCount(1)
				dp.SetScale(0)
				dp.Positive().SetOffset(0)
				dp.Positive().BucketCounts().FromRaw([]uint64{
					1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
				})
				return data
			}(),
			mapping: normalMapping,
		},
		{
			name: "negative",
			input: []string{
				"expohisto:-0|ms|#mykey:myvalue",
				"expohisto:-1.5|ms|#mykey:myvalue",
				"expohisto:-2.5|ms|#mykey:myvalue",
				"expohisto:-4.5|ms|#mykey:myvalue",
				"expohisto:-8.5|ms|#mykey:myvalue",
				"expohisto:-16.5|ms|#mykey:myvalue",
				"expohisto:-32.5|ms|#mykey:myvalue",
				"expohisto:-64.5|ms|#mykey:myvalue",
				"expohisto:-128.5|ms|#mykey:myvalue",
				"expohisto:-256.5|ms|#mykey:myvalue",
				"expohisto:-512.5|ms|#mykey:myvalue",
			},
			expected: func() pmetric.Metrics {
				data, dp := newPoint()
				dp.SetCount(11)
				dp.SetSum(-1028)
				dp.SetMin(-512.5)
				dp.SetMax(0)
				dp.SetZeroCount(1)
				dp.SetScale(0)
				dp.Negative().SetOffset(0)
				dp.Negative().BucketCounts().FromRaw([]uint64{
					1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
				})
				return data
			}(),
			mapping: normalMapping,
		},
		{
			name: "halffull",
			input: []string{
				"expohisto:1.5|ms|#mykey:myvalue",
				"expohisto:4.5|ms|#mykey:myvalue",
				"expohisto:16.5|ms|#mykey:myvalue",
				"expohisto:64.5|ms|#mykey:myvalue",
				"expohisto:512.5|ms|#mykey:myvalue",
			},
			expected: func() pmetric.Metrics {
				data, dp := newPoint()
				dp.SetCount(5)
				dp.SetSum(599.5)
				dp.SetMin(1.5)
				dp.SetMax(512.5)
				dp.SetZeroCount(0)
				dp.SetScale(0)
				dp.Positive().SetOffset(0)
				dp.Positive().BucketCounts().FromRaw([]uint64{
					1, 0, 1, 0, 1, 0, 1, 0, 0, 1,
				})
				return data
			}(),
			mapping: normalMapping,
		},
		{
			name: "one_each",
			input: []string{
				"expohisto:1|h|#mykey:myvalue",
				"expohisto:0|h|#mykey:myvalue",
				"expohisto:-1|h|#mykey:myvalue",
			},
			expected: func() pmetric.Metrics {
				data, dp := newPoint()
				dp.SetCount(3)
				dp.SetSum(0)
				dp.SetMin(-1)
				dp.SetMax(1)
				dp.SetZeroCount(1)
				dp.SetScale(logarithm.MaxScale)
				dp.Positive().SetOffset(-1)
				dp.Negative().SetOffset(-1)
				dp.Positive().BucketCounts().FromRaw([]uint64{
					1,
				})
				dp.Negative().BucketCounts().FromRaw([]uint64{
					1,
				})
				return data
			}(),
			mapping: normalMapping,
		},
		{
			name: "all_zeros",
			input: []string{
				"expohisto:0|h|#mykey:myvalue",
				"expohisto:0|h|#mykey:myvalue",
				"expohisto:0|h|#mykey:myvalue",
				"expohisto:0|h|#mykey:myvalue",
			},
			expected: func() pmetric.Metrics {
				data, dp := newPoint()
				dp.SetCount(4)
				dp.SetSum(0)
				dp.SetMin(0)
				dp.SetMax(0)
				dp.SetZeroCount(4)
				dp.SetScale(0)
				return data
			}(),
			mapping: normalMapping,
		},
		{
			name: "sampled",
			input: []string{
				"expohisto:1|h|@0.125|#mykey:myvalue",
				"expohisto:0|h|@0.25|#mykey:myvalue",
				"expohisto:-1|h|@0.5|#mykey:myvalue",
			},
			expected: func() pmetric.Metrics {
				data, dp := newPoint()
				dp.SetCount(14)
				dp.SetSum(6)
				dp.SetMin(-1)
				dp.SetMax(1)
				dp.SetZeroCount(4)
				dp.SetScale(logarithm.MaxScale)
				dp.Positive().SetOffset(-1)
				dp.Negative().SetOffset(-1)
				dp.Positive().BucketCounts().FromRaw([]uint64{
					8, // 1 / 0.125
				})
				dp.Negative().BucketCounts().FromRaw([]uint64{
					2, // 1 / 0.5
				})
				return data
			}(),
			mapping: normalMapping,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			p := &StatsDParser{}
			assert.NoError(t, p.Initialize(false, false, tt.mapping))
			addr, _ := net.ResolveUDPAddr("udp", "1.2.3.4:5678")
			for _, line := range tt.input {
				err = p.Aggregate(line, addr)
				assert.NoError(t, err)
			}
			var nodiffs []*metricstestutil.MetricDiff
			assert.Equal(t, nodiffs, metricstestutil.DiffMetrics(nodiffs, tt.expected, p.GetMetrics()[0].Metrics))
		})
	}
}

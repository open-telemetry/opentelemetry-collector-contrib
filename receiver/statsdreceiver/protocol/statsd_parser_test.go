// Copyright 2020, OpenTelemetry Authors
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

package protocol

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/otel/attribute"
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
				"42",
				42,
				0,
				false,
				"c", 0, nil, nil),
		},
		{
			name:  "invalid  counter metric value",
			input: "test.metric:42.abc|c",
			err:   errors.New("counter: parse metric value string: 42.abc"),
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
				"42",
				420,
				0,
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
				"42",
				52,
				0,
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
				"42",
				52,
				0,
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
				"42.0",
				0,
				42,
				false,
				"g", 0, nil, nil),
		},
		{
			name:  "int gauge",
			input: "test.metric:42|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				"42",
				0,
				42,
				false,
				"g", 0, nil, nil),
		},
		{
			name:  "invalid gauge metric value",
			input: "test.metric:42.abc|g",
			err:   errors.New("gauge: parse metric value string: 42.abc"),
		},
		{
			name:  "gauge metric with sample rate and tag",
			input: "test.metric:11|g|@0.1|#key:value",
			wantMetric: testStatsDMetric(
				"test.metric",
				"11",
				0,
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
				"11",
				0,
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
				"+42.0",
				0,
				42,
				true,
				"g", 0, nil, nil),
		},
		{
			name:  "double gauge minus",
			input: "test.metric:-42.0|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				"-42.0",
				0,
				-42,
				true,
				"g", 0, nil, nil),
		},
		{
			name:  "int gauge plus",
			input: "test.metric:+42|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				"+42",
				0,
				42,
				true,
				"g", 0, nil, nil),
		},
		{
			name:  "int gauge minus",
			input: "test.metric:-42|g",
			wantMetric: testStatsDMetric(
				"test.metric",
				"-42",
				0,
				-42,
				true,
				"g", 0, nil, nil),
		},
		{
			name:  "invalid histogram metric value",
			input: "test.metric:42.abc|h",
			err:   errors.New("timing/histogram: parse metric value string: 42.abc"),
		},
		{
			name:  "int timer",
			input: "test.metric:-42|ms",
			wantMetric: testStatsDMetric(
				"test.metric",
				"-42",
				0,
				-42,
				true,
				"ms", 0, nil, nil),
		},
		{
			name:  "int histogram",
			input: "test.metric:42|h",
			wantMetric: testStatsDMetric(
				"test.metric",
				"42",
				0,
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
				"42",
				42,
				0,
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
				"42",
				420,
				0,
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
				"42",
				52,
				0,
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
				"42",
				52,
				0,
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
				"42.0",
				0,
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
				"42",
				0,
				42,
				false,
				"g", 0,
				[]string{"metric_type"},
				[]string{"gauge"}),
		},
		{
			name:  "invalid gauge metric value",
			input: "test.metric:42.abc|g",
			err:   errors.New("gauge: parse metric value string: 42.abc"),
		},
		{
			name:  "gauge metric with sample rate and tag",
			input: "test.metric:11|g|@0.1|#key:value",
			wantMetric: testStatsDMetric(
				"test.metric",
				"11",
				0,
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
				"11",
				0,
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
				"+42.0",
				0,
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
				"-42.0",
				0,
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
				"+42",
				0,
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
				"-42",
				0,
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
				"-42",
				0,
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
				"42",
				0,
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

func testStatsDMetric(name string, value string,
	intValue int64, floatValue float64,
	addition bool, statsdMetricType string,
	sampleRate float64, labelKeys []string,
	labelValue []string) statsDMetric {
	if len(labelKeys) > 0 {
		var kvs []attribute.KeyValue
		var sortable attribute.Sortable
		for n, k := range labelKeys {
			kvs = append(kvs, attribute.String(k, labelValue[n]))
		}
		set := attribute.NewSetWithSortable(kvs, &sortable)
		return statsDMetric{
			description: statsDMetricdescription{
				name:             name,
				statsdMetricType: statsdMetricType,
				labels:           set.Equivalent(),
			},
			value:       value,
			intvalue:    intValue,
			floatvalue:  floatValue,
			addition:    addition,
			unit:        "",
			sampleRate:  sampleRate,
			labelKeys:   labelKeys,
			labelValues: labelValue,
		}
	}
	return statsDMetric{
		description: statsDMetricdescription{
			name:             name,
			statsdMetricType: statsdMetricType,
		},
		value:       value,
		intvalue:    intValue,
		floatvalue:  floatValue,
		addition:    addition,
		unit:        "",
		sampleRate:  sampleRate,
		labelKeys:   labelKeys,
		labelValues: labelValue,
	}
}

func testDescription(name string, statsdMetricType string, keys []string, values []string) statsDMetricdescription {
	var kvs []attribute.KeyValue
	var sortable attribute.Sortable
	for n, k := range keys {
		kvs = append(kvs, attribute.String(k, values[n]))
	}
	set := attribute.NewSetWithSortable(kvs, &sortable)
	return statsDMetricdescription{
		name:             name,
		statsdMetricType: statsdMetricType,
		labels:           set.Equivalent(),
	}
}
func TestStatsDParser_Aggregate(t *testing.T) {
	timeNowFunc = func() time.Time {
		return time.Unix(711, 0)
	}

	tests := []struct {
		name             string
		input            []string
		expectedGauges   map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
		expectedCounters map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
		expectedTimer    []pdata.InstrumentationLibraryMetrics
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
			expectedGauges: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 10102, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", "", 0, 507, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{},
			expectedTimer:    []pdata.InstrumentationLibraryMetrics{},
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
			expectedGauges: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 4885, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", "", 0, 5, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{},
			expectedTimer:    []pdata.InstrumentationLibraryMetrics{},
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
			expectedGauges: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 4101, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", "", 0, 200, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{},
			expectedTimer:    []pdata.InstrumentationLibraryMetrics{},
		},
		{
			name: "counter with increment and sample rate",
			input: []string{
				"statsdTestMetric1:3000|c|#mykey:myvalue",
				"statsdTestMetric1:4000|c|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", "", 7000, 0, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false, time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", "", 50, 0, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false, time.Unix(711, 0)),
			},
			expectedTimer: []pdata.InstrumentationLibraryMetrics{},
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
			expectedGauges: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 421, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", "", 7000, 0, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false, time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", "", 50, 0, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false, time.Unix(711, 0)),
			},
			expectedTimer: []pdata.InstrumentationLibraryMetrics{},
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
			expectedGauges: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 319, false, "g", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey"}, []string{"myvalue1"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 399, false, "g", 0, []string{"mykey"}, []string{"myvalue1"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", "", 215, 0, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false, time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", "", 75, 0, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), false, time.Unix(711, 0)),
			},
			expectedTimer: []pdata.InstrumentationLibraryMetrics{},
		},
		{
			name: "counter and gauge: 2 timings and 2 histograms",
			input: []string{
				"statsdTestMetric1:500|ms|#mykey:myvalue",
				"statsdTestMetric1:400|h|#mykey:myvalue",
				"statsdTestMetric1:300|ms|#mykey:myvalue",
				"statsdTestMetric1:10|h|@0.1|#mykey:myvalue",
			},
			expectedGauges:   map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{},
			expectedTimer: []pdata.InstrumentationLibraryMetrics{
				buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 500, false, "ms", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 400, false, "h", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 300, false, "ms", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
				buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 100, false, "h", 0, []string{"mykey"}, []string{"myvalue"}), time.Unix(711, 0)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			p := &StatsDParser{}
			p.Initialize(false, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}})
			for _, line := range tt.input {
				err = p.Aggregate(line)
			}
			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.Equal(t, tt.expectedGauges, p.gauges)
				assert.Equal(t, tt.expectedCounters, p.counters)
				assert.Equal(t, tt.expectedTimer, p.timersAndDistributions)
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
		expectedGauges   map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
		expectedCounters map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
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
			expectedGauges: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "g",
					[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 10102, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "g",
					[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}): buildGaugeMetric(testStatsDMetric("statsdTestMetric2", "", 0, 507, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0)),
			},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{},
		},

		{
			name: "counter with increment and sample rate",
			input: []string{
				"statsdTestMetric1:3000|c|#mykey:myvalue",
				"statsdTestMetric1:4000|c|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
				"statsdTestMetric2:20|c|@0.8|#mykey:myvalue",
			},
			expectedGauges: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey", "metric_type"}, []string{"myvalue", "counter"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", "", 7000, 0, false, "c", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "counter"}), false, time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey", "metric_type"}, []string{"myvalue", "counter"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", "", 50, 0, false, "c", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "counter"}), false, time.Unix(711, 0)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			p := &StatsDParser{}
			p.Initialize(true, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}})
			for _, line := range tt.input {
				err = p.Aggregate(line)
			}
			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.Equal(t, tt.expectedGauges, p.gauges)
				assert.Equal(t, tt.expectedCounters, p.counters)
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
		expectedGauges   map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
		expectedCounters map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
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
			expectedGauges: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{},
			expectedCounters: map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics{
				testDescription("statsdTestMetric1", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric1", "", 7000, 0, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), true, time.Unix(711, 0)),
				testDescription("statsdTestMetric2", "c",
					[]string{"mykey"}, []string{"myvalue"}): buildCounterMetric(testStatsDMetric("statsdTestMetric2", "", 50, 0, false, "c", 0, []string{"mykey"}, []string{"myvalue"}), true, time.Unix(711, 0)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			p := &StatsDParser{}
			p.Initialize(false, true, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}})
			for _, line := range tt.input {
				err = p.Aggregate(line)
			}
			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.Equal(t, tt.expectedGauges, p.gauges)
				assert.Equal(t, tt.expectedCounters, p.counters)
			}
		})
	}
}

func TestStatsDParser_AggregateTimmerWithSummary(t *testing.T) {
	timeNowFunc = func() time.Time {
		return time.Unix(711, 0)
	}

	tests := []struct {
		name              string
		input             []string
		expectedSummaries map[statsDMetricdescription]summaryMetric
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
				"statsdTestMetric1:20|ms|@0.1|#mykey:myvalue",
			},
			expectedSummaries: map[statsDMetricdescription]summaryMetric{
				testDescription("statsdTestMetric1", "ms",
					[]string{"mykey"}, []string{"myvalue"}): {
					name:          "statsdTestMetric1",
					summaryPoints: []float64{1, 1, 10, 20, 200},
					labelKeys:     []string{"mykey"},
					labelValues:   []string{"myvalue"},
					timeNow:       timeNowFunc(),
				},
				testDescription("statsdTestMetric2", "ms",
					[]string{"mykey"}, []string{"myvalue"}): {
					name:          "statsdTestMetric2",
					summaryPoints: []float64{2, 5, 10},
					labelKeys:     []string{"mykey"},
					labelValues:   []string{"myvalue"},
					timeNow:       timeNowFunc(),
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
			expectedSummaries: map[statsDMetricdescription]summaryMetric{
				testDescription("statsdTestMetric1", "h",
					[]string{"mykey"}, []string{"myvalue"}): {
					name:          "statsdTestMetric1",
					summaryPoints: []float64{1, 1, 10, 20},
					labelKeys:     []string{"mykey"},
					labelValues:   []string{"myvalue"},
					timeNow:       timeNowFunc(),
				},
				testDescription("statsdTestMetric2", "h",
					[]string{"mykey"}, []string{"myvalue"}): {
					name:          "statsdTestMetric2",
					summaryPoints: []float64{2, 5, 10},
					labelKeys:     []string{"mykey"},
					labelValues:   []string{"myvalue"},
					timeNow:       timeNowFunc(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			p := &StatsDParser{}
			p.Initialize(false, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "summary"}, {StatsdType: "histogram", ObserverType: "summary"}})
			for _, line := range tt.input {
				err = p.Aggregate(line)
			}
			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.Equal(t, tt.expectedSummaries, p.summaries)
			}
		})
	}
}

func TestStatsDParser_Initialize(t *testing.T) {
	p := &StatsDParser{}
	p.Initialize(true, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}})
	labels := attribute.Distinct{}
	teststatsdDMetricdescription := statsDMetricdescription{
		name:             "test",
		statsdMetricType: "g",
		labels:           labels}
	p.gauges[teststatsdDMetricdescription] = pdata.InstrumentationLibraryMetrics{}
	assert.Equal(t, 1, len(p.gauges))
	assert.Equal(t, "gauge", p.observeTimer)
	assert.Equal(t, "gauge", p.observeHistogram)
}

func TestStatsDParser_GetMetrics(t *testing.T) {
	p := &StatsDParser{}
	p.Initialize(true, false, []TimerHistogramMapping{{StatsdType: "timer", ObserverType: "gauge"}, {StatsdType: "histogram", ObserverType: "gauge"}})
	p.gauges[testDescription("statsdTestMetric1", "g",
		[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"})] =
		buildGaugeMetric(testStatsDMetric("testGauge1", "", 0, 1, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0))
	p.gauges[testDescription("statsdTestMetric1", "g",
		[]string{"mykey2", "metric_type"}, []string{"myvalue2", "gauge"})] =
		buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 10102, false, "g", 0, []string{"mykey2", "metric_type"}, []string{"myvalue2", "gauge"}), time.Unix(711, 0))
	p.counters[testDescription("statsdTestMetric1", "g",
		[]string{"mykey", "metric_type"}, []string{"myvalue", "gauge"})] =
		buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 10102, false, "g", 0, []string{"mykey", "metric_type"}, []string{"myvalue", "gauge"}), time.Unix(711, 0))
	p.timersAndDistributions = append(p.timersAndDistributions, buildGaugeMetric(testStatsDMetric("statsdTestMetric1", "", 0, 10102, false, "ms", 0, []string{"mykey2", "metric_type"}, []string{"myvalue2", "gauge"}), time.Unix(711, 0)))
	p.summaries = map[statsDMetricdescription]summaryMetric{
		testDescription("statsdTestMetric1", "h",
			[]string{"mykey"}, []string{"myvalue"}): {
			name:          "statsdTestMetric1",
			summaryPoints: []float64{1, 1, 10, 20},
			labelKeys:     []string{"mykey"},
			labelValues:   []string{"myvalue"},
			timeNow:       timeNowFunc(),
		}}
	metrics := p.GetMetrics()
	assert.Equal(t, 5, metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().Len())
}

func TestTimeNowFunc(t *testing.T) {
	timeNow := timeNowFunc()
	assert.NotNil(t, timeNow)
}

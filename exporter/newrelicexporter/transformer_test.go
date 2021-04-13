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

package newrelicexporter

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
)

func TestCommonAttributes(t *testing.T) {
	startInfo := &component.ApplicationStartInfo{
		ExeName: "the-collector",
		Version: "0.0.1",
	}

	resource := pdata.NewResource()
	resource.Attributes().InsertString("resource", "R1")

	ilm := pdata.NewInstrumentationLibrary()
	ilm.SetName("test name")
	ilm.SetVersion("test version")

	commonAttrs := newTransformer(startInfo, nil).CommonAttributes(resource, ilm)
	assert.Equal(t, "the-collector", commonAttrs[collectorNameKey])
	assert.Equal(t, "0.0.1", commonAttrs[collectorVersionKey])
	assert.Equal(t, "R1", commonAttrs["resource"])
	assert.Equal(t, "test name", commonAttrs[instrumentationNameKey])
	assert.Equal(t, "test version", commonAttrs[instrumentationVersionKey])
}

func TestCaptureSpanMetadata(t *testing.T) {
	details := newTraceMetadata(context.TODO())
	transform := newTransformer(nil, &details)

	tests := []struct {
		name     string
		err      error
		spanFunc func() pdata.Span
		wantKey  spanStatsKey
	}{
		{
			name: "no events or links",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 1}))
				s.SetName("no events or links")
				return s
			},
			err:     errInvalidTraceID,
			wantKey: spanStatsKey{hasEvents: false, hasLinks: false},
		},
		{
			name: "has events but no links",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetName("invalid SpanID")
				s.Events().Append(pdata.NewSpanEvent())
				return s
			},
			err:     errInvalidSpanID,
			wantKey: spanStatsKey{hasEvents: true, hasLinks: false},
		},
		{
			name: "no events but has links",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 1}))
				s.SetName("no events but has links")
				s.Links().Append(pdata.NewSpanLink())
				return s
			},
			wantKey: spanStatsKey{hasEvents: false, hasLinks: true},
		},
		{
			name: "has events and links",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 2}))
				s.SetParentSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 1}))
				s.SetName("has events and links")
				s.Events().Append(pdata.NewSpanEvent())
				s.Links().Append(pdata.NewSpanLink())
				return s
			},
			wantKey: spanStatsKey{hasEvents: true, hasLinks: true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := transform.Span(test.spanFunc())
			if test.err != nil {
				assert.True(t, errors.Is(err, test.err))
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, 1, details.spanMetadataCount[test.wantKey])
		})
	}
}

func TestTransformSpan(t *testing.T) {
	now := time.Unix(100, 0)
	details := newTraceMetadata(context.TODO())
	transform := newTransformer(nil, &details)

	tests := []struct {
		name     string
		err      error
		spanFunc func() pdata.Span
		want     telemetry.Span
	}{
		{
			name: "invalid TraceID",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 1}))
				s.SetName("invalid TraceID")
				return s
			},
			err: errInvalidTraceID,
			want: telemetry.Span{
				ID:         "0000000000000001",
				Name:       "invalid TraceID",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{},
			},
		},
		{
			name: "invalid SpanID",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetName("invalid SpanID")
				return s
			},
			err: errInvalidSpanID,
			want: telemetry.Span{
				TraceID:    "01010101010101010101010101010101",
				Name:       "invalid SpanID",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{},
			},
		},
		{
			name: "root",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 1}))
				s.SetName("root")
				return s
			},
			want: telemetry.Span{
				ID:         "0000000000000001",
				TraceID:    "01010101010101010101010101010101",
				Name:       "root",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{},
				Events:     nil,
			},
		},
		{
			name: "client",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 2}))
				s.SetParentSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 1}))
				s.SetName("client")
				return s
			},
			want: telemetry.Span{
				ID:         "0000000000000002",
				TraceID:    "01010101010101010101010101010101",
				Name:       "client",
				ParentID:   "0000000000000001",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{},
				Events:     nil,
			},
		},
		{
			name: "error code",
			spanFunc: func() pdata.Span {
				// There is no setter method for a Status so convert instead.
				return internaldata.OCToTraces(
					nil, nil, []*tracepb.Span{
						{
							TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
							SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 3},
							Name:    &tracepb.TruncatableString{Value: "error code"},
							Status:  &tracepb.Status{Code: 1},
						},
					}).ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0)
			},
			want: telemetry.Span{
				ID:        "0000000000000003",
				TraceID:   "01010101010101010101010101010101",
				Name:      "error code",
				Timestamp: time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{
					statusCodeKey: "ERROR",
				},
				Events: nil,
			},
		},
		{
			name: "error message",
			spanFunc: func() pdata.Span {
				// There is no setter method for a Status so convert instead.
				return internaldata.OCToTraces(
					nil, nil, []*tracepb.Span{
						{
							TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
							SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 3},
							Name:    &tracepb.TruncatableString{Value: "error message"},
							Status:  &tracepb.Status{Code: 1, Message: "error message"},
						},
					}).ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0)
			},
			want: telemetry.Span{
				ID:        "0000000000000003",
				TraceID:   "01010101010101010101010101010101",
				Name:      "error message",
				Timestamp: time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{
					statusCodeKey:        "ERROR",
					statusDescriptionKey: "error message",
				},
				Events: nil,
			},
		},
		{
			name: "attributes",
			spanFunc: func() pdata.Span {
				// There is no setter method for attributes so convert instead.
				return internaldata.OCToTraces(
					nil, nil, []*tracepb.Span{
						{
							TraceId: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
							SpanId:  []byte{0, 0, 0, 0, 0, 0, 0, 4},
							Name:    &tracepb.TruncatableString{Value: "attrs"},
							Status:  &tracepb.Status{},
							Attributes: &tracepb.Span_Attributes{
								AttributeMap: map[string]*tracepb.AttributeValue{
									"prod": {
										Value: &tracepb.AttributeValue_BoolValue{
											BoolValue: true,
										},
									},
									"weight": {
										Value: &tracepb.AttributeValue_IntValue{
											IntValue: 10,
										},
									},
									"score": {
										Value: &tracepb.AttributeValue_DoubleValue{
											DoubleValue: 99.8,
										},
									},
									"user": {
										Value: &tracepb.AttributeValue_StringValue{
											StringValue: &tracepb.TruncatableString{Value: "alice"},
										},
									},
								},
							},
						},
					}).ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0)
			},
			want: telemetry.Span{
				ID:        "0000000000000004",
				TraceID:   "01010101010101010101010101010101",
				Name:      "attrs",
				Timestamp: time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{
					"prod":   true,
					"weight": int64(10),
					"score":  99.8,
					"user":   "alice",
				},
				Events: nil,
			},
		},
		{
			name: "with timestamps",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 5}))
				s.SetName("with time")
				s.SetStartTimestamp(pdata.TimestampFromTime(now))
				s.SetEndTimestamp(pdata.TimestampFromTime(now.Add(time.Second * 5)))
				return s
			},
			want: telemetry.Span{
				ID:         "0000000000000005",
				TraceID:    "01010101010101010101010101010101",
				Name:       "with time",
				Timestamp:  now.UTC(),
				Duration:   time.Second * 5,
				Attributes: map[string]interface{}{},
				Events:     nil,
			},
		},
		{
			name: "span kind server",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 6}))
				s.SetName("span kind server")
				s.SetKind(pdata.SpanKindSERVER)
				return s
			},
			want: telemetry.Span{
				ID:        "0000000000000006",
				TraceID:   "01010101010101010101010101010101",
				Name:      "span kind server",
				Timestamp: time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{
					spanKindKey: "server",
				},
				Events: nil,
			},
		},
		{
			name: "with events",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 7}))
				s.SetName("with events")

				ev := pdata.NewSpanEventSlice()
				ev.Resize(1)
				event := ev.At(0)
				event.SetName("this is the event name")
				event.SetTimestamp(pdata.TimestampFromTime(now))
				s.Events().Append(event)
				return s
			},
			want: telemetry.Span{
				ID:         "0000000000000007",
				TraceID:    "01010101010101010101010101010101",
				Name:       "with events",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{},
				Events: []telemetry.Event{
					{
						EventType:  "this is the event name",
						Timestamp:  now.UTC(),
						Attributes: map[string]interface{}{},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := transform.Span(test.spanFunc())
			if test.err != nil {
				assert.True(t, errors.Is(err, test.err))
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, test.want, got)
		})
	}
}

func testTransformMetric(t *testing.T, metric pdata.Metric, want []telemetry.Metric) {
	details := newMetricMetadata(context.Background())
	transform := newTransformer(&component.ApplicationStartInfo{
		ExeName: testCollectorName,
		Version: testCollectorVersion,
	}, &details)
	got, err := transform.Metric(metric)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	assert.Equal(t, len(details.metricMetadataCount), 1)
	for k, v := range details.metricMetadataCount {
		assert.Equal(t, metric.DataType(), k.MetricType)
		assert.Equal(t, 1, v)
	}
}

func testTransformMetricWithError(t *testing.T, metric pdata.Metric, expectedErrorType interface{}) {
	details := newMetricMetadata(context.Background())
	transform := newTransformer(&component.ApplicationStartInfo{
		ExeName: testCollectorName,
		Version: testCollectorVersion,
	}, &details)
	_, err := transform.Metric(metric)
	assert.IsType(t, expectedErrorType, err)

	assert.Equal(t, len(details.metricMetadataCount), 1)
	for k, v := range details.metricMetadataCount {
		assert.Equal(t, metric.DataType(), k.MetricType)
		assert.Equal(t, 1, v)
	}
}

func TestTransformGauge(t *testing.T) {
	ts := pdata.TimestampFromTime(time.Unix(1, 0))
	expected := []telemetry.Metric{
		telemetry.Gauge{
			Name:      "gauge",
			Value:     42.0,
			Timestamp: ts.AsTime(),
			Attributes: map[string]interface{}{
				"unit":        "1",
				"description": "description",
			},
		},
	}
	{
		m := pdata.NewMetric()
		m.SetName("gauge")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeDoubleGauge)
		gd := m.DoubleGauge()
		dp := pdata.NewDoubleDataPoint()
		dp.SetTimestamp(ts)
		dp.SetValue(42.0)
		gd.DataPoints().Append(dp)
		t.Run("Double", func(t *testing.T) { testTransformMetric(t, m, expected) })
	}
	{
		m := pdata.NewMetric()
		m.SetName("gauge")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeIntGauge)
		gi := m.IntGauge()
		dp := pdata.NewIntDataPoint()
		dp.SetTimestamp(ts)
		dp.SetValue(42)
		gi.DataPoints().Append(dp)
		t.Run("Int64", func(t *testing.T) { testTransformMetric(t, m, expected) })
	}
}

func TestTransformSum(t *testing.T) {
	start := pdata.TimestampFromTime(time.Unix(1, 0))
	end := pdata.TimestampFromTime(time.Unix(3, 0))

	expected := []telemetry.Metric{
		telemetry.Count{
			Name:      "sum",
			Value:     42.0,
			Timestamp: start.AsTime(),
			Interval:  time.Second * 2,
			Attributes: map[string]interface{}{
				"unit":        "1",
				"description": "description",
			},
		},
	}

	{
		m := pdata.NewMetric()
		m.SetName("sum")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeDoubleSum)
		d := m.DoubleSum()
		d.SetAggregationTemporality(pdata.AggregationTemporalityDelta)
		dp := pdata.NewDoubleDataPoint()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetValue(42.0)
		d.DataPoints().Append(dp)
		t.Run("DoubleSum-Delta", func(t *testing.T) { testTransformMetric(t, m, expected) })
	}
	{
		m := pdata.NewMetric()
		m.SetName("sum")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeDoubleSum)
		d := m.DoubleSum()
		d.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		dp := pdata.NewDoubleDataPoint()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetValue(42.0)
		d.DataPoints().Append(dp)
		t.Run("DoubleSum-Cumulative", func(t *testing.T) { testTransformMetricWithError(t, m, &errUnsupportedMetricType{}) })
	}
	{
		m := pdata.NewMetric()
		m.SetName("sum")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeIntSum)
		d := m.IntSum()
		d.SetAggregationTemporality(pdata.AggregationTemporalityDelta)
		dp := pdata.NewIntDataPoint()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetValue(42.0)
		d.DataPoints().Append(dp)
		t.Run("IntSum-Delta", func(t *testing.T) { testTransformMetric(t, m, expected) })
	}
	{
		m := pdata.NewMetric()
		m.SetName("sum")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeIntSum)
		d := m.IntSum()
		d.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		dp := pdata.NewIntDataPoint()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetValue(42.0)
		d.DataPoints().Append(dp)
		t.Run("IntSum-Cumulative", func(t *testing.T) { testTransformMetricWithError(t, m, &errUnsupportedMetricType{}) })
	}
}

func TestTransformDeltaSummary(t *testing.T) {
	start := pdata.TimestampFromTime(time.Unix(1, 0))
	end := pdata.TimestampFromTime(time.Unix(3, 0))

	expected := []telemetry.Metric{
		telemetry.Summary{
			Name:      "summary",
			Count:     2.0,
			Sum:       7.0,
			Min:       1,
			Max:       6,
			Timestamp: time.Unix(1, 0).UTC(),
			Interval:  2 * time.Second,
			Attributes: map[string]interface{}{
				"description": "description",
				"unit":        "s",
				"foo":         "bar",
			},
		},
	}

	m := pdata.NewMetric()
	m.SetName("summary")
	m.SetDescription("description")
	m.SetUnit("s")
	m.SetDataType(pdata.MetricDataTypeSummary)
	ds := m.Summary()
	dp := pdata.NewSummaryDataPoint()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(end)
	dp.SetSum(7)
	dp.SetCount(2)
	dp.LabelsMap().Insert("foo", "bar")
	q := dp.QuantileValues()
	min := pdata.NewValueAtQuantile()
	min.SetQuantile(0)
	min.SetValue(1)
	max := pdata.NewValueAtQuantile()
	max.SetQuantile(1)
	max.SetValue(6)
	q.Append(min)
	q.Append(max)
	ds.DataPoints().Append(dp)

	t.Run("Double", func(t *testing.T) { testTransformMetric(t, m, expected) })
}

func TestUnsupportedMetricTypes(t *testing.T) {
	start := pdata.TimestampFromTime(time.Unix(1, 0))
	end := pdata.TimestampFromTime(time.Unix(3, 0))

	{
		m := pdata.NewMetric()
		m.SetName("no")
		m.SetDescription("no")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeIntHistogram)
		h := m.IntHistogram()
		dp := pdata.NewIntHistogramDataPoint()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetCount(2)
		dp.SetSum(8)
		dp.SetExplicitBounds([]float64{3, 7, 11})
		dp.SetBucketCounts([]uint64{1, 1, 0, 0})
		h.SetAggregationTemporality(pdata.AggregationTemporalityDelta)
		h.DataPoints().Append(dp)

		t.Run("IntHistogram", func(t *testing.T) { testTransformMetricWithError(t, m, &errUnsupportedMetricType{}) })
	}
	{
		m := pdata.NewMetric()
		m.SetName("no")
		m.SetDescription("no")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeHistogram)
		h := m.Histogram()
		dp := pdata.NewHistogramDataPoint()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetCount(2)
		dp.SetSum(8.0)
		dp.SetExplicitBounds([]float64{3, 7, 11})
		dp.SetBucketCounts([]uint64{1, 1, 0, 0})
		h.SetAggregationTemporality(pdata.AggregationTemporalityDelta)
		h.DataPoints().Append(dp)

		t.Run("DoubleHistogram", func(t *testing.T) { testTransformMetricWithError(t, m, &errUnsupportedMetricType{}) })
	}
}

func TestTransformer_Log(t *testing.T) {
	tests := []struct {
		name    string
		logFunc func() pdata.LogRecord
		want    telemetry.Log
	}{
		{
			name: "Basic Conversion",
			logFunc: func() pdata.LogRecord {
				log := pdata.NewLogRecord()
				timestamp := pdata.TimestampFromTime(time.Unix(0, 0).UTC())
				log.SetTimestamp(timestamp)
				return log
			},
			want: telemetry.Log{
				Message:    "",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{"name": ""},
			},
		},
		{
			name: "With Log attributes",
			logFunc: func() pdata.LogRecord {
				log := pdata.NewLogRecord()
				log.SetName("bloopbleep")
				log.Attributes().InsertString("foo", "bar")
				log.Body().SetStringVal("Hello World")
				return log
			},
			want: telemetry.Log{
				Message:    "Hello World",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{"foo": "bar", "name": "bloopbleep"},
			},
		},
		{
			name: "With severity number",
			logFunc: func() pdata.LogRecord {
				log := pdata.NewLogRecord()
				log.SetName("bloopbleep")
				log.SetSeverityNumber(pdata.SeverityNumberWARN)
				return log
			},
			want: telemetry.Log{
				Message:    "bloopbleep",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{"name": "bloopbleep", "log.levelNum": int32(13)},
			},
		},
		{
			name: "With severity text",
			logFunc: func() pdata.LogRecord {
				log := pdata.NewLogRecord()
				log.SetName("bloopbleep")
				log.SetSeverityText("SEVERE")
				return log
			},
			want: telemetry.Log{
				Message:    "bloopbleep",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{"name": "bloopbleep", "log.level": "SEVERE"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transform := newTransformer(nil, nil)
			got, _ := transform.Log(test.logFunc())
			assert.EqualValues(t, test.want, got)
		})
	}
}

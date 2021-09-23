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
	"math"
	"testing"
	"time"

	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func TestCommonAttributes(t *testing.T) {
	buildInfo := &component.BuildInfo{
		Command: "the-collector",
		Version: "0.0.1",
	}

	resource := pdata.NewResource()
	resource.Attributes().InsertString("resource", "R1")

	ilm := pdata.NewInstrumentationLibrary()
	ilm.SetName("test name")
	ilm.SetVersion("test version")

	details := newTraceMetadata(context.TODO())
	commonAttrs := newTransformer(zap.NewNop(), buildInfo, &details).CommonAttributes(resource, ilm)
	assert.Equal(t, "the-collector", commonAttrs[collectorNameKey])
	assert.Equal(t, "0.0.1", commonAttrs[collectorVersionKey])
	assert.Equal(t, "R1", commonAttrs["resource"])
	assert.Equal(t, "test name", commonAttrs[instrumentationNameKey])
	assert.Equal(t, "test version", commonAttrs[instrumentationVersionKey])

	assert.Equal(t, 1, len(details.attributeMetadataCount))
	assert.Equal(t, 1, details.attributeMetadataCount[attributeStatsKey{location: attributeLocationResource, attributeType: pdata.AttributeValueTypeString}])
}

func TestDoesNotCaptureResourceAttributeMetadata(t *testing.T) {
	buildInfo := &component.BuildInfo{
		Command: "the-collector",
		Version: "0.0.1",
	}

	resource := pdata.NewResource()

	ilm := pdata.NewInstrumentationLibrary()
	ilm.SetName("test name")
	ilm.SetVersion("test version")

	details := newTraceMetadata(context.TODO())
	commonAttrs := newTransformer(zap.NewNop(), buildInfo, &details).CommonAttributes(resource, ilm)

	assert.Greater(t, len(commonAttrs), 0)
	assert.Equal(t, 0, len(details.attributeMetadataCount))
}

func TestCaptureSpanMetadata(t *testing.T) {
	details := newTraceMetadata(context.TODO())
	transform := newTransformer(zap.NewNop(), nil, &details)

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
				s.Events().AppendEmpty()
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
				s.Links().AppendEmpty()
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
				s.Events().AppendEmpty()
				s.Links().AppendEmpty()
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

func TestCaptureSpanAttributeMetadata(t *testing.T) {
	details := newTraceMetadata(context.TODO())
	transform := newTransformer(zap.NewNop(), nil, &details)

	s := pdata.NewSpan()
	s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 2}))
	s.SetParentSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	s.SetName("test span")

	se := s.Events().AppendEmpty()
	se.Attributes().InsertBool("testattr", true)

	s.Attributes().InsertInt("spanattr", 42)

	_, err := transform.Span(s)

	require.NoError(t, err)
	assert.Equal(t, 2, len(details.attributeMetadataCount))
	assert.Equal(t, 1, details.attributeMetadataCount[attributeStatsKey{location: attributeLocationSpan, attributeType: pdata.AttributeValueTypeInt}])
	assert.Equal(t, 1, details.attributeMetadataCount[attributeStatsKey{location: attributeLocationSpanEvent, attributeType: pdata.AttributeValueTypeBool}])
}

func TestDoesNotCaptureSpanAttributeMetadata(t *testing.T) {
	details := newTraceMetadata(context.TODO())
	transform := newTransformer(zap.NewNop(), nil, &details)

	s := pdata.NewSpan()
	s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 2}))
	s.SetParentSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	s.SetName("test span")
	s.Events().AppendEmpty()

	_, err := transform.Span(s)

	require.NoError(t, err)
	assert.Equal(t, 0, len(details.attributeMetadataCount))
}

func TestTransformSpan(t *testing.T) {
	now := time.Unix(100, 0)
	details := newTraceMetadata(context.TODO())
	transform := newTransformer(zap.NewNop(), nil, &details)

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
				s := pdata.NewSpan()
				s.SetName("error code")
				s.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 3}))
				s.Status().SetCode(pdata.StatusCodeError)
				return s
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
				s := pdata.NewSpan()
				s.SetName("error message")
				s.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 3}))
				s.Status().SetCode(pdata.StatusCodeError)
				s.Status().SetMessage("error message")
				return s
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
				s := pdata.NewSpan()
				s.SetName("attrs")
				s.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 4}))
				s.Attributes().UpsertBool("prod", true)
				s.Attributes().UpsertInt("weight", 10)
				s.Attributes().UpsertDouble("score", 99.8)
				s.Attributes().UpsertString("user", "alice")
				return s
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
				s.SetStartTimestamp(pdata.NewTimestampFromTime(now))
				s.SetEndTimestamp(pdata.NewTimestampFromTime(now.Add(time.Second * 5)))
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
				s.SetKind(pdata.SpanKindServer)
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

				event := s.Events().AppendEmpty()
				event.SetName("this is the event name")
				event.SetTimestamp(pdata.NewTimestampFromTime(now))
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
		{
			name: "with dropped attributes",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 8}))
				s.SetName("with dropped attributes")
				s.SetDroppedAttributesCount(2)
				return s
			},
			want: telemetry.Span{
				ID:        "0000000000000008",
				TraceID:   "01010101010101010101010101010101",
				Name:      "with dropped attributes",
				Timestamp: time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{
					droppedAttributesCountKey: uint32(2),
				},
				Events: nil,
			},
		},
		{
			name: "with dropped events",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 9}))
				s.SetName("with dropped events")
				s.SetDroppedEventsCount(3)
				return s
			},
			want: telemetry.Span{
				ID:        "0000000000000009",
				TraceID:   "01010101010101010101010101010101",
				Name:      "with dropped events",
				Timestamp: time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{
					droppedEventsCountKey: uint32(3),
				},
				Events: nil,
			},
		},
		{
			name: "with dropped attributes on events",
			spanFunc: func() pdata.Span {
				s := pdata.NewSpan()
				s.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				s.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 10}))
				s.SetName("with dropped attributes on events")

				ev := pdata.NewSpanEventSlice()
				ev.EnsureCapacity(1)
				event := ev.AppendEmpty()
				event.SetName("this is the event name")
				event.SetTimestamp(pdata.NewTimestampFromTime(now))
				event.SetDroppedAttributesCount(1)
				tgt := s.Events().AppendEmpty()
				event.CopyTo(tgt)
				return s
			},
			want: telemetry.Span{
				ID:         "000000000000000a",
				TraceID:    "01010101010101010101010101010101",
				Name:       "with dropped attributes on events",
				Timestamp:  time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{},
				Events: []telemetry.Event{
					{
						EventType: "this is the event name",
						Timestamp: now.UTC(),
						Attributes: map[string]interface{}{
							droppedAttributesCountKey: uint32(1),
						},
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
	comparer := func(t *testing.T, want []telemetry.Metric, got []telemetry.Metric) {
		assert.Equal(t, want, got)
	}
	testTransformMetricWithComparer(t, metric, want, comparer)
}

func testTransformMetricWithComparer(t *testing.T, metric pdata.Metric, want []telemetry.Metric, compare func(t *testing.T, want []telemetry.Metric, got []telemetry.Metric)) {
	details := newMetricMetadata(context.Background())
	transform := newTransformer(zap.NewNop(), &component.BuildInfo{
		Command: testCollectorName,
		Version: testCollectorVersion,
	}, &details)
	got, err := transform.Metric(metric)
	require.NoError(t, err)
	compare(t, want, got)

	assert.Equal(t, len(details.metricMetadataCount), 1)
	for k, v := range details.metricMetadataCount {
		assert.Equal(t, metric.DataType(), k.MetricType)
		assert.Equal(t, 1, v)
	}
}

func testTransformMetricWithError(t *testing.T, metric pdata.Metric, expectedErrorType interface{}) {
	details := newMetricMetadata(context.Background())
	transform := newTransformer(zap.NewNop(), &component.BuildInfo{
		Command: testCollectorName,
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
	ts := pdata.NewTimestampFromTime(time.Unix(1, 0))
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
		m.SetDataType(pdata.MetricDataTypeGauge)
		gd := m.Gauge()
		dp := gd.DataPoints().AppendEmpty()
		dp.SetTimestamp(ts)
		dp.SetDoubleVal(42.0)
		t.Run("Double", func(t *testing.T) { testTransformMetric(t, m, expected) })
	}
	{
		m := pdata.NewMetric()
		m.SetName("gauge")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeGauge)
		gi := m.Gauge()
		dp := gi.DataPoints().AppendEmpty()
		dp.SetTimestamp(ts)
		dp.SetIntVal(42)
		t.Run("Int64", func(t *testing.T) { testTransformMetric(t, m, expected) })
	}
}

func TestTransformSum(t *testing.T) {
	start := pdata.NewTimestampFromTime(time.Unix(1, 0))
	end := pdata.NewTimestampFromTime(time.Unix(3, 0))

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
			ForceIntervalValid: true,
		},
	}
	expectedGauge := []telemetry.Metric{
		telemetry.Gauge{
			Name:      "sum",
			Value:     42.0,
			Timestamp: end.AsTime(),
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
		m.SetDataType(pdata.MetricDataTypeSum)
		d := m.Sum()
		d.SetAggregationTemporality(pdata.AggregationTemporalityDelta)
		dp := d.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetDoubleVal(42.0)
		t.Run("Sum-Delta", func(t *testing.T) { testTransformMetric(t, m, expected) })
	}
	{
		m := pdata.NewMetric()
		m.SetName("sum")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeSum)
		d := m.Sum()
		d.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		dp := d.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetDoubleVal(42.0)
		t.Run("Sum-Cumulative", func(t *testing.T) { testTransformMetric(t, m, expectedGauge) })
	}
	{
		m := pdata.NewMetric()
		m.SetName("sum")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeSum)
		d := m.Sum()
		d.SetAggregationTemporality(pdata.AggregationTemporalityDelta)
		dp := d.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetIntVal(42.0)
		t.Run("IntSum-Delta", func(t *testing.T) { testTransformMetric(t, m, expected) })
	}
	{
		m := pdata.NewMetric()
		m.SetName("sum")
		m.SetDescription("description")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeSum)
		d := m.Sum()
		d.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		dp := d.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetIntVal(42.0)
		t.Run("IntSum-Cumulative", func(t *testing.T) { testTransformMetric(t, m, expectedGauge) })
	}
}

func TestTransformDeltaSummary(t *testing.T) {
	testTransformDeltaSummaryWithValues(t, "Double With Min and Max", 2, 7, 1, 6)
	testTransformDeltaSummaryWithValues(t, "Double With Min and No Max", 1, 1, 1, math.NaN())
	testTransformDeltaSummaryWithValues(t, "Double With Max and No Min", 1, 1, math.NaN(), 1)
	testTransformDeltaSummaryWithValues(t, "Double With No Min and No Max", 0, 0, math.NaN(), math.NaN())
}

func testTransformDeltaSummaryWithValues(t *testing.T, testName string, count uint64, sum float64, min float64, max float64) {
	start := pdata.NewTimestampFromTime(time.Unix(1, 0))
	end := pdata.NewTimestampFromTime(time.Unix(3, 0))

	expected := []telemetry.Metric{
		telemetry.Summary{
			Name:      "summary",
			Count:     float64(count),
			Sum:       sum,
			Min:       min,
			Max:       max,
			Timestamp: time.Unix(1, 0).UTC(),
			Interval:  2 * time.Second,
			Attributes: map[string]interface{}{
				"description": "description",
				"unit":        "s",
				"foo":         "bar",
			},
		},
	}

	comparer := func(t *testing.T, want []telemetry.Metric, got []telemetry.Metric) {
		assert.Equal(t, len(want), len(got))

		for i := 0; i < len(want); i++ {
			wantedSummary, ok := want[i].(telemetry.Summary)
			assert.True(t, ok)
			gotSummary, ok := got[i].(telemetry.Summary)
			assert.True(t, ok)
			assert.Equal(t, wantedSummary.Name, gotSummary.Name)
			assert.Equal(t, wantedSummary.Count, gotSummary.Count)
			assert.Equal(t, wantedSummary.Sum, gotSummary.Sum)
			assert.Equal(t, wantedSummary.Timestamp, gotSummary.Timestamp)
			assert.Equal(t, wantedSummary.Interval, gotSummary.Interval)
			assert.Equal(t, wantedSummary.Attributes, gotSummary.Attributes)
			if math.IsNaN(wantedSummary.Min) {
				assert.True(t, math.IsNaN(gotSummary.Min))
			} else {
				assert.Equal(t, wantedSummary.Min, gotSummary.Min)
			}
			if math.IsNaN(wantedSummary.Max) {
				assert.True(t, math.IsNaN(gotSummary.Max))
			} else {
				assert.Equal(t, wantedSummary.Max, gotSummary.Max)
			}
		}
	}

	m := pdata.NewMetric()
	m.SetName("summary")
	m.SetDescription("description")
	m.SetUnit("s")
	m.SetDataType(pdata.MetricDataTypeSummary)
	ds := m.Summary()
	dp := ds.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(end)
	dp.SetSum(sum)
	dp.SetCount(count)
	dp.Attributes().InsertString("foo", "bar")
	q := dp.QuantileValues()
	if !math.IsNaN(min) {
		minQuantile := q.AppendEmpty()
		minQuantile.SetQuantile(0)
		minQuantile.SetValue(min)
	}
	if !math.IsNaN(max) {
		maxQuantile := q.AppendEmpty()
		maxQuantile.SetQuantile(1)
		maxQuantile.SetValue(max)
	}

	t.Run(testName, func(t *testing.T) { testTransformMetricWithComparer(t, m, expected, comparer) })
}

func TestUnsupportedMetricTypes(t *testing.T) {
	start := pdata.NewTimestampFromTime(time.Unix(1, 0))
	end := pdata.NewTimestampFromTime(time.Unix(3, 0))
	{
		m := pdata.NewMetric()
		m.SetName("no")
		m.SetDescription("no")
		m.SetUnit("1")
		m.SetDataType(pdata.MetricDataTypeHistogram)
		h := m.Histogram()
		dp := h.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(start)
		dp.SetTimestamp(end)
		dp.SetCount(2)
		dp.SetSum(8.0)
		dp.SetExplicitBounds([]float64{3, 7, 11})
		dp.SetBucketCounts([]uint64{1, 1, 0, 0})
		h.SetAggregationTemporality(pdata.AggregationTemporalityDelta)

		t.Run("DoubleHistogram", func(t *testing.T) {
			testTransformMetricWithError(t, m, consumererror.Permanent(&errUnsupportedMetricType{}))
		})
	}
}

func TestTransformUnknownMetricType(t *testing.T) {
	metric := pdata.NewMetric()
	details := newMetricMetadata(context.Background())
	transform := newTransformer(zap.NewNop(), &component.BuildInfo{
		Command: testCollectorName,
		Version: testCollectorVersion,
	}, &details)

	got, err := transform.Metric(metric)

	require.NoError(t, err)
	assert.Nil(t, got)
	assert.Equal(t, 1, details.metricMetadataCount[metricStatsKey{MetricType: pdata.MetricDataTypeNone}])
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
				timestamp := pdata.NewTimestampFromTime(time.Unix(0, 0).UTC())
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
		{
			name: "With traceID and spanID",
			logFunc: func() pdata.LogRecord {
				log := pdata.NewLogRecord()
				timestamp := pdata.NewTimestampFromTime(time.Unix(0, 0).UTC())
				log.SetTraceID(pdata.NewTraceID([...]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
				log.SetSpanID(pdata.NewSpanID([...]byte{0, 0, 0, 0, 0, 0, 0, 1}))
				log.SetTimestamp(timestamp)
				return log
			},
			want: telemetry.Log{
				Message:   "",
				Timestamp: time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{
					"name":     "",
					"trace.id": "01010101010101010101010101010101",
					"span.id":  "0000000000000001",
				},
			},
		},
		{
			name: "With dropped attribute count",
			logFunc: func() pdata.LogRecord {
				log := pdata.NewLogRecord()
				timestamp := pdata.NewTimestampFromTime(time.Unix(0, 0).UTC())
				log.SetTimestamp(timestamp)
				log.SetDroppedAttributesCount(4)
				return log
			},
			want: telemetry.Log{
				Message:   "",
				Timestamp: time.Unix(0, 0).UTC(),
				Attributes: map[string]interface{}{
					"name":                    "",
					droppedAttributesCountKey: uint32(4),
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			details := newLogMetadata(context.TODO())
			transform := newTransformer(zap.NewNop(), nil, &details)
			got, _ := transform.Log(test.logFunc())
			assert.EqualValues(t, test.want, got)
		})
	}
}

func TestCaptureLogAttributeMetadata(t *testing.T) {
	log := pdata.NewLogRecord()
	log.SetName("bloopbleep")
	log.Attributes().InsertString("foo", "bar")
	log.Body().SetStringVal("Hello World")

	details := newLogMetadata(context.TODO())
	transform := newTransformer(zap.NewNop(), nil, &details)
	_, err := transform.Log(log)

	require.NoError(t, err)
	assert.Equal(t, 1, len(details.attributeMetadataCount))
	assert.Equal(t, 1, details.attributeMetadataCount[attributeStatsKey{location: attributeLocationLog, attributeType: pdata.AttributeValueTypeString}])
}

func TestDoesNotCaptureLogAttributeMetadata(t *testing.T) {
	log := pdata.NewLogRecord()
	log.SetName("bloopbleep")
	log.Body().SetStringVal("Hello World")

	details := newLogMetadata(context.TODO())
	transform := newTransformer(zap.NewNop(), nil, &details)
	_, err := transform.Log(log)

	require.NoError(t, err)
	assert.Equal(t, 0, len(details.attributeMetadataCount))
}

func TestUnsupportedMetricErrorCreation(t *testing.T) {
	e := errUnsupportedMetricType{
		metricType:    "testType",
		metricName:    "testName",
		numDataPoints: 1,
	}

	errorMessage := e.Error()

	assert.Equal(t, "unsupported metric testName (testType)", errorMessage)
}
